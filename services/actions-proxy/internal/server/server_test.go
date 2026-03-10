package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"actions-proxy/internal/config"
	"actions-proxy/internal/chain"
	"actions-proxy/internal/ratelimit"
	"actions-proxy/internal/server"
)

func newTestServer() *server.Server {
	cfg := &config.Config{
		APIKey:           "test-key",
		CORSAllowOrigins: []string{"*"},
	}
	chainClient := chain.NewClient(cfg)
	limiter := ratelimit.New(100, 10) // generous limits for tests
	return server.New(cfg, chainClient, limiter)
}

func TestHealth(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health status: got %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) {
		t.Errorf("health body should contain ok:true, got %s", body)
	}
	if !strings.Contains(body, `"service":"actions-proxy"`) {
		t.Errorf("health body should contain service name, got %s", body)
	}
}

func TestConfirmExecution_NoAuth(t *testing.T) {
	srv := newTestServer()
	body := `{"proposal_id":"42"}`
	req := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestConfirmExecution_WrongAuth(t *testing.T) {
	srv := newTestServer()
	body := `{"proposal_id":"42"}`
	req := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestConfirmExecution_BadBody(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestConfirmExecution_InvalidProposalID(t *testing.T) {
	srv := newTestServer()
	body := `{"proposal_id":"abc"}`
	req := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestConfirmExecution_ZeroProposalID(t *testing.T) {
	srv := newTestServer()
	body := `{"proposal_id":"0"}`
	req := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPreflight_CORS(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("OPTIONS", "/confirm-execution", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status: got %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin: got %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-API-Key") {
		t.Errorf("CORS headers should include X-API-Key, got %q", got)
	}
}

func TestRateLimit(t *testing.T) {
	cfg := &config.Config{
		APIKey:           "test-key",
		CORSAllowOrigins: []string{"*"},
	}
	chainClient := chain.NewClient(cfg)
	limiter := ratelimit.New(100, 1) // burst of 1

	srv := server.New(cfg, chainClient, limiter)

	body := `{"proposal_id":"42"}`

	// First request uses the single token
	req1 := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-API-Key", "test-key")
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	// This will fail at chain query (500), but it passed auth + rate limit

	// Second request should be rate limited
	req2 := httptest.NewRequest("POST", "/confirm-execution", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-API-Key", "test-key")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second request, got %d", w2.Code)
	}
}
