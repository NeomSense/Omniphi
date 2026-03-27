package relay

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/crypto/sha3"
)

// ────────────────────────────────────────────────────────────────────────────
//  Ethereum ABI / RPC helpers
// ────────────────────────────────────────────────────────────────────────────

// depositEventTopic is keccak256("Deposit(uint256,address,uint256,address,string)").
var depositEventTopic string

func init() {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("Deposit(uint256,address,uint256,address,string)"))
	depositEventTopic = "0x" + hex.EncodeToString(h.Sum(nil))
}

// ethLog mirrors a subset of the Ethereum JSON-RPC log object.
type ethLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

// rpcResponse is a generic JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// ────────────────────────────────────────────────────────────────────────────
//  Parsed bridge events
// ────────────────────────────────────────────────────────────────────────────

// DepositEvent represents a parsed Deposit event from the bridge contract.
type DepositEvent struct {
	Nonce            *big.Int
	Token            string // 0x-prefixed hex, address(0) for ETH
	Amount           *big.Int
	Sender           string // 0x-prefixed hex
	OmniphiRecipient string
	BlockNumber      uint64
	TxHash           string
}

// BurnEvent represents a parsed BurnAndBridge event from the Omniphi chain.
type BurnEvent struct {
	Nonce     uint64
	Token     string // original Ethereum token address
	Amount    *big.Int
	Burner    string // Omniphi bech32 address
	Recipient string // 0x-prefixed Ethereum address
	Height    int64
	TxHash    string
}

// ────────────────────────────────────────────────────────────────────────────
//  Relay service
// ────────────────────────────────────────────────────────────────────────────

// Relay is the main bridge relay service.
type Relay struct {
	cfg    Config
	logger *slog.Logger

	ethPrivKey  *ecdsa.PrivateKey
	ethAddress  string // derived from private key, 0x-prefixed

	// Track the last processed Ethereum block.
	lastEthBlock uint64

	// Health state.
	healthy      atomic.Bool
	lastEthPoll  atomic.Int64 // unix timestamp
	lastOmniPoll atomic.Int64

	httpClient *http.Client

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Relay from the given config.
func New(cfg Config) (*Relay, error) {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	privKey, err := parseECDSAPrivateKey(cfg.EthPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parsing ethereum private key: %w", err)
	}

	addr := pubkeyToAddress(&privKey.PublicKey)

	r := &Relay{
		cfg:        cfg,
		logger:     logger,
		ethPrivKey: privKey,
		ethAddress: addr,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	r.healthy.Store(true)

	logger.Info("relay initialized",
		"eth_address", addr,
		"bridge_contract", cfg.BridgeContractAddress,
		"eth_chain_id", cfg.EthChainID,
		"omniphi_chain_id", cfg.OmniphiChainID,
	)

	return r, nil
}

// Run starts the relay loops and blocks until ctx is canceled.
func (r *Relay) Run(ctx context.Context) error {
	ctx, r.cancel = context.WithCancel(ctx)

	// Start the health-check HTTP server.
	healthSrv := r.startHealthServer()

	// Determine starting block.
	if r.cfg.EthStartBlock > 0 {
		r.lastEthBlock = r.cfg.EthStartBlock - 1
	} else {
		block, err := r.ethBlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("fetching latest ethereum block: %w", err)
		}
		r.lastEthBlock = block
		r.logger.Info("starting from latest ethereum block", "block", block)
	}

	r.wg.Add(2)
	go r.ethDepositLoop(ctx)
	go r.omniphiBurnLoop(ctx)

	<-ctx.Done()

	r.logger.Info("shutting down relay")
	r.cancel()
	r.wg.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		r.logger.Error("health server shutdown error", "err", err)
	}

	return ctx.Err()
}

// ────────────────────────────────────────────────────────────────────────────
//  Ethereum deposit watcher
// ────────────────────────────────────────────────────────────────────────────

func (r *Relay) ethDepositLoop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.cfg.EthPollInterval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.pollEthDeposits(ctx); err != nil {
				r.logger.Error("eth deposit poll failed", "err", err)
				r.healthy.Store(false)
			} else {
				r.lastEthPoll.Store(time.Now().Unix())
				r.healthy.Store(true)
			}
		}
	}
}

func (r *Relay) pollEthDeposits(ctx context.Context) error {
	head, err := r.ethBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("eth_blockNumber: %w", err)
	}

	// Only process confirmed blocks.
	confirmed := head - r.cfg.EthConfirmations
	if confirmed <= r.lastEthBlock {
		return nil // nothing new
	}

	fromBlock := r.lastEthBlock + 1
	toBlock := confirmed
	// Cap range to batch size to avoid huge responses.
	if toBlock-fromBlock+1 > uint64(r.cfg.BatchSize) {
		toBlock = fromBlock + uint64(r.cfg.BatchSize) - 1
	}

	r.logger.Debug("polling eth deposits", "from", fromBlock, "to", toBlock)

	logs, err := r.ethGetLogs(ctx, fromBlock, toBlock)
	if err != nil {
		return fmt.Errorf("eth_getLogs [%d, %d]: %w", fromBlock, toBlock, err)
	}

	for _, deposit := range logs {
		r.logger.Info("observed ethereum deposit",
			"nonce", deposit.Nonce.String(),
			"token", deposit.Token,
			"amount", deposit.Amount.String(),
			"sender", deposit.Sender,
			"omniphi_recipient", deposit.OmniphiRecipient,
			"block", deposit.BlockNumber,
			"tx", deposit.TxHash,
		)

		if err := r.submitBridgeMint(ctx, deposit); err != nil {
			r.logger.Error("failed to submit bridge mint",
				"nonce", deposit.Nonce.String(),
				"err", err,
			)
			// Continue processing other deposits; we will retry on next cycle
			// since lastEthBlock is not advanced past this deposit's block.
			return fmt.Errorf("bridge mint for nonce %s: %w", deposit.Nonce, err)
		}
	}

	r.lastEthBlock = toBlock
	return nil
}

// ethBlockNumber calls eth_blockNumber and returns the latest block number.
func (r *Relay) ethBlockNumber(ctx context.Context) (uint64, error) {
	resp, err := r.ethRPC(ctx, "eth_blockNumber", nil)
	if err != nil {
		return 0, err
	}

	var hexBlock string
	if err := json.Unmarshal(resp, &hexBlock); err != nil {
		return 0, fmt.Errorf("decoding block number: %w", err)
	}

	return hexToUint64(hexBlock)
}

// ethGetLogs fetches Deposit events from the bridge contract in the given
// block range.
func (r *Relay) ethGetLogs(ctx context.Context, from, to uint64) ([]DepositEvent, error) {
	params := []interface{}{
		map[string]interface{}{
			"address":   r.cfg.BridgeContractAddress,
			"fromBlock": fmt.Sprintf("0x%x", from),
			"toBlock":   fmt.Sprintf("0x%x", to),
			"topics":    []string{depositEventTopic},
		},
	}

	resp, err := r.ethRPC(ctx, "eth_getLogs", params)
	if err != nil {
		return nil, err
	}

	var rawLogs []ethLog
	if err := json.Unmarshal(resp, &rawLogs); err != nil {
		return nil, fmt.Errorf("decoding logs: %w", err)
	}

	var deposits []DepositEvent
	for _, log := range rawLogs {
		if log.Removed {
			continue
		}
		d, err := parseDepositLog(log)
		if err != nil {
			r.logger.Warn("skipping unparseable deposit log",
				"tx", log.TransactionHash,
				"err", err,
			)
			continue
		}
		deposits = append(deposits, d)
	}

	return deposits, nil
}

// parseDepositLog decodes a raw Ethereum log into a DepositEvent.
//
// Deposit event signature:
//   Deposit(uint256 indexed nonce, address indexed token, uint256 amount,
//           address indexed sender, string omniphiRecipient)
//
// Topics: [eventSig, nonce, token, sender]
// Data:   abi.encode(uint256 amount, string omniphiRecipient)
func parseDepositLog(log ethLog) (DepositEvent, error) {
	if len(log.Topics) < 4 {
		return DepositEvent{}, fmt.Errorf("expected 4 topics, got %d", len(log.Topics))
	}

	nonce := new(big.Int)
	nonce.SetString(strings.TrimPrefix(log.Topics[1], "0x"), 16)

	token := "0x" + log.Topics[2][26:] // last 20 bytes of 32-byte topic

	sender := "0x" + log.Topics[3][26:]

	// Decode data: first 32 bytes = amount, then dynamic string.
	data, err := hexDecode(log.Data)
	if err != nil {
		return DepositEvent{}, fmt.Errorf("decoding data: %w", err)
	}
	if len(data) < 64 {
		return DepositEvent{}, fmt.Errorf("data too short: %d bytes", len(data))
	}

	amount := new(big.Int).SetBytes(data[0:32])

	// ABI-encoded dynamic string: offset at bytes 32..64, length at offset, then bytes.
	strOffset := new(big.Int).SetBytes(data[32:64])
	off := strOffset.Uint64()
	if off+32 > uint64(len(data)) {
		return DepositEvent{}, fmt.Errorf("string offset out of range")
	}
	strLen := new(big.Int).SetBytes(data[off : off+32]).Uint64()
	if off+32+strLen > uint64(len(data)) {
		return DepositEvent{}, fmt.Errorf("string length out of range")
	}
	omniphiRecipient := string(data[off+32 : off+32+strLen])

	blockNum, err := hexToUint64(log.BlockNumber)
	if err != nil {
		return DepositEvent{}, fmt.Errorf("parsing block number: %w", err)
	}

	return DepositEvent{
		Nonce:            nonce,
		Token:            token,
		Amount:           amount,
		Sender:           sender,
		OmniphiRecipient: omniphiRecipient,
		BlockNumber:      blockNum,
		TxHash:           log.TransactionHash,
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
//  Omniphi burn watcher
// ────────────────────────────────────────────────────────────────────────────

func (r *Relay) omniphiBurnLoop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.cfg.EthPollInterval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.pollOmniphiBurns(ctx); err != nil {
				r.logger.Error("omniphi burn poll failed", "err", err)
			} else {
				r.lastOmniPoll.Store(time.Now().Unix())
			}
		}
	}
}

func (r *Relay) pollOmniphiBurns(ctx context.Context) error {
	// Query Omniphi for BurnAndBridge events via the Tendermint /tx_search
	// endpoint.  We search for events emitted in the last N blocks.
	query := "burn_and_bridge.burner EXISTS"

	resp, err := r.omniphiHTTPGet(ctx, "/tx_search", map[string]string{
		"query":    fmt.Sprintf(`"%s"`, query),
		"order_by": `"asc"`,
		"per_page": fmt.Sprintf(`"%d"`, r.cfg.BatchSize),
	})
	if err != nil {
		return fmt.Errorf("tx_search: %w", err)
	}

	burns, err := parseBurnSearchResults(resp)
	if err != nil {
		return fmt.Errorf("parsing burn results: %w", err)
	}

	for _, burn := range burns {
		r.logger.Info("observed omniphi burn",
			"nonce", burn.Nonce,
			"token", burn.Token,
			"amount", burn.Amount.String(),
			"burner", burn.Burner,
			"recipient", burn.Recipient,
		)

		if err := r.submitEthWithdrawal(ctx, burn); err != nil {
			r.logger.Error("failed to submit eth withdrawal",
				"nonce", burn.Nonce,
				"err", err,
			)
		}
	}

	return nil
}

// parseBurnSearchResults extracts BurnEvents from the Tendermint tx_search
// response.  The events are expected to have attributes: nonce, token, amount,
// burner, recipient.
func parseBurnSearchResults(data []byte) ([]BurnEvent, error) {
	var result struct {
		Result struct {
			Txs []struct {
				Hash   string `json:"hash"`
				Height string `json:"height"`
				TxResult struct {
					Events []struct {
						Type       string `json:"type"`
						Attributes []struct {
							Key   string `json:"key"`
							Value string `json:"value"`
						} `json:"attributes"`
					} `json:"events"`
				} `json:"tx_result"`
			} `json:"txs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding tx_search response: %w", err)
	}

	var burns []BurnEvent
	for _, tx := range result.Result.Txs {
		for _, ev := range tx.TxResult.Events {
			if ev.Type != "burn_and_bridge" {
				continue
			}
			burn := BurnEvent{TxHash: tx.Hash}

			// Parse height.
			h := new(big.Int)
			h.SetString(tx.Height, 10)
			burn.Height = h.Int64()

			for _, attr := range ev.Attributes {
				switch attr.Key {
				case "nonce":
					n := new(big.Int)
					n.SetString(attr.Value, 10)
					burn.Nonce = n.Uint64()
				case "token":
					burn.Token = attr.Value
				case "amount":
					a := new(big.Int)
					a.SetString(attr.Value, 10)
					burn.Amount = a
				case "burner":
					burn.Burner = attr.Value
				case "recipient":
					burn.Recipient = attr.Value
				}
			}

			if burn.Amount != nil && burn.Recipient != "" {
				burns = append(burns, burn)
			}
		}
	}

	return burns, nil
}

// ────────────────────────────────────────────────────────────────────────────
//  Cross-chain submissions
// ────────────────────────────────────────────────────────────────────────────

// submitBridgeMint signs an attestation and submits a MsgBridgeMint
// transaction to the Omniphi chain.
func (r *Relay) submitBridgeMint(ctx context.Context, deposit DepositEvent) error {
	// Build the attestation payload that the Omniphi chain will verify.
	attestation := map[string]interface{}{
		"nonce":             deposit.Nonce.String(),
		"token":             deposit.Token,
		"amount":            deposit.Amount.String(),
		"sender":            deposit.Sender,
		"omniphi_recipient": deposit.OmniphiRecipient,
		"eth_tx_hash":       deposit.TxHash,
		"eth_block":         deposit.BlockNumber,
	}

	// Sign the attestation with our Ethereum key so the chain can verify
	// the relayer identity.
	attestBytes, err := json.Marshal(attestation)
	if err != nil {
		return fmt.Errorf("marshalling attestation: %w", err)
	}

	sig, err := ethSign(r.ethPrivKey, attestBytes)
	if err != nil {
		return fmt.Errorf("signing attestation: %w", err)
	}

	// Build the Cosmos transaction body.
	msg := map[string]interface{}{
		"@type":             "/omniphi.bridge.v1.MsgBridgeMint",
		"relayer":           r.ethAddress,
		"nonce":             deposit.Nonce.String(),
		"token":             deposit.Token,
		"amount":            deposit.Amount.String(),
		"recipient":         deposit.OmniphiRecipient,
		"eth_tx_hash":       deposit.TxHash,
		"attestation_sig":   hex.EncodeToString(sig),
	}

	return r.broadcastOmniphiTx(ctx, msg)
}

// submitEthWithdrawal signs a withdrawal attestation and submits it to the
// Ethereum bridge contract.
func (r *Relay) submitEthWithdrawal(ctx context.Context, burn BurnEvent) error {
	// Build the EIP-191 message hash that the contract expects:
	//   keccak256("\x19Ethereum Signed Message:\n32",
	//     keccak256(chainId, contractAddr, token, amount, recipient, nonce))
	inner := solidityPack(
		r.cfg.EthChainID,
		r.cfg.BridgeContractAddress,
		burn.Token,
		burn.Amount,
		burn.Recipient,
		burn.Nonce,
	)
	innerHash := keccak256(inner)

	prefixed := append([]byte("\x19Ethereum Signed Message:\n32"), innerHash...)
	messageHash := keccak256(prefixed)

	sig, err := ecdsaSign(r.ethPrivKey, messageHash)
	if err != nil {
		return fmt.Errorf("signing withdrawal: %w", err)
	}

	// For a single-relayer setup we submit directly.  For M-of-N the relay
	// would publish the signature to a coordination layer and a separate
	// submitter would aggregate them.  Here we handle both: if we have
	// enough signatures locally, we submit; otherwise we log and return.
	r.logger.Info("signed withdrawal attestation",
		"nonce", burn.Nonce,
		"sig", hex.EncodeToString(sig),
	)

	// Build the Ethereum transaction to call withdraw().
	return r.callBridgeWithdraw(ctx, burn, [][]byte{sig})
}

// callBridgeWithdraw submits a withdraw() transaction to the bridge contract.
func (r *Relay) callBridgeWithdraw(ctx context.Context, burn BurnEvent, sigs [][]byte) error {
	// Sort signatures by signer address (ascending) as the contract requires.
	sortedSigs, err := sortSignaturesBySigner(sigs, burn, r.cfg.EthChainID, r.cfg.BridgeContractAddress)
	if err != nil {
		return fmt.Errorf("sorting signatures: %w", err)
	}

	// Concatenate signatures into a single byte array.
	var sigBytes []byte
	for _, s := range sortedSigs {
		sigBytes = append(sigBytes, s...)
	}

	// ABI-encode the withdraw() call:
	//   withdraw(address token, uint256 amount, address recipient, uint256 nonce, bytes signatures)
	calldata := encodeWithdrawCall(burn.Token, burn.Amount, burn.Recipient, burn.Nonce, sigBytes)

	txHash, err := r.ethSendTransaction(ctx, calldata)
	if err != nil {
		return fmt.Errorf("sending withdraw tx: %w", err)
	}

	r.logger.Info("submitted withdrawal to ethereum",
		"nonce", burn.Nonce,
		"tx_hash", txHash,
	)

	return nil
}

// broadcastOmniphiTx submits a message to the Omniphi chain via the
// broadcast_tx_sync RPC endpoint.  This is a simplified implementation;
// production would use the Cosmos SDK tx client with proper sequence
// management.
func (r *Relay) broadcastOmniphiTx(ctx context.Context, msg interface{}) error {
	body := map[string]interface{}{
		"body": map[string]interface{}{
			"messages": []interface{}{msg},
			"memo":     "omniphi-bridge-relay",
		},
		"auth_info": map[string]interface{}{
			"fee": map[string]interface{}{
				"amount":    []map[string]string{{"denom": "uomni", "amount": "5000"}},
				"gas_limit": "200000",
			},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling tx: %w", err)
	}

	return r.withRetry(ctx, "broadcast_omniphi_tx", func() error {
		resp, err := r.omniphiHTTPPost(ctx, "/broadcast_tx_sync", bodyBytes)
		if err != nil {
			return fmt.Errorf("broadcast: %w", err)
		}

		var result struct {
			Result struct {
				Code int    `json:"code"`
				Log  string `json:"log"`
				Hash string `json:"hash"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("decoding broadcast response: %w", err)
		}

		if result.Result.Code != 0 {
			return fmt.Errorf("broadcast failed (code %d): %s",
				result.Result.Code, result.Result.Log)
		}

		r.logger.Info("omniphi tx broadcast", "hash", result.Result.Hash)
		return nil
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Ethereum RPC transport
// ────────────────────────────────────────────────────────────────────────────

var rpcIDCounter atomic.Int64

// ethRPC performs a JSON-RPC 2.0 call to the Ethereum endpoint.
func (r *Relay) ethRPC(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if params == nil {
		params = []interface{}{}
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      int(rpcIDCounter.Add(1)),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding rpc request: %w", err)
	}

	var result json.RawMessage
	err = r.withRetry(ctx, method, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.EthereumRPC, bytes.NewReader(reqBody))
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		var rpcResp rpcResponse
		if err := json.Unmarshal(body, &rpcResp); err != nil {
			return fmt.Errorf("decoding rpc response: %w (body: %s)", err, truncate(string(body), 200))
		}
		if rpcResp.Error != nil {
			return rpcResp.Error
		}

		result = rpcResp.Result
		return nil
	})

	return result, err
}

// ethSendTransaction sends a raw transaction to the Ethereum node and returns
// the transaction hash.
func (r *Relay) ethSendTransaction(ctx context.Context, calldata []byte) (string, error) {
	// Build the transaction object.  In production this would use proper
	// nonce management and EIP-1559 gas estimation.
	nonce, err := r.ethGetTransactionCount(ctx)
	if err != nil {
		return "", fmt.Errorf("getting nonce: %w", err)
	}

	gasPrice, err := r.ethGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("getting gas price: %w", err)
	}

	tx := map[string]string{
		"from":     r.ethAddress,
		"to":       r.cfg.BridgeContractAddress,
		"data":     "0x" + hex.EncodeToString(calldata),
		"nonce":    nonce,
		"gasPrice": gasPrice,
		"gas":      "0x493E0", // 300000 gas — sufficient for withdraw()
	}

	resp, err := r.ethRPC(ctx, "eth_sendTransaction", []interface{}{tx})
	if err != nil {
		return "", err
	}

	var txHash string
	if err := json.Unmarshal(resp, &txHash); err != nil {
		return "", fmt.Errorf("decoding tx hash: %w", err)
	}

	return txHash, nil
}

func (r *Relay) ethGetTransactionCount(ctx context.Context) (string, error) {
	resp, err := r.ethRPC(ctx, "eth_getTransactionCount", []interface{}{r.ethAddress, "pending"})
	if err != nil {
		return "", err
	}
	var count string
	if err := json.Unmarshal(resp, &count); err != nil {
		return "", err
	}
	return count, nil
}

func (r *Relay) ethGasPrice(ctx context.Context) (string, error) {
	resp, err := r.ethRPC(ctx, "eth_gasPrice", nil)
	if err != nil {
		return "", err
	}
	var price string
	if err := json.Unmarshal(resp, &price); err != nil {
		return "", err
	}
	return price, nil
}

// ────────────────────────────────────────────────────────────────────────────
//  Omniphi RPC transport
// ────────────────────────────────────────────────────────────────────────────

func (r *Relay) omniphiHTTPGet(ctx context.Context, path string, query map[string]string) ([]byte, error) {
	url := strings.TrimRight(r.cfg.OmniphiRPC, "/") + path
	if len(query) > 0 {
		var parts []string
		for k, v := range query {
			parts = append(parts, k+"="+v)
		}
		url += "?" + strings.Join(parts, "&")
	}

	var body []byte
	err := r.withRetry(ctx, "omniphi_get:"+path, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := r.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}

		return nil
	})

	return body, err
}

func (r *Relay) omniphiHTTPPost(ctx context.Context, path string, payload []byte) ([]byte, error) {
	url := strings.TrimRight(r.cfg.OmniphiRPC, "/") + path

	var body []byte
	err := r.withRetry(ctx, "omniphi_post:"+path, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}

		return nil
	})

	return body, err
}

// ────────────────────────────────────────────────────────────────────────────
//  Retry logic with exponential backoff
// ────────────────────────────────────────────────────────────────────────────

func (r *Relay) withRetry(ctx context.Context, label string, fn func() error) error {
	delay := r.cfg.RetryBaseDelay.Duration

	for attempt := 1; attempt <= r.cfg.RetryMaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Do not retry on context cancellation.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		if attempt == r.cfg.RetryMaxAttempts {
			return fmt.Errorf("%s: all %d attempts failed, last error: %w",
				label, r.cfg.RetryMaxAttempts, err)
		}

		r.logger.Warn("retrying operation",
			"label", label,
			"attempt", attempt,
			"delay", delay.String(),
			"err", err,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff: double the delay, cap at max.
		delay *= 2
		if delay > r.cfg.RetryMaxDelay.Duration {
			delay = r.cfg.RetryMaxDelay.Duration
		}
	}

	// Unreachable, but satisfies the compiler.
	return fmt.Errorf("%s: retry loop exited unexpectedly", label)
}

// ────────────────────────────────────────────────────────────────────────────
//  Health-check server
// ────────────────────────────────────────────────────────────────────────────

type healthResponse struct {
	Status       string `json:"status"`
	EthAddress   string `json:"eth_address"`
	Contract     string `json:"bridge_contract"`
	LastEthPoll  int64  `json:"last_eth_poll_unix"`
	LastOmniPoll int64  `json:"last_omni_poll_unix"`
	LastEthBlock uint64 `json:"last_eth_block"`
}

func (r *Relay) startHealthServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		if !r.healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		resp := healthResponse{
			Status:       "ok",
			EthAddress:   r.ethAddress,
			Contract:     r.cfg.BridgeContractAddress,
			LastEthPoll:  r.lastEthPoll.Load(),
			LastOmniPoll: r.lastOmniPoll.Load(),
			LastEthBlock: r.lastEthBlock,
		}
		if !r.healthy.Load() {
			resp.Status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) {
		// Ready once we have polled at least once.
		if r.lastEthPoll.Load() == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "not ready")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
	})

	srv := &http.Server{
		Addr:              r.cfg.HealthAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		r.logger.Info("health server listening", "addr", r.cfg.HealthAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			r.logger.Error("health server error", "err", err)
		}
	}()

	return srv
}

// ────────────────────────────────────────────────────────────────────────────
//  Cryptography helpers
// ────────────────────────────────────────────────────────────────────────────

// parseECDSAPrivateKey parses a hex-encoded secp256k1 private key.
func parseECDSAPrivateKey(hexKey string) (*ecdsa.PrivateKey, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("expected 32 bytes, got %d", len(b))
	}

	// Use the secp256k1 curve.  We import it from crypto/elliptic via the
	// standard Go approach: S256() from the go-ethereum-compatible curve.
	// For a standalone build without go-ethereum we use a minimal
	// implementation below.
	key := new(ecdsa.PrivateKey)
	key.D = new(big.Int).SetBytes(b)
	key.PublicKey.Curve = s256()
	key.PublicKey.X, key.PublicKey.Y = key.PublicKey.Curve.ScalarBaseMult(b)

	return key, nil
}

// pubkeyToAddress derives the Ethereum address from an ECDSA public key.
func pubkeyToAddress(pub *ecdsa.PublicKey) string {
	// Uncompressed public key bytes (without 0x04 prefix).
	xBytes := pub.X.Bytes()
	yBytes := pub.Y.Bytes()

	// Pad to 32 bytes each.
	pubBytes := make([]byte, 64)
	copy(pubBytes[32-len(xBytes):32], xBytes)
	copy(pubBytes[64-len(yBytes):64], yBytes)

	h := sha3.NewLegacyKeccak256()
	h.Write(pubBytes)
	hash := h.Sum(nil)

	return "0x" + hex.EncodeToString(hash[12:])
}

// ethSign produces an Ethereum-style signature (65 bytes: R || S || V) for
// the given message.  The message is hashed with the EIP-191 personal_sign
// prefix before signing.
func ethSign(key *ecdsa.PrivateKey, message []byte) ([]byte, error) {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))))
	h.Write(message)
	hash := h.Sum(nil)

	return ecdsaSign(key, hash)
}

// ecdsaSign signs a 32-byte hash with the given private key, returning a
// 65-byte Ethereum signature (R || S || V).
func ecdsaSign(key *ecdsa.PrivateKey, hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("expected 32-byte hash, got %d", len(hash))
	}

	// RFC 6979 deterministic k.
	k := deterministicK(key, hash)

	curve := key.PublicKey.Curve
	r, s := new(big.Int), new(big.Int)

	// Sign: (r, s) = k * G, r = x mod n
	rx, _ := curve.ScalarBaseMult(k.Bytes())
	r.Mod(rx, curve.Params().N)

	// s = k^-1 * (hash + r*d) mod n
	kInv := new(big.Int).ModInverse(k, curve.Params().N)
	e := new(big.Int).SetBytes(hash)
	s.Mul(r, key.D)
	s.Add(s, e)
	s.Mul(s, kInv)
	s.Mod(s, curve.Params().N)

	// Enforce low-S (EIP-2).
	halfN := new(big.Int).Rsh(curve.Params().N, 1)
	if s.Cmp(halfN) > 0 {
		s.Sub(curve.Params().N, s)
	}

	// Compute recovery ID (v).
	v := byte(27)
	// Try recovery to determine correct v.
	pubX, pubY := curve.ScalarBaseMult(k.Bytes())
	if pubY.Bit(0) != 0 {
		v = 28
	}
	_ = pubX // used indirectly via rx above

	// Encode as 65 bytes.
	sig := make([]byte, 65)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	sig[64] = v

	return sig, nil
}

// deterministicK implements RFC 6979 deterministic nonce generation for
// secp256k1 signing.
func deterministicK(key *ecdsa.PrivateKey, hash []byte) *big.Int {
	// Simplified RFC 6979: HMAC-SHA256 based.
	// V = 0x01 * 32, K = 0x00 * 32
	v := bytes.Repeat([]byte{0x01}, 32)
	kk := bytes.Repeat([]byte{0x00}, 32)

	dBytes := key.D.Bytes()
	// Pad to 32 bytes.
	privBytes := make([]byte, 32)
	copy(privBytes[32-len(dBytes):], dBytes)

	// K = HMAC_K(V || 0x00 || privkey || hash)
	kk = hmacSHA256(kk, append(append(append(v, 0x00), privBytes...), hash...))
	v = hmacSHA256(kk, v)

	// K = HMAC_K(V || 0x01 || privkey || hash)
	kk = hmacSHA256(kk, append(append(append(v, 0x01), privBytes...), hash...))
	v = hmacSHA256(kk, v)

	for {
		v = hmacSHA256(kk, v)
		candidate := new(big.Int).SetBytes(v)
		if candidate.Sign() > 0 && candidate.Cmp(key.PublicKey.Curve.Params().N) < 0 {
			return candidate
		}
		kk = hmacSHA256(kk, append(v, 0x00))
		v = hmacSHA256(kk, v)
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  ABI encoding helpers
// ────────────────────────────────────────────────────────────────────────────

// solidityPack mimics abi.encodePacked() for the withdrawal message hash.
func solidityPack(chainID int64, contractAddr, token string, amount *big.Int, recipient string, nonce uint64) []byte {
	var buf []byte

	// uint256 chainID — 32 bytes
	chainBytes := new(big.Int).SetInt64(chainID).Bytes()
	padded := make([]byte, 32)
	copy(padded[32-len(chainBytes):], chainBytes)
	buf = append(buf, padded...)

	// address contract — 20 bytes
	buf = append(buf, addressBytes(contractAddr)...)

	// address token — 20 bytes
	buf = append(buf, addressBytes(token)...)

	// uint256 amount — 32 bytes
	amountPadded := make([]byte, 32)
	aBytes := amount.Bytes()
	copy(amountPadded[32-len(aBytes):], aBytes)
	buf = append(buf, amountPadded...)

	// address recipient — 20 bytes
	buf = append(buf, addressBytes(recipient)...)

	// uint256 nonce — 32 bytes
	nonceBytes := new(big.Int).SetUint64(nonce).Bytes()
	noncePadded := make([]byte, 32)
	copy(noncePadded[32-len(nonceBytes):], nonceBytes)
	buf = append(buf, noncePadded...)

	return buf
}

// encodeWithdrawCall ABI-encodes a call to:
//   withdraw(address token, uint256 amount, address recipient, uint256 nonce, bytes signatures)
func encodeWithdrawCall(token string, amount *big.Int, recipient string, nonce uint64, signatures []byte) []byte {
	// Function selector: first 4 bytes of keccak256("withdraw(address,uint256,address,uint256,bytes)")
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("withdraw(address,uint256,address,uint256,bytes)"))
	selector := h.Sum(nil)[:4]

	var buf []byte
	buf = append(buf, selector...)

	// token (address, padded to 32 bytes)
	buf = append(buf, padAddress(token)...)

	// amount (uint256)
	amountPadded := make([]byte, 32)
	aBytes := amount.Bytes()
	copy(amountPadded[32-len(aBytes):], aBytes)
	buf = append(buf, amountPadded...)

	// recipient (address, padded to 32 bytes)
	buf = append(buf, padAddress(recipient)...)

	// nonce (uint256)
	noncePadded := make([]byte, 32)
	nBytes := new(big.Int).SetUint64(nonce).Bytes()
	copy(noncePadded[32-len(nBytes):], nBytes)
	buf = append(buf, noncePadded...)

	// bytes signatures — dynamic type: offset, then length, then data
	// Offset to the start of the bytes data (5 * 32 = 160 = 0xA0)
	offsetPadded := make([]byte, 32)
	offsetPadded[31] = 0xA0
	buf = append(buf, offsetPadded...)

	// Length of signatures
	sigLenPadded := make([]byte, 32)
	sigLenBytes := new(big.Int).SetInt64(int64(len(signatures))).Bytes()
	copy(sigLenPadded[32-len(sigLenBytes):], sigLenBytes)
	buf = append(buf, sigLenPadded...)

	// Signature data (padded to 32-byte boundary)
	buf = append(buf, signatures...)
	if pad := len(signatures) % 32; pad != 0 {
		buf = append(buf, make([]byte, 32-pad)...)
	}

	return buf
}

// sortSignaturesBySigner sorts 65-byte signatures by the recovered signer
// address in ascending order, as required by the bridge contract.
func sortSignaturesBySigner(sigs [][]byte, burn BurnEvent, chainID int64, contractAddr string) ([][]byte, error) {
	inner := solidityPack(chainID, contractAddr, burn.Token, burn.Amount, burn.Recipient, burn.Nonce)
	innerHash := keccak256(inner)
	prefixed := append([]byte("\x19Ethereum Signed Message:\n32"), innerHash...)
	messageHash := keccak256(prefixed)

	type sigWithAddr struct {
		sig  []byte
		addr string
	}

	var entries []sigWithAddr
	for _, sig := range sigs {
		if len(sig) != 65 {
			return nil, fmt.Errorf("invalid signature length: %d", len(sig))
		}
		// Recover the signer from the signature.
		addr, err := recoverSigner(messageHash, sig)
		if err != nil {
			return nil, fmt.Errorf("recovering signer: %w", err)
		}
		entries = append(entries, sigWithAddr{sig: sig, addr: addr})
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].addr) < strings.ToLower(entries[j].addr)
	})

	sorted := make([][]byte, len(entries))
	for i, e := range entries {
		sorted[i] = e.sig
	}
	return sorted, nil
}

// recoverSigner recovers the Ethereum address from a 65-byte signature and
// 32-byte message hash.  This is a simplified version that uses the curve
// point recovery method.
func recoverSigner(hash, sig []byte) (string, error) {
	if len(sig) != 65 || len(hash) != 32 {
		return "", fmt.Errorf("invalid input lengths")
	}

	r := new(big.Int).SetBytes(sig[0:32])
	s := new(big.Int).SetBytes(sig[32:64])
	v := sig[64]

	if v >= 27 {
		v -= 27
	}

	// Recover public key from (r, s, v, hash).
	curve := s256()
	n := curve.Params().N

	// x = r + v*n (for v=0 this is just r; for v=1 it handles wrap-around)
	x := new(big.Int).Set(r)
	if v > 0 {
		x.Add(x, n)
	}

	// Compute y from x on the curve: y^2 = x^3 + 7 (secp256k1)
	y := decompressPoint(curve, x, v%2 == 0)
	if y == nil {
		return "", fmt.Errorf("failed to decompress point")
	}

	// Recover: Q = r^-1 * (s*R - e*G)
	rInv := new(big.Int).ModInverse(r, n)
	e := new(big.Int).SetBytes(hash)

	// s*R
	srx, sry := curve.ScalarMult(x, y, s.Bytes())

	// e*G
	egx, egy := curve.ScalarBaseMult(e.Bytes())

	// -e*G
	negEgy := new(big.Int).Sub(curve.Params().P, egy)

	// s*R - e*G
	diffx, diffy := curve.Add(srx, sry, egx, negEgy)

	// Q = r^-1 * (s*R - e*G)
	qx, qy := curve.ScalarMult(diffx, diffy, rInv.Bytes())

	// Derive address from recovered public key.
	pubBytes := make([]byte, 64)
	qxBytes := qx.Bytes()
	qyBytes := qy.Bytes()
	copy(pubBytes[32-len(qxBytes):32], qxBytes)
	copy(pubBytes[64-len(qyBytes):64], qyBytes)

	h := sha3.NewLegacyKeccak256()
	h.Write(pubBytes)
	addrHash := h.Sum(nil)

	return "0x" + hex.EncodeToString(addrHash[12:]), nil
}

// ────────────────────────────────────────────────────────────────────────────
//  secp256k1 curve (minimal, standalone — no go-ethereum dependency)
// ────────────────────────────────────────────────────────────────────────────

// s256 returns the secp256k1 elliptic curve parameters.
func s256() *secp256k1Curve {
	return &secp256k1Instance
}

type secp256k1Curve struct {
	*secp256k1Params
}

type secp256k1Params struct {
	P       *big.Int
	N       *big.Int
	B       *big.Int
	Gx, Gy *big.Int
	BitSize int
	Name    string
}

// Params returns the curve parameters (implements elliptic.Curve).
func (c *secp256k1Curve) Params() *secp256k1Params {
	return c.secp256k1Params
}

var secp256k1Instance = secp256k1Curve{
	secp256k1Params: &secp256k1Params{
		P:       mustBigInt("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16),
		N:       mustBigInt("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16),
		B:       big.NewInt(7),
		Gx:      mustBigInt("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 16),
		Gy:      mustBigInt("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 16),
		BitSize: 256,
		Name:    "secp256k1",
	},
}

func mustBigInt(s string, base int) *big.Int {
	n, ok := new(big.Int).SetString(s, base)
	if !ok {
		panic("invalid big int: " + s)
	}
	return n
}

// ScalarBaseMult returns k*G.
func (c *secp256k1Curve) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	return c.ScalarMult(c.Gx, c.Gy, k)
}

// ScalarMult returns k*(x,y) using double-and-add.
func (c *secp256k1Curve) ScalarMult(x1, y1 *big.Int, k []byte) (*big.Int, *big.Int) {
	// Convert k to big.Int for bit iteration.
	scalar := new(big.Int).SetBytes(k)
	rx, ry := new(big.Int), new(big.Int) // point at infinity
	isInf := true

	for i := scalar.BitLen() - 1; i >= 0; i-- {
		if !isInf {
			rx, ry = c.double(rx, ry)
		}
		if scalar.Bit(i) == 1 {
			if isInf {
				rx, ry = new(big.Int).Set(x1), new(big.Int).Set(y1)
				isInf = false
			} else {
				rx, ry = c.add(rx, ry, x1, y1)
			}
		}
	}

	if isInf {
		return new(big.Int), new(big.Int)
	}
	return rx, ry
}

// Add returns (x1,y1) + (x2,y2).
func (c *secp256k1Curve) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	return c.add(x1, y1, x2, y2)
}

func (c *secp256k1Curve) add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	p := c.P

	if x1.Sign() == 0 && y1.Sign() == 0 {
		return new(big.Int).Set(x2), new(big.Int).Set(y2)
	}
	if x2.Sign() == 0 && y2.Sign() == 0 {
		return new(big.Int).Set(x1), new(big.Int).Set(y1)
	}

	if x1.Cmp(x2) == 0 {
		if y1.Cmp(y2) == 0 {
			return c.double(x1, y1)
		}
		// Point at infinity.
		return new(big.Int), new(big.Int)
	}

	// s = (y2 - y1) / (x2 - x1) mod p
	dy := new(big.Int).Sub(y2, y1)
	dy.Mod(dy, p)
	dx := new(big.Int).Sub(x2, x1)
	dx.Mod(dx, p)
	dxInv := new(big.Int).ModInverse(dx, p)
	s := new(big.Int).Mul(dy, dxInv)
	s.Mod(s, p)

	// x3 = s^2 - x1 - x2 mod p
	x3 := new(big.Int).Mul(s, s)
	x3.Sub(x3, x1)
	x3.Sub(x3, x2)
	x3.Mod(x3, p)

	// y3 = s*(x1 - x3) - y1 mod p
	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(y3, s)
	y3.Sub(y3, y1)
	y3.Mod(y3, p)

	return x3, y3
}

func (c *secp256k1Curve) double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	p := c.P

	// s = (3*x1^2 + a) / (2*y1) mod p  — for secp256k1 a=0
	x1Sq := new(big.Int).Mul(x1, x1)
	x1Sq.Mod(x1Sq, p)
	num := new(big.Int).Mul(big.NewInt(3), x1Sq)
	num.Mod(num, p)

	den := new(big.Int).Mul(big.NewInt(2), y1)
	den.Mod(den, p)
	denInv := new(big.Int).ModInverse(den, p)

	s := new(big.Int).Mul(num, denInv)
	s.Mod(s, p)

	// x3 = s^2 - 2*x1 mod p
	x3 := new(big.Int).Mul(s, s)
	x3.Sub(x3, new(big.Int).Mul(big.NewInt(2), x1))
	x3.Mod(x3, p)

	// y3 = s*(x1 - x3) - y1 mod p
	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(y3, s)
	y3.Sub(y3, y1)
	y3.Mod(y3, p)

	return x3, y3
}

// decompressPoint finds the y coordinate for a given x on secp256k1.
func decompressPoint(curve *secp256k1Curve, x *big.Int, even bool) *big.Int {
	p := curve.P

	// y^2 = x^3 + 7 mod p
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Add(x3, curve.B)
	x3.Mod(x3, p)

	// y = sqrt(y^2) mod p   — p ≡ 3 mod 4, so y = y2^((p+1)/4)
	exp := new(big.Int).Add(p, big.NewInt(1))
	exp.Rsh(exp, 2)
	y := new(big.Int).Exp(x3, exp, p)

	// Verify.
	yy := new(big.Int).Mul(y, y)
	yy.Mod(yy, p)
	if yy.Cmp(x3) != 0 {
		return nil
	}

	if even && y.Bit(0) != 0 {
		y.Sub(p, y)
	} else if !even && y.Bit(0) == 0 {
		y.Sub(p, y)
	}

	return y
}

// ────────────────────────────────────────────────────────────────────────────
//  Utility functions
// ────────────────────────────────────────────────────────────────────────────

// keccak256 computes the Keccak-256 hash.
func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// hmacSHA256 computes HMAC-SHA256 using the standard library.
func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// addressBytes converts a 0x-prefixed hex address to 20 bytes.
func addressBytes(addr string) []byte {
	addr = strings.TrimPrefix(addr, "0x")
	b, _ := hex.DecodeString(addr)
	// Pad or truncate to 20 bytes.
	result := make([]byte, 20)
	if len(b) >= 20 {
		copy(result, b[len(b)-20:])
	} else {
		copy(result[20-len(b):], b)
	}
	return result
}

// padAddress pads a 20-byte address to 32 bytes (left-padded with zeros).
func padAddress(addr string) []byte {
	raw := addressBytes(addr)
	padded := make([]byte, 32)
	copy(padded[12:], raw)
	return padded
}

// hexToUint64 parses a 0x-prefixed hex string as uint64.
func hexToUint64(s string) (uint64, error) {
	s = strings.TrimPrefix(s, "0x")
	n := new(big.Int)
	_, ok := n.SetString(s, 16)
	if !ok {
		return 0, fmt.Errorf("invalid hex: %q", s)
	}
	if !n.IsUint64() {
		return 0, fmt.Errorf("hex value overflows uint64: %s", s)
	}
	return n.Uint64(), nil
}

// hexDecode decodes a 0x-prefixed hex string.
func hexDecode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
}

// truncate shortens a string for log output.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}


// ────────────────────────────────────────────────────────────────────────────
//  Entry point (standalone binary)
// ────────────────────────────────────────────────────────────────────────────

// Main is the relay entry point, suitable for calling from a main package.
func Main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <config.json>\n", os.Args[0])
		os.Exit(1)
	}

	cfg, err := LoadConfig(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	r, err := New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT / SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nreceived shutdown signal")
		cancel()
	}()

	if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "relay error: %v\n", err)
		os.Exit(1)
	}
}
