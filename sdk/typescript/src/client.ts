/**
 * OmniphiClient — the main entry point for interacting with the Omniphi chain.
 *
 * Wraps `@cosmjs/stargate`'s `SigningStargateClient` with Omniphi-specific
 * functionality: custom message encoding, intent submission, and typed
 * queries for all custom modules.
 */

import {
  SigningStargateClient,
  StargateClient,
  type Coin,
  type DeliverTxResponse,
  type StargateClientOptions,
  GasPrice,
} from "@cosmjs/stargate";
import type { OfflineDirectSigner } from "@cosmjs/proto-signing";

import {
  DEFAULT_FEE,
  DEFAULT_GAS_PRICE,
  DEFAULT_RPC_ENDPOINT,
  DEFAULT_REST_ENDPOINT,
  DENOM,
  MSG_TYPE_URLS,
  REST_PATHS,
} from "./constants";
import { createOmniphiRegistry, createOmniphiAminoTypes, encodeMsg, normalizeHash } from "./encoding";
import type {
  StdFee,
  Contribution,
  CheckpointAnchorRecord,
  EpochStateReference,
  Intent,
  IntentTransaction,
  App,
  BatchCommitment,
  TokenSupply,
  InflationInfo,
  RoyaltyToken,
  Adapter,
  VoterWeight,
  MsgSubmitContribution,
} from "./types";

// ---------------------------------------------------------------------------
// Error classes
// ---------------------------------------------------------------------------

/** Thrown when a query to the REST API fails. */
export class OmniphiQueryError extends Error {
  constructor(
    public readonly endpoint: string,
    public readonly status: number,
    public readonly body: string,
  ) {
    super(`Query to ${endpoint} failed (HTTP ${status}): ${body}`);
    this.name = "OmniphiQueryError";
  }
}

/** Thrown when a transaction broadcast fails. */
export class OmniphiTxError extends Error {
  constructor(
    public readonly code: number,
    public readonly txHash: string,
    public readonly rawLog: string,
  ) {
    super(`Transaction ${txHash} failed with code ${code}: ${rawLog}`);
    this.name = "OmniphiTxError";
  }
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

/**
 * Configuration options for `OmniphiClient`.
 */
export interface OmniphiClientOptions {
  /** RPC endpoint URL. Defaults to `http://localhost:26657`. */
  rpcEndpoint?: string;
  /** REST / LCD endpoint URL. Defaults to `http://localhost:1318`. */
  restEndpoint?: string;
  /** Gas price string. Defaults to `"0.025omniphi"`. */
  gasPrice?: string;
  /** Additional StargateClient options. */
  stargateOptions?: StargateClientOptions;
}

/**
 * High-level client for the Omniphi blockchain.
 *
 * Use the static factory methods `connect` (read-only) or
 * `connectWithSigner` (read + write) to create an instance.
 *
 * @example
 * ```ts
 * // Read-only
 * const client = await OmniphiClient.connect("http://localhost:26657");
 * const balance = await client.getBalance("omni1...", "omniphi");
 *
 * // With signer
 * const client = await OmniphiClient.connectWithSigner(
 *   "http://localhost:26657",
 *   signer,
 * );
 * const result = await client.sendTokens(sender, recipient, "1000000", "omniphi");
 * ```
 */
export class OmniphiClient {
  /** The underlying StargateClient (read-only). May be null if signing client is used. */
  private readonly queryClient: StargateClient | null;

  /** The underlying SigningStargateClient (read + write). Null for read-only clients. */
  private readonly signingClient: SigningStargateClient | null;

  /** REST API base URL for gRPC-gateway queries. */
  private readonly restEndpoint: string;

  // -----------------------------------------------------------------------
  // Construction
  // -----------------------------------------------------------------------

  private constructor(
    queryClient: StargateClient | null,
    signingClient: SigningStargateClient | null,
    restEndpoint: string,
  ) {
    this.queryClient = queryClient;
    this.signingClient = signingClient;
    this.restEndpoint = restEndpoint.replace(/\/+$/, ""); // strip trailing slash
  }

  /**
   * Creates a **read-only** client connected to the given RPC endpoint.
   *
   * Use this when you only need to query chain state (balances, contributions,
   * validators, etc.) without signing or broadcasting transactions.
   *
   * @param rpcEndpoint  - Tendermint RPC URL. Defaults to localhost.
   * @param options      - Optional REST endpoint and Stargate options.
   */
  static async connect(
    rpcEndpoint: string = DEFAULT_RPC_ENDPOINT,
    options: OmniphiClientOptions = {},
  ): Promise<OmniphiClient> {
    const restEndpoint = options.restEndpoint ?? DEFAULT_REST_ENDPOINT;
    const client = await StargateClient.connect(rpcEndpoint);
    return new OmniphiClient(client, null, restEndpoint);
  }

  /**
   * Creates a **signing** client connected to the given RPC endpoint.
   *
   * Use this when you need to broadcast transactions (send tokens, submit
   * contributions, delegate, etc.).
   *
   * @param rpcEndpoint - Tendermint RPC URL.
   * @param signer      - An `OfflineDirectSigner` (e.g., from `fromMnemonic`).
   * @param options     - Optional REST endpoint, gas price, and Stargate options.
   */
  static async connectWithSigner(
    rpcEndpoint: string = DEFAULT_RPC_ENDPOINT,
    signer: OfflineDirectSigner,
    options: OmniphiClientOptions = {},
  ): Promise<OmniphiClient> {
    const restEndpoint = options.restEndpoint ?? DEFAULT_REST_ENDPOINT;
    const gasPrice = GasPrice.fromString(options.gasPrice ?? DEFAULT_GAS_PRICE);

    const registry = createOmniphiRegistry();
    const aminoTypes = createOmniphiAminoTypes();

    const signingClient = await SigningStargateClient.connectWithSigner(
      rpcEndpoint,
      signer,
      {
        registry,
        aminoTypes,
        gasPrice,
        ...options.stargateOptions,
      },
    );

    return new OmniphiClient(null, signingClient, restEndpoint);
  }

  /**
   * Returns the effective Stargate client for queries.
   * Signing clients can also query; read-only clients cannot sign.
   */
  private get client(): StargateClient {
    const c = this.signingClient ?? this.queryClient;
    if (!c) {
      throw new Error("Client is not connected");
    }
    return c;
  }

  /**
   * Returns the signing client, or throws if this is a read-only connection.
   */
  private get signer(): SigningStargateClient {
    if (!this.signingClient) {
      throw new Error(
        "This client was created with connect() (read-only). " +
          "Use connectWithSigner() to enable transaction signing.",
      );
    }
    return this.signingClient;
  }

  /**
   * Disconnects the client and releases resources.
   */
  disconnect(): void {
    if (this.signingClient) {
      this.signingClient.disconnect();
    } else if (this.queryClient) {
      this.queryClient.disconnect();
    }
  }

  // -----------------------------------------------------------------------
  // REST query helper
  // -----------------------------------------------------------------------

  /**
   * Fetches JSON from the REST / gRPC-gateway endpoint.
   * @throws {OmniphiQueryError} on non-2xx responses.
   */
  private async restQuery<T>(path: string): Promise<T> {
    const url = `${this.restEndpoint}${path}`;
    const response = await fetch(url);
    const body = await response.text();

    if (!response.ok) {
      throw new OmniphiQueryError(url, response.status, body);
    }

    return JSON.parse(body) as T;
  }

  // -----------------------------------------------------------------------
  // Chain info
  // -----------------------------------------------------------------------

  /**
   * Returns the current block height.
   */
  async getHeight(): Promise<number> {
    return this.client.getHeight();
  }

  /**
   * Returns the chain ID.
   */
  async getChainId(): Promise<string> {
    return this.client.getChainId();
  }

  // -----------------------------------------------------------------------
  // Bank / balance queries
  // -----------------------------------------------------------------------

  /**
   * Queries the balance of a specific denomination for an address.
   *
   * @param address - Bech32-encoded account address.
   * @param denom   - Token denomination. Defaults to `"omniphi"`.
   * @returns The `Coin` balance.
   */
  async getBalance(address: string, denom: string = DENOM): Promise<Coin> {
    return this.client.getBalance(address, denom);
  }

  /**
   * Queries all balances for an address.
   *
   * @param address - Bech32-encoded account address.
   * @returns Array of `Coin` balances.
   */
  async getAllBalances(address: string): Promise<readonly Coin[]> {
    return this.client.getAllBalances(address);
  }

  // -----------------------------------------------------------------------
  // Token transfers
  // -----------------------------------------------------------------------

  /**
   * Sends tokens from one address to another.
   *
   * @param sender    - Sender's bech32 address (must match the signer).
   * @param recipient - Recipient's bech32 address.
   * @param amount    - Amount as a string (in base units).
   * @param denom     - Token denomination. Defaults to `"omniphi"`.
   * @param fee       - Transaction fee. Defaults to the standard fee.
   * @param memo      - Optional transaction memo.
   * @returns The transaction result.
   * @throws {OmniphiTxError} if the transaction fails on-chain.
   */
  async sendTokens(
    sender: string,
    recipient: string,
    amount: string,
    denom: string = DENOM,
    fee: StdFee = DEFAULT_FEE,
    memo: string = "",
  ): Promise<DeliverTxResponse> {
    const coins: Coin[] = [{ denom, amount }];
    const result = await this.signer.sendTokens(sender, recipient, coins, fee, memo);
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // Staking
  // -----------------------------------------------------------------------

  /**
   * Queries the list of validators.
   *
   * @returns Array of validator objects from the REST API.
   */
  async getValidators(): Promise<unknown[]> {
    const response = await this.restQuery<{
      validators: unknown[];
    }>(REST_PATHS.STAKING_VALIDATORS);
    return response.validators;
  }

  /**
   * Delegates tokens to a validator.
   *
   * @param delegator - Delegator's bech32 address (must match the signer).
   * @param validator - Validator's operator bech32 address (`omnivaloper...`).
   * @param amount    - Amount as a string (in base units).
   * @param fee       - Transaction fee. Defaults to the standard fee.
   * @param memo      - Optional transaction memo.
   * @returns The transaction result.
   */
  async delegate(
    delegator: string,
    validator: string,
    amount: string,
    fee: StdFee = DEFAULT_FEE,
    memo: string = "",
  ): Promise<DeliverTxResponse> {
    const coin: Coin = { denom: DENOM, amount };
    const result = await this.signer.delegateTokens(
      delegator,
      validator,
      coin,
      fee,
      memo,
    );
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Undelegates tokens from a validator.
   *
   * @param delegator - Delegator's bech32 address.
   * @param validator - Validator's operator bech32 address.
   * @param amount    - Amount to undelegate.
   * @param fee       - Transaction fee.
   * @param memo      - Optional memo.
   */
  async undelegate(
    delegator: string,
    validator: string,
    amount: string,
    fee: StdFee = DEFAULT_FEE,
    memo: string = "",
  ): Promise<DeliverTxResponse> {
    const coin: Coin = { denom: DENOM, amount };
    const result = await this.signer.undelegateTokens(
      delegator,
      validator,
      coin,
      fee,
      memo,
    );
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // x/poc — Proof of Contribution
  // -----------------------------------------------------------------------

  /**
   * Submits a new Proof-of-Contribution.
   *
   * @param msg  - The contribution details.
   * @param fee  - Transaction fee.
   * @param memo - Optional memo.
   * @returns The transaction result. The contribution ID is in the events.
   */
  async submitContribution(
    msg: MsgSubmitContribution,
    fee: StdFee = DEFAULT_FEE,
    memo: string = "",
  ): Promise<DeliverTxResponse> {
    const hash = normalizeHash(msg.hash);
    const value: Record<string, unknown> = {
      contributor: msg.contributor,
      ctype: msg.ctype,
      uri: msg.uri,
      hash: Array.from(hash),
    };
    if (msg.canonicalHash) {
      value.canonical_hash = Array.from(normalizeHash(msg.canonicalHash));
    }
    if (msg.canonicalSpecVersion !== undefined) {
      value.canonical_spec_version = msg.canonicalSpecVersion;
    }

    const encObj = encodeMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, value);
    const result = await this.signer.signAndBroadcast(
      msg.contributor,
      [encObj],
      fee,
      memo,
    );
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Endorses a contribution as a validator.
   *
   * @param validator      - Validator's bech32 address.
   * @param contributionId - The contribution ID to endorse.
   * @param decision       - `true` to approve, `false` to reject.
   * @param fee            - Transaction fee.
   */
  async endorseContribution(
    validator: string,
    contributionId: number,
    decision: boolean,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.ENDORSE, {
      validator,
      contribution_id: contributionId,
      decision,
    });
    const result = await this.signer.signAndBroadcast(
      validator,
      [encObj],
      fee,
    );
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Withdraws accumulated PoC rewards for a contributor.
   */
  async withdrawPocRewards(
    contributor: string,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.WITHDRAW_POC_REWARDS, {
      contributor,
    });
    const result = await this.signer.signAndBroadcast(
      contributor,
      [encObj],
      fee,
    );
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Queries a contribution by its ID.
   *
   * @param id - The contribution ID.
   * @returns The contribution record, or `null` if not found.
   */
  async queryContribution(id: number): Promise<Contribution | null> {
    try {
      const response = await this.restQuery<{ contribution: Contribution }>(
        REST_PATHS.POC_CONTRIBUTION(id),
      );
      return response.contribution ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  // -----------------------------------------------------------------------
  // x/poseq — Proof of Sequencing
  // -----------------------------------------------------------------------

  /**
   * Queries the PoSeq checkpoint anchor for a given epoch.
   *
   * @param epoch - The PoSeq epoch number.
   * @returns The checkpoint anchor record, or `null` if not found.
   */
  async queryPoSeqCheckpoint(epoch: number): Promise<CheckpointAnchorRecord | null> {
    try {
      const response = await this.restQuery<{ checkpoint: CheckpointAnchorRecord }>(
        REST_PATHS.POSEQ_CHECKPOINT(epoch),
      );
      return response.checkpoint ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  /**
   * Queries the PoSeq epoch state reference.
   *
   * @param epoch - The PoSeq epoch number.
   * @returns The epoch state reference, or `null` if not found.
   */
  async queryPoSeqEpochState(epoch: number): Promise<EpochStateReference | null> {
    try {
      const response = await this.restQuery<{ epoch_state: EpochStateReference }>(
        REST_PATHS.POSEQ_EPOCH_STATE(epoch),
      );
      return response.epoch_state ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  // -----------------------------------------------------------------------
  // x/por — Proof of Record
  // -----------------------------------------------------------------------

  /**
   * Registers a new application in the PoR module.
   */
  async registerApp(
    owner: string,
    name: string,
    schemaCid: string,
    challengePeriod: number,
    minVerifiers: number,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.REGISTER_APP, {
      owner,
      name,
      schema_cid: schemaCid,
      challenge_period: challengePeriod,
      min_verifiers: minVerifiers,
    });
    const result = await this.signer.signAndBroadcast(owner, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Submits a batch commitment to the PoR module.
   */
  async submitBatch(
    submitter: string,
    appId: number,
    epoch: number,
    recordMerkleRoot: Uint8Array | string,
    recordCount: number,
    verifierSetId: number,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const root = normalizeHash(recordMerkleRoot);
    const encObj = encodeMsg(MSG_TYPE_URLS.SUBMIT_BATCH, {
      submitter,
      app_id: appId,
      epoch,
      record_merkle_root: Array.from(root),
      record_count: recordCount,
      verifier_set_id: verifierSetId,
    });
    const result = await this.signer.signAndBroadcast(submitter, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Queries a PoR app by ID.
   */
  async queryApp(appId: number): Promise<App | null> {
    try {
      const response = await this.restQuery<{ app: App }>(
        REST_PATHS.POR_APP(appId),
      );
      return response.app ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  /**
   * Queries a PoR batch commitment by ID.
   */
  async queryBatch(batchId: number): Promise<BatchCommitment | null> {
    try {
      const response = await this.restQuery<{ batch: BatchCommitment }>(
        REST_PATHS.POR_BATCH(batchId),
      );
      return response.batch ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  // -----------------------------------------------------------------------
  // x/tokenomics
  // -----------------------------------------------------------------------

  /**
   * Queries the current token supply.
   */
  async queryTokenSupply(): Promise<TokenSupply> {
    const response = await this.restQuery<TokenSupply>(
      REST_PATHS.TOKENOMICS_SUPPLY,
    );
    return response;
  }

  /**
   * Queries current inflation parameters.
   */
  async queryInflation(): Promise<InflationInfo> {
    const response = await this.restQuery<InflationInfo>(
      REST_PATHS.TOKENOMICS_INFLATION,
    );
    return response;
  }

  // -----------------------------------------------------------------------
  // x/royalty
  // -----------------------------------------------------------------------

  /**
   * Queries a royalty token by ID.
   */
  async queryRoyaltyToken(tokenId: number): Promise<RoyaltyToken | null> {
    try {
      const response = await this.restQuery<{ token: RoyaltyToken }>(
        REST_PATHS.ROYALTY_TOKEN(tokenId),
      );
      return response.token ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  /**
   * Tokenizes a royalty stream from a PoC contribution.
   */
  async tokenizeRoyalty(
    creator: string,
    claimId: number,
    royaltyShare: string,
    metadata: string = "",
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.TOKENIZE_ROYALTY, {
      creator,
      claim_id: claimId,
      royalty_share: royaltyShare,
      metadata,
    });
    const result = await this.signer.signAndBroadcast(creator, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Transfers a royalty token to a new owner.
   */
  async transferRoyaltyToken(
    sender: string,
    recipient: string,
    tokenId: number,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.TRANSFER_TOKEN, {
      sender,
      recipient,
      token_id: tokenId,
    });
    const result = await this.signer.signAndBroadcast(sender, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Claims accumulated royalties for a token.
   */
  async claimRoyalties(
    owner: string,
    tokenId: number,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.CLAIM_ROYALTIES, {
      owner,
      token_id: tokenId,
    });
    const result = await this.signer.signAndBroadcast(owner, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // x/uci — Universal Contribution Interface
  // -----------------------------------------------------------------------

  /**
   * Queries a UCI adapter by ID.
   */
  async queryAdapter(adapterId: number): Promise<Adapter | null> {
    try {
      const response = await this.restQuery<{ adapter: Adapter }>(
        REST_PATHS.UCI_ADAPTER(adapterId),
      );
      return response.adapter ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  /**
   * Registers a new DePIN adapter.
   */
  async registerAdapter(
    owner: string,
    name: string,
    schemaCid: string,
    oracleAllowlist: string[],
    networkType: string,
    rewardShare: string,
    description: string = "",
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.REGISTER_ADAPTER, {
      owner,
      name,
      schema_cid: schemaCid,
      oracle_allowlist: oracleAllowlist,
      network_type: networkType,
      reward_share: rewardShare,
      description,
    });
    const result = await this.signer.signAndBroadcast(owner, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Submits a DePIN contribution through an adapter.
   */
  async submitDePINContribution(
    submitter: string,
    adapterId: number,
    externalId: string,
    contributor: string,
    dataHash: string,
    dataUri: string,
    batchId: string = "",
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.SUBMIT_DEPIN_CONTRIBUTION, {
      submitter,
      adapter_id: adapterId,
      external_id: externalId,
      contributor,
      data_hash: dataHash,
      data_uri: dataUri,
      batch_id: batchId,
    });
    const result = await this.signer.signAndBroadcast(submitter, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // x/repgov — Reputation Governance
  // -----------------------------------------------------------------------

  /**
   * Queries the voter weight for an address.
   */
  async queryVoterWeight(address: string): Promise<VoterWeight | null> {
    try {
      const response = await this.restQuery<{ weight: VoterWeight }>(
        REST_PATHS.REPGOV_VOTER_WEIGHT(address),
      );
      return response.weight ?? null;
    } catch (err) {
      if (err instanceof OmniphiQueryError && err.status === 404) {
        return null;
      }
      throw err;
    }
  }

  /**
   * Delegates governance reputation to another address.
   */
  async delegateReputation(
    delegator: string,
    delegatee: string,
    amount: string,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.DELEGATE_REPUTATION, {
      delegator,
      delegatee,
      amount,
    });
    const result = await this.signer.signAndBroadcast(delegator, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // x/guard — Safety Guard
  // -----------------------------------------------------------------------

  /**
   * Queries the guard risk report.
   */
  async queryGuardRiskReport(): Promise<unknown> {
    return this.restQuery(REST_PATHS.GUARD_RISK_REPORT);
  }

  // -----------------------------------------------------------------------
  // x/contracts
  // -----------------------------------------------------------------------

  /**
   * Deploys a new contract schema with Wasm bytecode.
   */
  async deployContract(
    deployer: string,
    name: string,
    description: string,
    domainTag: string,
    intentSchemas: Array<{ method: string; params: Array<{ name: string; typeHint: string }>; capabilities: string[] }>,
    maxGasPerCall: number,
    maxStateBytes: number,
    wasmBytecode: Uint8Array,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.DEPLOY_CONTRACT, {
      deployer,
      name,
      description,
      domain_tag: domainTag,
      intent_schemas: intentSchemas.map((s) => ({
        method: s.method,
        params: s.params.map((p) => ({ name: p.name, type_hint: p.typeHint })),
        capabilities: s.capabilities,
      })),
      max_gas_per_call: maxGasPerCall,
      max_state_bytes: maxStateBytes,
      wasm_bytecode: Array.from(wasmBytecode),
    });
    const result = await this.signer.signAndBroadcast(deployer, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Instantiates a deployed contract schema.
   */
  async instantiateContract(
    creator: string,
    schemaId: string,
    label: string,
    admin: string = "",
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(MSG_TYPE_URLS.INSTANTIATE_CONTRACT, {
      creator,
      schema_id: schemaId,
      label,
      admin: admin || creator,
    });
    const result = await this.signer.signAndBroadcast(creator, [encObj], fee);
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // Intent submission
  // -----------------------------------------------------------------------

  /**
   * Submits an intent-based transaction to the chain.
   *
   * Intents are a high-level abstraction over raw Cosmos SDK messages. The
   * SDK translates each intent into the appropriate on-chain message(s) and
   * broadcasts them atomically.
   *
   * @param sender  - The sender's bech32 address (must match the signer).
   * @param intent  - A single intent to submit.
   * @param fee     - Transaction fee.
   * @param memo    - Optional memo.
   * @returns The transaction result.
   *
   * @example
   * ```ts
   * const result = await client.submitIntent("omni1...", {
   *   type: "transfer",
   *   sender: "omni1...",
   *   recipient: "omni1...",
   *   amount: "1000000",
   *   denom: "omniphi",
   * });
   * ```
   */
  async submitIntent(
    sender: string,
    intent: Intent,
    fee: StdFee = DEFAULT_FEE,
    memo: string = "",
  ): Promise<DeliverTxResponse> {
    const messages = this.intentToMessages(sender, intent);
    const result = await this.signer.signAndBroadcast(sender, messages, fee, memo);
    this.assertTxSuccess(result);
    return result;
  }

  /**
   * Submits multiple intents atomically in a single transaction.
   *
   * @param tx - The intent transaction containing one or more intents.
   * @param fee - Transaction fee.
   * @returns The transaction result.
   */
  async submitIntentTransaction(
    tx: IntentTransaction,
    fee: StdFee = DEFAULT_FEE,
  ): Promise<DeliverTxResponse> {
    const messages = tx.intents.flatMap((intent) =>
      this.intentToMessages(tx.sender, intent),
    );
    const result = await this.signer.signAndBroadcast(
      tx.sender,
      messages,
      fee,
      tx.memo ?? "",
    );
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // Module params (generic)
  // -----------------------------------------------------------------------

  /**
   * Queries the parameters for any Omniphi module via REST.
   *
   * @param module - Module name (e.g., "poc", "por", "guard").
   * @returns The raw params object.
   */
  async queryModuleParams(module: string): Promise<unknown> {
    const path = `/pos/${module}/v1/params`;
    const response = await this.restQuery<{ params: unknown }>(path);
    return response.params;
  }

  // -----------------------------------------------------------------------
  // Raw message broadcast
  // -----------------------------------------------------------------------

  /**
   * Signs and broadcasts an arbitrary encoded message.
   *
   * Use this for message types not covered by the convenience methods above.
   *
   * @param sender   - The sender's bech32 address.
   * @param typeUrl  - The protobuf type URL (use `MSG_TYPE_URLS`).
   * @param value    - The message body as a plain object.
   * @param fee      - Transaction fee.
   * @param memo     - Optional memo.
   */
  async signAndBroadcast(
    sender: string,
    typeUrl: string,
    value: Record<string, unknown>,
    fee: StdFee = DEFAULT_FEE,
    memo: string = "",
  ): Promise<DeliverTxResponse> {
    const encObj = encodeMsg(typeUrl, value);
    const result = await this.signer.signAndBroadcast(
      sender,
      [encObj],
      fee,
      memo,
    );
    this.assertTxSuccess(result);
    return result;
  }

  // -----------------------------------------------------------------------
  // Private helpers
  // -----------------------------------------------------------------------

  /**
   * Converts a high-level intent into one or more `EncodeObject` messages.
   */
  private intentToMessages(
    sender: string,
    intent: Intent,
  ): import("@cosmjs/proto-signing").EncodeObject[] {
    switch (intent.type) {
      case "transfer":
        return [
          {
            typeUrl: "/cosmos.bank.v1beta1.MsgSend",
            value: {
              fromAddress: intent.sender || sender,
              toAddress: intent.recipient,
              amount: [{ denom: intent.denom, amount: intent.amount }],
            },
          },
        ];

      case "swap":
        // Swaps are submitted as a custom intent message.
        // The chain's solver network picks them up and executes the optimal route.
        return [
          encodeMsg("/pos.contracts.v1.MsgSubmitSwapIntent", {
            sender: intent.sender || sender,
            input_denom: intent.inputDenom,
            input_amount: intent.inputAmount,
            output_denom: intent.outputDenom,
            min_output_amount: intent.minOutputAmount,
            max_slippage_bps: intent.maxSlippageBps ?? 50,
          }),
        ];

      case "delegate":
        return [
          {
            typeUrl: "/cosmos.staking.v1beta1.MsgDelegate",
            value: {
              delegatorAddress: intent.delegator || sender,
              validatorAddress: intent.validator,
              amount: { denom: intent.denom, amount: intent.amount },
            },
          },
        ];

      case "contribute": {
        const hash = normalizeHash(intent.hash);
        return [
          encodeMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, {
            contributor: intent.contributor || sender,
            ctype: intent.ctype,
            uri: intent.uri,
            hash: Array.from(hash),
          }),
        ];
      }

      case "deploy_contract":
        return [
          encodeMsg(MSG_TYPE_URLS.DEPLOY_CONTRACT, {
            deployer: intent.deployer || sender,
            name: intent.name,
            description: intent.description,
            domain_tag: intent.domainTag,
            intent_schemas: intent.intentSchemas.map((s) => ({
              method: s.method,
              params: s.params.map((p) => ({
                name: p.name,
                type_hint: p.typeHint,
              })),
              capabilities: s.capabilities,
            })),
            max_gas_per_call: intent.maxGasPerCall,
            max_state_bytes: intent.maxStateBytes,
            wasm_bytecode: Array.from(intent.wasmBytecode),
          }),
        ];

      default:
        throw new Error(`Unknown intent type: ${(intent as { type: string }).type}`);
    }
  }

  /**
   * Asserts that a transaction result indicates success.
   * @throws {OmniphiTxError} if `result.code !== 0`.
   */
  private assertTxSuccess(result: DeliverTxResponse): void {
    if (result.code !== 0) {
      throw new OmniphiTxError(
        result.code,
        result.transactionHash,
        result.rawLog ?? "Unknown error",
      );
    }
  }
}
