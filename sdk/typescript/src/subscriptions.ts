/**
 * WebSocket subscription support for real-time Omniphi chain events.
 *
 * Uses the standard Tendermint JSONRPC WebSocket protocol to subscribe to
 * block, transaction, and custom event streams.
 *
 * @example
 * ```ts
 * const subscriber = new OmniphiSubscriber("ws://localhost:26657/websocket");
 * await subscriber.connect();
 *
 * subscriber.subscribeNewBlocks((event) => {
 *   console.log("New block:", event.height);
 * });
 *
 * subscriber.subscribeTx("message.module='poc'", (event) => {
 *   console.log("PoC transaction:", event.hash);
 * });
 * ```
 */

// ---------------------------------------------------------------------------
// Event types
// ---------------------------------------------------------------------------

/** Data from a new block event. */
export interface NewBlockEvent {
  /** Block height. */
  height: number;
  /** Block hash (hex-encoded). */
  hash: string;
  /** Number of transactions in the block. */
  numTxs: number;
  /** Block timestamp (ISO-8601 string). */
  time: string;
  /** The proposer address. */
  proposerAddress: string;
  /** Raw result data from the Tendermint response. */
  raw: Record<string, unknown>;
}

/** Data from a new transaction event. */
export interface NewTxEvent {
  /** Transaction hash (hex-encoded). */
  hash: string;
  /** Block height the transaction was included in. */
  height: number;
  /** Transaction result code (0 = success). */
  code: number;
  /** Transaction log. */
  log: string;
  /** Events emitted by the transaction. */
  events: Array<{ type: string; attributes: Array<{ key: string; value: string }> }>;
  /** Raw result data from the Tendermint response. */
  raw: Record<string, unknown>;
}

/** Generic event data for arbitrary event subscriptions. */
export interface EventData {
  /** The query that produced this event. */
  query: string;
  /** Raw event data from the Tendermint WebSocket response. */
  data: Record<string, unknown>;
  /** Events array from the response (if present). */
  events: Record<string, string[]>;
}

// ---------------------------------------------------------------------------
// Callback types
// ---------------------------------------------------------------------------

export type NewBlockCallback = (event: NewBlockEvent) => void;
export type NewTxCallback = (event: NewTxEvent) => void;
export type EventCallback = (event: EventData) => void;

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

/** A single managed subscription. */
interface Subscription {
  id: string;
  query: string;
  callback: NewBlockCallback | NewTxCallback | EventCallback;
  type: "newBlock" | "newTx" | "tx" | "events";
}

/** Tendermint JSONRPC WebSocket response. */
interface TendermintWSResponse {
  id?: string;
  jsonrpc: string;
  result?: Record<string, unknown>;
  error?: { code: number; message: string; data?: string };
}

// ---------------------------------------------------------------------------
// Reconnect config
// ---------------------------------------------------------------------------

export interface ReconnectOptions {
  /** Whether to auto-reconnect on connection loss. Defaults to true. */
  enabled: boolean;
  /** Initial delay in ms before first reconnect attempt. Defaults to 1000. */
  initialDelayMs: number;
  /** Maximum delay in ms between reconnect attempts. Defaults to 30000. */
  maxDelayMs: number;
  /** Backoff multiplier. Defaults to 2. */
  multiplier: number;
  /** Maximum number of reconnect attempts. 0 = unlimited. Defaults to 0. */
  maxAttempts: number;
}

const DEFAULT_RECONNECT: ReconnectOptions = {
  enabled: true,
  initialDelayMs: 1000,
  maxDelayMs: 30000,
  multiplier: 2,
  maxAttempts: 0,
};

// ---------------------------------------------------------------------------
// WebSocket abstraction (allows mocking in tests)
// ---------------------------------------------------------------------------

/**
 * Minimal WebSocket interface that OmniphiSubscriber depends on.
 * Compatible with both the browser `WebSocket` and Node.js `ws` module.
 */
export interface IWebSocket {
  readonly readyState: number;
  onopen: ((ev: unknown) => void) | null;
  onclose: ((ev: unknown) => void) | null;
  onmessage: ((ev: { data: unknown }) => void) | null;
  onerror: ((ev: unknown) => void) | null;
  send(data: string): void;
  close(code?: number, reason?: string): void;
}

export type WebSocketFactory = (url: string) => IWebSocket;

// ---------------------------------------------------------------------------
// OmniphiSubscriber
// ---------------------------------------------------------------------------

/**
 * Real-time WebSocket subscriber for Tendermint events.
 *
 * Implements the Tendermint JSONRPC WebSocket protocol for subscribing to
 * new blocks, transactions, and arbitrary chain events.
 */
export class OmniphiSubscriber {
  /** WebSocket endpoint URL. */
  private readonly url: string;

  /** Reconnection options. */
  private readonly reconnectOpts: ReconnectOptions;

  /** Factory for creating WebSocket instances (injectable for testing). */
  private readonly wsFactory: WebSocketFactory;

  /** Current WebSocket connection. */
  private ws: IWebSocket | null = null;

  /** Active subscriptions keyed by subscription ID. */
  private subscriptions: Map<string, Subscription> = new Map();

  /** Monotonically increasing ID for JSONRPC requests. */
  private nextRequestId = 1;

  /** Maps JSONRPC request ID to the subscription ID (for correlating responses). */
  private pendingRequests: Map<string, string> = new Map();

  /** Current reconnect attempt count. */
  private reconnectAttempts = 0;

  /** Timer handle for scheduled reconnect. */
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  /** Whether `disconnect()` was called explicitly (suppresses reconnect). */
  private intentionalDisconnect = false;

  /** Callback for connection state changes. */
  private onConnectionChange: ((connected: boolean) => void) | null = null;

  /** Callback for errors. */
  private onError: ((error: Error) => void) | null = null;

  /**
   * Creates a new subscriber.
   *
   * @param url             - Tendermint WebSocket URL (e.g., "ws://localhost:26657/websocket").
   * @param reconnectOpts   - Reconnection behavior options.
   * @param wsFactory       - Optional factory for creating WebSocket instances (used for testing).
   */
  constructor(
    url: string = "ws://localhost:26657/websocket",
    reconnectOpts: Partial<ReconnectOptions> = {},
    wsFactory?: WebSocketFactory,
  ) {
    this.url = url;
    this.reconnectOpts = { ...DEFAULT_RECONNECT, ...reconnectOpts };
    this.wsFactory = wsFactory ?? defaultWebSocketFactory;
  }

  // -----------------------------------------------------------------------
  // Lifecycle
  // -----------------------------------------------------------------------

  /**
   * Opens the WebSocket connection.
   *
   * @returns A promise that resolves when the connection is established,
   *          or rejects if the initial connection fails.
   */
  connect(): Promise<void> {
    this.intentionalDisconnect = false;
    return this.doConnect();
  }

  /**
   * Closes the WebSocket connection and cleans up all subscriptions.
   * Suppresses auto-reconnect.
   */
  disconnect(): void {
    this.intentionalDisconnect = true;
    this.clearReconnectTimer();
    if (this.ws) {
      // Clear handlers before closing to avoid triggering reconnect
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      this.ws.onopen = null;
      this.ws.close(1000, "client disconnect");
      this.ws = null;
    }
    this.subscriptions.clear();
    this.pendingRequests.clear();
  }

  /**
   * Whether the WebSocket is currently connected.
   */
  get connected(): boolean {
    return this.ws !== null && this.ws.readyState === 1; // OPEN
  }

  // -----------------------------------------------------------------------
  // Event handlers
  // -----------------------------------------------------------------------

  /**
   * Registers a callback invoked when the connection state changes.
   */
  onConnectionStateChange(callback: (connected: boolean) => void): void {
    this.onConnectionChange = callback;
  }

  /**
   * Registers a callback invoked on WebSocket or protocol errors.
   */
  onSubscriptionError(callback: (error: Error) => void): void {
    this.onError = callback;
  }

  // -----------------------------------------------------------------------
  // Subscribe methods
  // -----------------------------------------------------------------------

  /**
   * Subscribes to new block events.
   *
   * @param callback - Invoked for each new block.
   * @returns A subscription ID that can be used with `unsubscribe()`.
   */
  subscribeNewBlocks(callback: NewBlockCallback): string {
    return this.addSubscription("tm.event='NewBlock'", callback, "newBlock");
  }

  /**
   * Subscribes to all new transactions.
   *
   * @param callback - Invoked for each new transaction.
   * @returns A subscription ID.
   */
  subscribeNewTxs(callback: NewTxCallback): string {
    return this.addSubscription("tm.event='Tx'", callback, "newTx");
  }

  /**
   * Subscribes to transactions matching a specific query.
   *
   * @param query    - Tendermint event query (e.g., "message.module='poc'").
   * @param callback - Invoked for each matching transaction.
   * @returns A subscription ID.
   *
   * @example
   * ```ts
   * subscriber.subscribeTx("message.module='poc'", (event) => {
   *   console.log("PoC tx:", event.hash);
   * });
   * ```
   */
  subscribeTx(query: string, callback: NewTxCallback): string {
    const fullQuery = `tm.event='Tx' AND ${query}`;
    return this.addSubscription(fullQuery, callback, "tx");
  }

  /**
   * Subscribes to arbitrary events matching a Tendermint query.
   *
   * @param query    - Tendermint event query string.
   * @param callback - Invoked for each matching event.
   * @returns A subscription ID.
   */
  subscribeEvents(query: string, callback: EventCallback): string {
    return this.addSubscription(query, callback, "events");
  }

  /**
   * Unsubscribes from a previously registered subscription.
   *
   * @param subscriptionId - The ID returned by a subscribe method.
   */
  unsubscribe(subscriptionId: string): void {
    const sub = this.subscriptions.get(subscriptionId);
    if (!sub) {
      return;
    }

    // Send unsubscribe to server
    if (this.connected && this.ws) {
      const reqId = String(this.nextRequestId++);
      const msg = JSON.stringify({
        jsonrpc: "2.0",
        id: reqId,
        method: "unsubscribe",
        params: { query: sub.query },
      });
      this.ws.send(msg);
    }

    this.subscriptions.delete(subscriptionId);
  }

  /**
   * Returns the number of active subscriptions.
   */
  get subscriptionCount(): number {
    return this.subscriptions.size;
  }

  // -----------------------------------------------------------------------
  // Internal: connection management
  // -----------------------------------------------------------------------

  private doConnect(): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      try {
        this.ws = this.wsFactory(this.url);
      } catch (err) {
        reject(new Error(`Failed to create WebSocket: ${err}`));
        return;
      }

      this.ws.onopen = () => {
        this.reconnectAttempts = 0;
        this.onConnectionChange?.(true);
        // Re-subscribe any existing subscriptions (after reconnect)
        this.resubscribeAll();
        resolve();
      };

      this.ws.onclose = () => {
        this.onConnectionChange?.(false);
        if (!this.intentionalDisconnect) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = (ev) => {
        const error = new Error(`WebSocket error: ${ev}`);
        this.onError?.(error);
        // If this is the initial connection attempt, reject the promise
        if (this.reconnectAttempts === 0 && !this.connected) {
          reject(error);
        }
      };

      this.ws.onmessage = (ev) => {
        this.handleMessage(ev.data);
      };
    });
  }

  private scheduleReconnect(): void {
    if (!this.reconnectOpts.enabled) {
      return;
    }
    if (this.reconnectOpts.maxAttempts > 0 && this.reconnectAttempts >= this.reconnectOpts.maxAttempts) {
      this.onError?.(new Error(`Max reconnect attempts (${this.reconnectOpts.maxAttempts}) exceeded`));
      return;
    }

    const delay = Math.min(
      this.reconnectOpts.initialDelayMs * Math.pow(this.reconnectOpts.multiplier, this.reconnectAttempts),
      this.reconnectOpts.maxDelayMs,
    );

    this.reconnectAttempts++;

    this.reconnectTimer = setTimeout(() => {
      this.doConnect().catch((err) => {
        this.onError?.(new Error(`Reconnect attempt ${this.reconnectAttempts} failed: ${err}`));
      });
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private resubscribeAll(): void {
    for (const [id, sub] of this.subscriptions.entries()) {
      this.sendSubscribe(sub.query, id);
    }
  }

  // -----------------------------------------------------------------------
  // Internal: subscription management
  // -----------------------------------------------------------------------

  private addSubscription(
    query: string,
    callback: NewBlockCallback | NewTxCallback | EventCallback,
    type: Subscription["type"],
  ): string {
    const subId = `sub_${this.nextRequestId++}`;

    const subscription: Subscription = {
      id: subId,
      query,
      callback,
      type,
    };

    this.subscriptions.set(subId, subscription);

    // Send subscribe to server if connected
    if (this.connected) {
      this.sendSubscribe(query, subId);
    }

    return subId;
  }

  private sendSubscribe(query: string, subId: string): void {
    if (!this.ws || !this.connected) {
      return;
    }

    const reqId = String(this.nextRequestId++);
    this.pendingRequests.set(reqId, subId);

    const msg = JSON.stringify({
      jsonrpc: "2.0",
      id: reqId,
      method: "subscribe",
      params: { query },
    });

    this.ws.send(msg);
  }

  // -----------------------------------------------------------------------
  // Internal: message handling
  // -----------------------------------------------------------------------

  private handleMessage(raw: unknown): void {
    let data: TendermintWSResponse;
    try {
      const str = typeof raw === "string" ? raw : String(raw);
      data = JSON.parse(str) as TendermintWSResponse;
    } catch {
      this.onError?.(new Error(`Failed to parse WebSocket message: ${raw}`));
      return;
    }

    // Handle errors
    if (data.error) {
      this.onError?.(new Error(`Tendermint RPC error: ${data.error.message} (code ${data.error.code})`));
      return;
    }

    // Handle subscription confirmation (has an id, result is empty or contains subscription info)
    if (data.id && this.pendingRequests.has(data.id)) {
      this.pendingRequests.delete(data.id);
      return;
    }

    // Handle event notification (no id, has result with query and data)
    if (data.result && typeof data.result === "object") {
      this.dispatchEvent(data.result);
    }
  }

  private dispatchEvent(result: Record<string, unknown>): void {
    const query = result.query as string | undefined;
    const eventData = result.data as Record<string, unknown> | undefined;
    const events = result.events as Record<string, string[]> | undefined;

    if (!query || !eventData) {
      return;
    }

    // Find matching subscriptions by query
    for (const sub of this.subscriptions.values()) {
      if (sub.query !== query) {
        continue;
      }

      switch (sub.type) {
        case "newBlock":
          (sub.callback as NewBlockCallback)(this.parseNewBlockEvent(eventData, result));
          break;
        case "newTx":
        case "tx":
          (sub.callback as NewTxCallback)(this.parseNewTxEvent(eventData, result));
          break;
        case "events":
          (sub.callback as EventCallback)({
            query,
            data: eventData,
            events: events ?? {},
          });
          break;
      }
    }
  }

  private parseNewBlockEvent(data: Record<string, unknown>, raw: Record<string, unknown>): NewBlockEvent {
    const value = (data.value ?? data) as Record<string, unknown>;
    const block = (value.block ?? {}) as Record<string, unknown>;
    const header = (block.header ?? {}) as Record<string, unknown>;

    return {
      height: typeof header.height === "string" ? parseInt(header.height, 10) : (header.height as number ?? 0),
      hash: (value.block_id as Record<string, unknown>)?.hash as string ?? "",
      numTxs: typeof header.num_txs === "string" ? parseInt(header.num_txs, 10) : (header.num_txs as number ?? 0),
      time: header.time as string ?? "",
      proposerAddress: header.proposer_address as string ?? "",
      raw,
    };
  }

  private parseNewTxEvent(data: Record<string, unknown>, raw: Record<string, unknown>): NewTxEvent {
    const value = (data.value ?? data) as Record<string, unknown>;
    const txResult = (value.TxResult ?? value.tx_result ?? {}) as Record<string, unknown>;
    const resultField = (txResult.result ?? {}) as Record<string, unknown>;

    return {
      hash: txResult.hash as string ?? value.hash as string ?? "",
      height: typeof txResult.height === "string" ? parseInt(txResult.height, 10) : (txResult.height as number ?? 0),
      code: resultField.code as number ?? 0,
      log: resultField.log as string ?? "",
      events: (resultField.events ?? []) as Array<{ type: string; attributes: Array<{ key: string; value: string }> }>,
      raw,
    };
  }

  // -----------------------------------------------------------------------
  // Static helpers
  // -----------------------------------------------------------------------

  /**
   * Builds a Tendermint subscribe message in JSONRPC format.
   * Useful for testing or manual WebSocket interaction.
   *
   * @param query - The event query string.
   * @param id    - Optional JSONRPC request ID.
   */
  static buildSubscribeMessage(query: string, id: string = "1"): string {
    return JSON.stringify({
      jsonrpc: "2.0",
      id,
      method: "subscribe",
      params: { query },
    });
  }

  /**
   * Builds a Tendermint unsubscribe message in JSONRPC format.
   *
   * @param query - The event query string to unsubscribe from.
   * @param id    - Optional JSONRPC request ID.
   */
  static buildUnsubscribeMessage(query: string, id: string = "1"): string {
    return JSON.stringify({
      jsonrpc: "2.0",
      id,
      method: "unsubscribe",
      params: { query },
    });
  }
}

// ---------------------------------------------------------------------------
// Default WebSocket factory
// ---------------------------------------------------------------------------

/**
 * Default factory that creates a WebSocket using the global `WebSocket`
 * constructor (available in browsers and Node 21+).
 */
function defaultWebSocketFactory(url: string): IWebSocket {
  // Try the global WebSocket first (browser / Node 21+)
  if (typeof WebSocket !== "undefined") {
    return new WebSocket(url) as unknown as IWebSocket;
  }
  throw new Error(
    "No WebSocket implementation available. " +
      "In Node.js < 21, install the 'ws' package and pass a factory: " +
      "new OmniphiSubscriber(url, {}, (u) => new (require('ws'))(u))",
  );
}
