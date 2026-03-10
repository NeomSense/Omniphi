// Package watcher subscribes to CometBFT WebSocket events and emits
// parsed poc_similarity_commitment events to a channel.
package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SimilarityEvent is a parsed poc_similarity_commitment event from the chain.
type SimilarityEvent struct {
	ContributionID    uint64
	Oracle            string
	OverallSimilarity uint32 // 0-10000 scaled
	Confidence        uint32 // 0-10000 scaled
	NearestParent     uint64
	IsDerivative      bool
	Epoch             uint64
	BlockHeight       int64
}

// Watcher connects to a CometBFT node via WebSocket and listens for
// poc_similarity_commitment events.
type Watcher struct {
	wsURL string

	mu   sync.Mutex
	conn *websocket.Conn
}

// New creates a new event watcher.
func New(wsURL string) *Watcher {
	return &Watcher{wsURL: wsURL}
}

// cometBFT JSON-RPC types for subscribe
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      int         `json:"id"`
	Params  interface{} `json:"params"`
}

// Watch connects to the WebSocket, subscribes to poc_similarity_commitment events,
// and sends parsed events to the returned channel. Reconnects on failure.
// Blocks until ctx is cancelled.
func (w *Watcher) Watch(ctx context.Context) (<-chan SimilarityEvent, error) {
	events := make(chan SimilarityEvent, 64)

	go w.watchLoop(ctx, events)

	return events, nil
}

func (w *Watcher) watchLoop(ctx context.Context, events chan<- SimilarityEvent) {
	defer close(events)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.connectAndListen(ctx, events); err != nil {
			log.Printf("[watcher] connection error: %v (reconnecting in %v)", err, backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			backoff = min(backoff*2, maxBackoff)
			continue
		}

		// Successful session ended (context cancelled)
		return
	}
}

func (w *Watcher) connectAndListen(ctx context.Context, events chan<- SimilarityEvent) error {
	log.Printf("[watcher] connecting to %s", w.wsURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, w.wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	w.mu.Lock()
	w.conn = conn
	w.mu.Unlock()

	defer func() {
		conn.Close()
		w.mu.Lock()
		w.conn = nil
		w.mu.Unlock()
	}()

	// Subscribe to poc_similarity_commitment events
	subscribeReq := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "subscribe",
		ID:      1,
		Params: map[string]string{
			"query": "tm.event='Tx' AND poc_similarity_commitment.contribution_id EXISTS",
		},
	}

	if err := conn.WriteJSON(subscribeReq); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	log.Printf("[watcher] subscribed to poc_similarity_commitment events")

	// Read messages
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		_, message, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil // context cancelled, clean exit
			}
			return fmt.Errorf("read: %w", err)
		}

		evt, err := parseSimilarityEvent(message)
		if err != nil {
			// Not a similarity event or parse error — skip
			continue
		}

		select {
		case events <- *evt:
		default:
			log.Printf("[watcher] event channel full, dropping contribution %d", evt.ContributionID)
		}
	}
}

// Close shuts down the WebSocket connection.
func (w *Watcher) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		w.conn.Close()
	}
}

// parseSimilarityEvent extracts poc_similarity_commitment attributes from a
// CometBFT WebSocket tx event message.
func parseSimilarityEvent(raw []byte) (*SimilarityEvent, error) {
	// CometBFT tx result envelope
	var envelope struct {
		Result struct {
			Events map[string][]string `json:"events"`
			Data   struct {
				Value struct {
					TxResult struct {
						Height string `json:"height"`
					} `json:"TxResult"`
				} `json:"value"`
			} `json:"data"`
		} `json:"result"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	evts := envelope.Result.Events
	if evts == nil {
		return nil, fmt.Errorf("no events in message")
	}

	// Look for poc_similarity_commitment.contribution_id
	contribIDs, ok := evts["poc_similarity_commitment.contribution_id"]
	if !ok || len(contribIDs) == 0 {
		return nil, fmt.Errorf("not a similarity event")
	}

	evt := &SimilarityEvent{}

	// Parse contribution_id
	cid, err := strconv.ParseUint(contribIDs[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse contribution_id: %w", err)
	}
	evt.ContributionID = cid

	// Parse oracle
	if vals := evts["poc_similarity_commitment.oracle"]; len(vals) > 0 {
		evt.Oracle = vals[0]
	}

	// Parse overall_similarity
	if vals := evts["poc_similarity_commitment.overall_similarity"]; len(vals) > 0 {
		if v, err := strconv.ParseUint(vals[0], 10, 32); err == nil {
			evt.OverallSimilarity = uint32(v)
		}
	}

	// Parse confidence
	if vals := evts["poc_similarity_commitment.confidence"]; len(vals) > 0 {
		if v, err := strconv.ParseUint(vals[0], 10, 32); err == nil {
			evt.Confidence = uint32(v)
		}
	}

	// Parse nearest_parent
	if vals := evts["poc_similarity_commitment.nearest_parent"]; len(vals) > 0 {
		if v, err := strconv.ParseUint(vals[0], 10, 64); err == nil {
			evt.NearestParent = v
		}
	}

	// Parse is_derivative
	if vals := evts["poc_similarity_commitment.is_derivative"]; len(vals) > 0 {
		evt.IsDerivative = vals[0] == "true"
	}

	// Parse epoch
	if vals := evts["poc_similarity_commitment.epoch"]; len(vals) > 0 {
		if v, err := strconv.ParseUint(vals[0], 10, 64); err == nil {
			evt.Epoch = v
		}
	}

	// Parse block height from tx result
	if h := envelope.Result.Data.Value.TxResult.Height; h != "" {
		if v, err := strconv.ParseInt(h, 10, 64); err == nil {
			evt.BlockHeight = v
		}
	}

	return evt, nil
}
