/**
 * Tests for OmniphiSubscriber — WebSocket subscription construction,
 * message formats, event dispatching, and reconnect logic.
 *
 * Uses a mock WebSocket to test all behavior deterministically.
 */

import {
  OmniphiSubscriber,
  type IWebSocket,
  type WebSocketFactory,
  type NewBlockEvent,
  type NewTxEvent,
  type EventData,
} from "../src/subscriptions";

// ---------------------------------------------------------------------------
// Mock WebSocket
// ---------------------------------------------------------------------------

class MockWebSocket implements IWebSocket {
  readyState: number = 0; // CONNECTING

  onopen: ((ev: unknown) => void) | null = null;
  onclose: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: { data: unknown }) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;

  sent: string[] = [];
  closed = false;
  closeCode?: number;

  send(data: string): void {
    this.sent.push(data);
  }

  close(code?: number, _reason?: string): void {
    this.closed = true;
    this.closeCode = code;
  }

  // Test helpers

  simulateOpen(): void {
    this.readyState = 1; // OPEN
    this.onopen?.({});
  }

  simulateClose(): void {
    this.readyState = 3; // CLOSED
    this.onclose?.({});
  }

  simulateError(msg: string): void {
    this.onerror?.(msg);
  }

  simulateMessage(data: unknown): void {
    this.onmessage?.({ data: typeof data === "string" ? data : JSON.stringify(data) });
  }
}

// ---------------------------------------------------------------------------
// Helper to create subscriber with mock
// ---------------------------------------------------------------------------

function createTestSubscriber(
  opts: { reconnect?: boolean; maxAttempts?: number } = {},
): { subscriber: OmniphiSubscriber; mockWs: MockWebSocket; factory: WebSocketFactory } {
  let mockWs: MockWebSocket = new MockWebSocket();

  const factory: WebSocketFactory = (_url: string) => {
    mockWs = new MockWebSocket();
    return mockWs;
  };

  const subscriber = new OmniphiSubscriber(
    "ws://localhost:26657/websocket",
    {
      enabled: opts.reconnect ?? false,
      maxAttempts: opts.maxAttempts ?? 3,
      initialDelayMs: 10,
      maxDelayMs: 50,
      multiplier: 2,
    },
    factory,
  );

  return { subscriber, get mockWs() { return mockWs; }, factory };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("OmniphiSubscriber", () => {
  // -----------------------------------------------------------------------
  // Construction
  // -----------------------------------------------------------------------

  describe("construction", () => {
    test("creates subscriber with default URL", () => {
      const sub = new OmniphiSubscriber();
      expect(sub).toBeDefined();
      expect(sub.connected).toBe(false);
    });

    test("creates subscriber with custom URL", () => {
      const sub = new OmniphiSubscriber("ws://custom:26657/websocket");
      expect(sub).toBeDefined();
    });

    test("creates subscriber with custom reconnect options", () => {
      const sub = new OmniphiSubscriber("ws://localhost:26657/websocket", {
        enabled: false,
        maxAttempts: 5,
      });
      expect(sub).toBeDefined();
    });

    test("initially not connected", () => {
      const { subscriber } = createTestSubscriber();
      expect(subscriber.connected).toBe(false);
    });

    test("initially has zero subscriptions", () => {
      const { subscriber } = createTestSubscriber();
      expect(subscriber.subscriptionCount).toBe(0);
    });
  });

  // -----------------------------------------------------------------------
  // Connection lifecycle
  // -----------------------------------------------------------------------

  describe("connection", () => {
    test("connect resolves when WebSocket opens", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const connectPromise = subscriber.connect();
      // Simulate the WS opening
      mockWs.simulateOpen();
      await connectPromise;
      expect(subscriber.connected).toBe(true);
    });

    test("disconnect closes the WebSocket", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.disconnect();
      expect(subscriber.connected).toBe(false);
      expect(mockWs.closed).toBe(true);
    });

    test("disconnect clears all subscriptions", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.subscribeNewBlocks(() => {});
      subscriber.subscribeNewTxs(() => {});
      expect(subscriber.subscriptionCount).toBe(2);

      subscriber.disconnect();
      expect(subscriber.subscriptionCount).toBe(0);
    });

    test("onConnectionStateChange fires on open", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const states: boolean[] = [];
      subscriber.onConnectionStateChange((connected) => states.push(connected));

      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      expect(states).toContain(true);
    });
  });

  // -----------------------------------------------------------------------
  // Subscribe message format
  // -----------------------------------------------------------------------

  describe("subscribe message format", () => {
    test("subscribeNewBlocks sends correct JSONRPC subscribe", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.subscribeNewBlocks(() => {});

      expect(mockWs.sent.length).toBeGreaterThanOrEqual(1);
      const lastMsg = JSON.parse(mockWs.sent[mockWs.sent.length - 1]);
      expect(lastMsg.jsonrpc).toBe("2.0");
      expect(lastMsg.method).toBe("subscribe");
      expect(lastMsg.params.query).toBe("tm.event='NewBlock'");
    });

    test("subscribeNewTxs sends correct query", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.subscribeNewTxs(() => {});

      const lastMsg = JSON.parse(mockWs.sent[mockWs.sent.length - 1]);
      expect(lastMsg.method).toBe("subscribe");
      expect(lastMsg.params.query).toBe("tm.event='Tx'");
    });

    test("subscribeTx combines event query with custom filter", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.subscribeTx("message.module='poc'", () => {});

      const lastMsg = JSON.parse(mockWs.sent[mockWs.sent.length - 1]);
      expect(lastMsg.params.query).toBe("tm.event='Tx' AND message.module='poc'");
    });

    test("subscribeEvents sends arbitrary query", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.subscribeEvents("custom.event='test'", () => {});

      const lastMsg = JSON.parse(mockWs.sent[mockWs.sent.length - 1]);
      expect(lastMsg.params.query).toBe("custom.event='test'");
    });
  });

  // -----------------------------------------------------------------------
  // Unsubscribe message format
  // -----------------------------------------------------------------------

  describe("unsubscribe message format", () => {
    test("unsubscribe sends JSONRPC unsubscribe message", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const subId = subscriber.subscribeNewBlocks(() => {});
      const sentBeforeUnsub = mockWs.sent.length;

      subscriber.unsubscribe(subId);

      // Should have sent one more message (the unsubscribe)
      expect(mockWs.sent.length).toBe(sentBeforeUnsub + 1);
      const unsubMsg = JSON.parse(mockWs.sent[mockWs.sent.length - 1]);
      expect(unsubMsg.method).toBe("unsubscribe");
      expect(unsubMsg.params.query).toBe("tm.event='NewBlock'");
    });

    test("unsubscribe removes subscription from count", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const subId = subscriber.subscribeNewBlocks(() => {});
      expect(subscriber.subscriptionCount).toBe(1);

      subscriber.unsubscribe(subId);
      expect(subscriber.subscriptionCount).toBe(0);
    });

    test("unsubscribe with unknown ID is a no-op", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const sentBefore = mockWs.sent.length;
      subscriber.unsubscribe("nonexistent_id");
      expect(mockWs.sent.length).toBe(sentBefore);
    });
  });

  // -----------------------------------------------------------------------
  // Event dispatching
  // -----------------------------------------------------------------------

  describe("event dispatching", () => {
    test("dispatches NewBlock event to callback", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const events: NewBlockEvent[] = [];
      subscriber.subscribeNewBlocks((ev) => events.push(ev));

      // Simulate a new block notification from Tendermint
      mockWs.simulateMessage({
        jsonrpc: "2.0",
        result: {
          query: "tm.event='NewBlock'",
          data: {
            value: {
              block: {
                header: {
                  height: "100",
                  time: "2026-03-27T00:00:00Z",
                  num_txs: "5",
                  proposer_address: "ABCD1234",
                },
              },
              block_id: { hash: "BLOCKHASH123" },
            },
          },
          events: {},
        },
      });

      expect(events.length).toBe(1);
      expect(events[0].height).toBe(100);
      expect(events[0].hash).toBe("BLOCKHASH123");
      expect(events[0].numTxs).toBe(5);
      expect(events[0].time).toBe("2026-03-27T00:00:00Z");
    });

    test("dispatches Tx event to callback", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const events: NewTxEvent[] = [];
      subscriber.subscribeNewTxs((ev) => events.push(ev));

      mockWs.simulateMessage({
        jsonrpc: "2.0",
        result: {
          query: "tm.event='Tx'",
          data: {
            value: {
              TxResult: {
                hash: "TX_HASH_ABC",
                height: "200",
                result: {
                  code: 0,
                  log: "success",
                  events: [{ type: "transfer", attributes: [{ key: "amount", value: "1000" }] }],
                },
              },
            },
          },
          events: {},
        },
      });

      expect(events.length).toBe(1);
      expect(events[0].hash).toBe("TX_HASH_ABC");
      expect(events[0].height).toBe(200);
      expect(events[0].code).toBe(0);
    });

    test("dispatches generic event to callback", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const events: EventData[] = [];
      subscriber.subscribeEvents("custom.event='test'", (ev) => events.push(ev));

      mockWs.simulateMessage({
        jsonrpc: "2.0",
        result: {
          query: "custom.event='test'",
          data: { something: "happened" },
          events: { "custom.event": ["test"] },
        },
      });

      expect(events.length).toBe(1);
      expect(events[0].query).toBe("custom.event='test'");
      expect(events[0].data).toEqual({ something: "happened" });
    });

    test("does not dispatch event to wrong subscription", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const blockEvents: NewBlockEvent[] = [];
      subscriber.subscribeNewBlocks((ev) => blockEvents.push(ev));

      // Send a Tx event -- should NOT trigger the block callback
      mockWs.simulateMessage({
        jsonrpc: "2.0",
        result: {
          query: "tm.event='Tx'",
          data: { value: { TxResult: { hash: "TX", height: "1", result: {} } } },
          events: {},
        },
      });

      expect(blockEvents.length).toBe(0);
    });
  });

  // -----------------------------------------------------------------------
  // Error handling
  // -----------------------------------------------------------------------

  describe("error handling", () => {
    test("reports Tendermint RPC errors", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const errors: Error[] = [];
      subscriber.onSubscriptionError((err) => errors.push(err));

      mockWs.simulateMessage({
        jsonrpc: "2.0",
        error: { code: -32600, message: "Invalid Request" },
      });

      expect(errors.length).toBe(1);
      expect(errors[0].message).toContain("Invalid Request");
    });

    test("reports malformed JSON gracefully", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const errors: Error[] = [];
      subscriber.onSubscriptionError((err) => errors.push(err));

      mockWs.simulateMessage("not valid json {{{{");

      expect(errors.length).toBe(1);
      expect(errors[0].message).toContain("Failed to parse");
    });
  });

  // -----------------------------------------------------------------------
  // Reconnect logic
  // -----------------------------------------------------------------------

  describe("reconnect logic", () => {
    test("schedules reconnect on unexpected close when enabled", async () => {
      const { subscriber, mockWs } = createTestSubscriber({ reconnect: true, maxAttempts: 1 });
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const states: boolean[] = [];
      subscriber.onConnectionStateChange((c) => states.push(c));

      // Simulate unexpected close (not user-initiated)
      mockWs.simulateClose();

      // The close should have been detected
      expect(states).toContain(false);

      // Clean up
      subscriber.disconnect();
    });

    test("does not reconnect after explicit disconnect", async () => {
      const { subscriber, mockWs } = createTestSubscriber({ reconnect: true });
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      subscriber.disconnect();
      // After explicit disconnect, mockWs.closed should be true
      expect(mockWs.closed).toBe(true);
    });

    test("re-subscribes existing subscriptions after reconnect", async () => {
      let wsInstance: MockWebSocket;
      let callCount = 0;

      const factory: WebSocketFactory = (_url: string) => {
        callCount++;
        wsInstance = new MockWebSocket();
        return wsInstance;
      };

      const subscriber = new OmniphiSubscriber(
        "ws://localhost:26657/websocket",
        { enabled: true, initialDelayMs: 10, maxDelayMs: 20, multiplier: 1, maxAttempts: 2 },
        factory,
      );

      // First connection
      const p = subscriber.connect();
      wsInstance!.simulateOpen();
      await p;

      // Add a subscription
      subscriber.subscribeNewBlocks(() => {});
      const sentAfterSub = wsInstance!.sent.length;
      expect(sentAfterSub).toBeGreaterThanOrEqual(1);

      // Simulate connection drop and reconnect
      wsInstance!.simulateClose();

      // Wait for reconnect timer to fire
      await new Promise((resolve) => setTimeout(resolve, 50));

      // A new WebSocket should have been created
      expect(callCount).toBe(2);

      // Simulate the new connection opening
      wsInstance!.simulateOpen();

      // Wait for resubscribe
      await new Promise((resolve) => setTimeout(resolve, 10));

      // The new WS should have received the re-subscribe message
      expect(wsInstance!.sent.length).toBeGreaterThanOrEqual(1);
      const resubMsg = JSON.parse(wsInstance!.sent[0]);
      expect(resubMsg.method).toBe("subscribe");
      expect(resubMsg.params.query).toBe("tm.event='NewBlock'");

      subscriber.disconnect();
    });
  });

  // -----------------------------------------------------------------------
  // Static helpers
  // -----------------------------------------------------------------------

  describe("static helpers", () => {
    test("buildSubscribeMessage creates valid JSONRPC", () => {
      const msg = OmniphiSubscriber.buildSubscribeMessage("tm.event='NewBlock'", "42");
      const parsed = JSON.parse(msg);

      expect(parsed.jsonrpc).toBe("2.0");
      expect(parsed.id).toBe("42");
      expect(parsed.method).toBe("subscribe");
      expect(parsed.params.query).toBe("tm.event='NewBlock'");
    });

    test("buildUnsubscribeMessage creates valid JSONRPC", () => {
      const msg = OmniphiSubscriber.buildUnsubscribeMessage("tm.event='Tx'", "99");
      const parsed = JSON.parse(msg);

      expect(parsed.jsonrpc).toBe("2.0");
      expect(parsed.id).toBe("99");
      expect(parsed.method).toBe("unsubscribe");
      expect(parsed.params.query).toBe("tm.event='Tx'");
    });

    test("buildSubscribeMessage uses default id", () => {
      const msg = OmniphiSubscriber.buildSubscribeMessage("tm.event='NewBlock'");
      const parsed = JSON.parse(msg);
      expect(parsed.id).toBe("1");
    });

    test("subscribe returns unique subscription IDs", async () => {
      const { subscriber, mockWs } = createTestSubscriber();
      const p = subscriber.connect();
      mockWs.simulateOpen();
      await p;

      const id1 = subscriber.subscribeNewBlocks(() => {});
      const id2 = subscriber.subscribeNewTxs(() => {});
      const id3 = subscriber.subscribeEvents("test", () => {});

      expect(id1).not.toBe(id2);
      expect(id2).not.toBe(id3);
      expect(id1).not.toBe(id3);

      subscriber.disconnect();
    });
  });
});
