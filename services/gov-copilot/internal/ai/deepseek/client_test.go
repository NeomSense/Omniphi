package deepseek_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gov-copilot/internal/ai/deepseek"
	"gov-copilot/internal/chain"
	"gov-copilot/internal/config"
)

func TestDeepSeek_SuccessfulResponse(t *testing.T) {
	// Mock DeepSeek API server
	aiResponse := map[string]interface{}{
		"summary":                    "Test proposal: LOW risk parameter change",
		"key_changes":               []string{"Change fee param from 0.1 to 0.2"},
		"what_could_go_wrong":        []string{"Higher fees may reduce tx volume"},
		"recommended_safety_actions": []string{"Monitor tx volume after change"},
	}
	aiJSON, _ := json.Marshal(aiResponse)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing/wrong Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing Content-Type header")
		}

		// Verify request body
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if req.Model != "deepseek-chat" {
			t.Errorf("model: got %q, want %q", req.Model, "deepseek-chat")
		}
		if req.Stream != false {
			t.Errorf("stream should be false")
		}
		if len(req.Messages) < 2 {
			t.Errorf("expected at least 2 messages (system + user), got %d", len(req.Messages))
		}

		// Return mock response
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": string(aiJSON),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekBaseURL: srv.URL,
		DeepSeekModel:   "deepseek-chat",
		DeepSeekAPIKey:  "test-key",
		AITimeoutSecs:   10,
		AIMaxRetries:    0,
	}

	client := deepseek.NewClient(cfg)

	proposal := &chain.Proposal{
		ID:    1,
		Title: "Test Proposal",
	}

	report, err := client.GenerateReport(context.Background(), proposal, nil, nil)
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}

	if report.Summary != "Test proposal: LOW risk parameter change" {
		t.Errorf("summary: got %q", report.Summary)
	}
	if len(report.KeyChanges) != 1 {
		t.Errorf("key_changes: got %d items", len(report.KeyChanges))
	}
	if len(report.WhatCouldGoWrong) != 1 {
		t.Errorf("what_could_go_wrong: got %d items", len(report.WhatCouldGoWrong))
	}
}

func TestDeepSeek_InvalidJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "This is not valid JSON at all",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekBaseURL: srv.URL,
		DeepSeekModel:   "deepseek-chat",
		DeepSeekAPIKey:  "test-key",
		AITimeoutSecs:   5,
		AIMaxRetries:    0,
	}

	client := deepseek.NewClient(cfg)
	proposal := &chain.Proposal{ID: 1, Title: "Test"}

	_, err := client.GenerateReport(context.Background(), proposal, nil, nil)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestDeepSeek_APIError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekBaseURL: srv.URL,
		DeepSeekModel:   "deepseek-chat",
		DeepSeekAPIKey:  "test-key",
		AITimeoutSecs:   5,
		AIMaxRetries:    0,
	}

	client := deepseek.NewClient(cfg)
	proposal := &chain.Proposal{ID: 1, Title: "Test"}

	_, err := client.GenerateReport(context.Background(), proposal, nil, nil)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestDeepSeek_MarkdownWrappedJSON(t *testing.T) {
	// Some LLMs wrap JSON in markdown code fences
	wrapped := "```json\n{\"summary\":\"wrapped test\",\"key_changes\":[],\"what_could_go_wrong\":[],\"recommended_safety_actions\":[]}\n```"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": wrapped,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekBaseURL: srv.URL,
		DeepSeekModel:   "deepseek-chat",
		DeepSeekAPIKey:  "test-key",
		AITimeoutSecs:   5,
		AIMaxRetries:    0,
	}

	client := deepseek.NewClient(cfg)
	proposal := &chain.Proposal{ID: 1, Title: "Test"}

	report, err := client.GenerateReport(context.Background(), proposal, nil, nil)
	if err != nil {
		t.Fatalf("expected success for markdown-wrapped JSON: %v", err)
	}
	if report.Summary != "wrapped test" {
		t.Errorf("summary: got %q, want %q", report.Summary, "wrapped test")
	}
}

func TestDeepSeek_RetryOnFailure(t *testing.T) {
	callCount := 0
	aiResponse := `{"summary":"retry success","key_changes":[],"what_could_go_wrong":[],"recommended_safety_actions":[]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": aiResponse}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekBaseURL: srv.URL,
		DeepSeekModel:   "deepseek-chat",
		DeepSeekAPIKey:  "test-key",
		AITimeoutSecs:   5,
		AIMaxRetries:    2,
	}

	client := deepseek.NewClient(cfg)
	proposal := &chain.Proposal{ID: 1, Title: "Test"}

	report, err := client.GenerateReport(context.Background(), proposal, nil, nil)
	if err != nil {
		t.Fatalf("expected success after retry: %v", err)
	}
	if report.Summary != "retry success" {
		t.Errorf("summary: got %q", report.Summary)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 fail + 1 success), got %d", callCount)
	}
}
