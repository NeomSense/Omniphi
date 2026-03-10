// Package server implements the HTTP API for the actions proxy.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"actions-proxy/internal/chain"
	"actions-proxy/internal/config"
	"actions-proxy/internal/ratelimit"
	"actions-proxy/internal/types"
)

// Server handles HTTP requests for the actions proxy.
type Server struct {
	cfg     *config.Config
	chain   *chain.Client
	limiter *ratelimit.Limiter
	mux     *http.ServeMux
}

// New creates a new HTTP server with all routes registered.
func New(cfg *config.Config, chainClient *chain.Client, limiter *ratelimit.Limiter) *Server {
	s := &Server{
		cfg:     cfg,
		chain:   chainClient,
		limiter: limiter,
		mux:     http.NewServeMux(),
	}

	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /confirm-execution", s.withCORS(s.withAuth(s.withRateLimit(s.handleConfirmExecution))))
	s.mux.HandleFunc("OPTIONS /confirm-execution", s.handlePreflight)

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:         s.cfg.BindAddr,
		Handler:      s,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

// ---------- Handlers ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, types.HealthResponse{
		OK:      true,
		Service: "actions-proxy",
		Version: "v1",
	})
}

func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	s.setCORSHeaders(w)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConfirmExecution(w http.ResponseWriter, r *http.Request) {
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Parse request
	var req types.ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, types.ConfirmResponse{
			Action:  "confirm-execution",
			Result:  "rejected",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate proposal_id
	proposalID, err := strconv.ParseUint(req.ProposalID, 10, 64)
	if err != nil || proposalID == 0 {
		writeJSON(w, http.StatusBadRequest, types.ConfirmResponse{
			Action:  "confirm-execution",
			Result:  "rejected",
			Message: "proposal_id must be a positive integer",
		})
		return
	}

	log.Printf("[%s] confirm-execution proposal_id=%d", reqID, proposalID)

	// Step 1: Query guard status (preflight check)
	status, err := s.chain.GetGuardStatus(r.Context(), proposalID)
	if err != nil {
		log.Printf("[%s] guard query failed: %v", reqID, err)
		writeJSON(w, http.StatusInternalServerError, types.ConfirmResponse{
			ProposalID: proposalID,
			Eligible:   false,
			Action:     "confirm-execution",
			Result:     "rejected",
			Message:    "Failed to query guard status: " + err.Error(),
		})
		return
	}

	// Step 2: Check idempotency — already confirmed
	if status.SecondConfirmReceived {
		log.Printf("[%s] proposal %d already confirmed", reqID, proposalID)
		writeJSON(w, http.StatusOK, types.ConfirmResponse{
			ProposalID: proposalID,
			Eligible:   false,
			Action:     "confirm-execution",
			Result:     "already_confirmed",
			Message:    "Second confirmation has already been received for this proposal.",
		})
		return
	}

	// Step 3: Validate eligibility
	if reason := s.checkEligibility(status); reason != "" {
		log.Printf("[%s] proposal %d not eligible: %s", reqID, proposalID, reason)
		writeJSON(w, http.StatusConflict, types.ConfirmResponse{
			ProposalID: proposalID,
			Eligible:   false,
			Action:     "confirm-execution",
			Result:     "rejected",
			Message:    reason,
		})
		return
	}

	// Step 4: Query risk tier to verify CRITICAL
	tier, err := s.chain.GetRiskTier(r.Context(), proposalID)
	if err != nil {
		log.Printf("[%s] risk report query failed: %v", reqID, err)
		// Continue — the on-chain tx will enforce this anyway
	} else if !isCriticalTier(tier) {
		log.Printf("[%s] proposal %d tier=%s, not CRITICAL", reqID, proposalID, tier)
		writeJSON(w, http.StatusConflict, types.ConfirmResponse{
			ProposalID: proposalID,
			Eligible:   false,
			Action:     "confirm-execution",
			Result:     "rejected",
			Message:    fmt.Sprintf("Proposal tier is %s, not CRITICAL. Second confirmation only applies to CRITICAL proposals.", tier),
		})
		return
	}

	log.Printf("[%s] proposal %d eligible, broadcasting confirm-execution tx", reqID, proposalID)

	// Step 5: Broadcast tx
	justification := "Confirmed via actions-proxy"
	txResult, err := s.chain.ConfirmExecution(r.Context(), proposalID, justification)
	if err != nil {
		log.Printf("[%s] tx broadcast failed: %v", reqID, err)
		writeJSON(w, http.StatusInternalServerError, types.ConfirmResponse{
			ProposalID: proposalID,
			Eligible:   true,
			Action:     "confirm-execution",
			Result:     "rejected",
			Message:    "Transaction broadcast failed: " + err.Error(),
		})
		return
	}

	// Step 6: Check tx result
	if txResult.Code != 0 {
		log.Printf("[%s] tx failed code=%d raw_log=%s", reqID, txResult.Code, txResult.RawLog)
		writeJSON(w, http.StatusInternalServerError, types.ConfirmResponse{
			ProposalID: proposalID,
			Eligible:   true,
			Action:     "confirm-execution",
			Result:     "rejected",
			Tx:         txResult,
			Message:    fmt.Sprintf("Transaction failed (code %d): %s", txResult.Code, txResult.RawLog),
		})
		return
	}

	log.Printf("[%s] proposal %d confirmed, txhash=%s", reqID, proposalID, txResult.TxHash)
	writeJSON(w, http.StatusOK, types.ConfirmResponse{
		ProposalID: proposalID,
		Eligible:   true,
		Action:     "confirm-execution",
		Result:     "submitted",
		Tx:         txResult,
		Message:    fmt.Sprintf("Execution confirmed. TxHash: %s", txResult.TxHash),
	})
}

// ---------- Eligibility checks ----------

func (s *Server) checkEligibility(status *types.GuardStatus) string {
	if !isReadyState(status.GateState) {
		return fmt.Sprintf("Proposal gate_state is %q, expected READY.", status.GateState)
	}
	if !status.RequiresSecondConfirm {
		return "Proposal does not require second confirmation."
	}
	return ""
}

func isReadyState(state string) bool {
	return strings.Contains(strings.ToUpper(state), "READY")
}

func isCriticalTier(tier string) bool {
	return strings.Contains(strings.ToUpper(tier), "CRITICAL")
}

// ---------- Middleware ----------

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key != s.cfg.APIKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid or missing API key",
			})
			return
		}
		next(w, r)
	}
}

func (s *Server) withRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Rate limit exceeded. Try again shortly.",
			})
			return
		}
		next(w, r)
	}
}

func (s *Server) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.setCORSHeaders(w)
		next(w, r)
	}
}

func (s *Server) setCORSHeaders(w http.ResponseWriter) {
	if len(s.cfg.CORSAllowOrigins) == 1 && s.cfg.CORSAllowOrigins[0] == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", strings.Join(s.cfg.CORSAllowOrigins, ", "))
	}
}

// ---------- Helpers ----------

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
