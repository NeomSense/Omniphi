// Package deepseek implements the DeepSeek API client (OpenAI-compatible Chat Completions).
package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gov-copilot/internal/chain"
	"gov-copilot/internal/config"
	"gov-copilot/internal/report"
)

// Client calls the DeepSeek chat/completions endpoint.
type Client struct {
	baseURL    string
	model      string
	apiKey     string
	timeout    time.Duration
	maxRetries int
	httpClient *http.Client
}

// NewClient creates a DeepSeek client from configuration.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL:    strings.TrimRight(cfg.DeepSeekBaseURL, "/"),
		model:      cfg.DeepSeekModel,
		apiKey:     cfg.DeepSeekAPIKey,
		timeout:    time.Duration(cfg.AITimeoutSecs) * time.Second,
		maxRetries: cfg.AIMaxRetries,
		httpClient: &http.Client{Timeout: time.Duration(cfg.AITimeoutSecs) * time.Second},
	}
}

// chatRequest matches OpenAI-format Chat Completions.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const systemPrompt = `You are a blockchain governance security analyst for the Omniphi network.
Your task is to analyze a governance proposal and produce a structured JSON advisory report.

Output ONLY valid JSON matching this exact schema (no markdown, no extra keys, no commentary):
{
  "summary": "one-sentence summary of the proposal and its risk",
  "key_changes": ["change 1", "change 2"],
  "what_could_go_wrong": ["risk 1", "risk 2"],
  "recommended_safety_actions": ["action 1", "action 2"]
}

Be concise and actionable. Focus on security implications.`

// GenerateReport calls DeepSeek to produce a structured advisory report.
// On failure (API error or invalid JSON), returns nil — the caller should
// fall back to the template generator.
func (c *Client) GenerateReport(
	ctx context.Context,
	proposal *chain.Proposal,
	riskReport *chain.RiskReport,
	queuedExec *chain.QueuedExecution,
) (*report.Report, error) {
	userContent := buildUserPrompt(proposal, riskReport, queuedExec)

	req := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Stream:      false,
		Temperature: 0.3,
		MaxTokens:   2000,
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("[deepseek] retry %d/%d after %v", attempt, c.maxRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		body, err := c.callAPI(ctx, req)
		if err != nil {
			lastErr = err
			log.Printf("[deepseek] API call failed: %v", err)
			continue
		}

		parsed, err := parseAIResponse(body)
		if err != nil {
			lastErr = fmt.Errorf("parse AI response: %w", err)
			log.Printf("[deepseek] invalid JSON response: %v", err)
			continue
		}

		return parsed, nil
	}

	return nil, fmt.Errorf("deepseek failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

func (c *Client) callAPI(ctx context.Context, req chatRequest) (string, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// aiFields are the fields we extract from the DeepSeek JSON response.
type aiFields struct {
	Summary                  string   `json:"summary"`
	KeyChanges               []string `json:"key_changes"`
	WhatCouldGoWrong         []string `json:"what_could_go_wrong"`
	RecommendedSafetyActions []string `json:"recommended_safety_actions"`
}

func parseAIResponse(content string) (*report.Report, error) {
	// Strip markdown code fences if present
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.SplitN(content, "\n", 2)
		if len(lines) > 1 {
			content = lines[1]
		}
		if idx := strings.LastIndex(content, "```"); idx >= 0 {
			content = content[:idx]
		}
		content = strings.TrimSpace(content)
	}

	var fields aiFields
	if err := json.Unmarshal([]byte(content), &fields); err != nil {
		return nil, fmt.Errorf("invalid JSON from AI: %w\nraw: %s", err, truncate(content, 500))
	}

	if fields.Summary == "" {
		return nil, fmt.Errorf("AI returned empty summary")
	}

	// Return a partial report — caller populates metadata fields
	r := &report.Report{
		Summary:                  fields.Summary,
		KeyChanges:               fields.KeyChanges,
		WhatCouldGoWrong:         fields.WhatCouldGoWrong,
		RecommendedSafetyActions: fields.RecommendedSafetyActions,
	}
	return r, nil
}

func buildUserPrompt(
	proposal *chain.Proposal,
	rr *chain.RiskReport,
	qe *chain.QueuedExecution,
) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Analyze this governance proposal:\n\n"))
	sb.WriteString(fmt.Sprintf("Proposal ID: %d\n", proposal.ID))
	sb.WriteString(fmt.Sprintf("Title: %s\n", proposal.Title))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", proposal.Summary))
	sb.WriteString(fmt.Sprintf("Status: %s\n", proposal.Status))
	if len(proposal.MessageTypes) > 0 {
		sb.WriteString(fmt.Sprintf("Messages: %s\n", strings.Join(proposal.MessageTypes, ", ")))
	}

	if rr != nil {
		sb.WriteString(fmt.Sprintf("\nGuard Risk Report:\n"))
		sb.WriteString(fmt.Sprintf("  Rules tier: %s (score %d)\n", rr.Tier, rr.Score))
		sb.WriteString(fmt.Sprintf("  Delay: %d blocks, Threshold: %d bps\n", rr.ComputedDelayBlocks, rr.ComputedThresholdBps))
		if rr.AIScore > 0 {
			sb.WriteString(fmt.Sprintf("  AI tier: %s (score %d)\n", rr.AITier, rr.AIScore))
		}
		if rr.ReasonCodes != "" {
			sb.WriteString(fmt.Sprintf("  Reason codes: %s\n", rr.ReasonCodes))
		}
	}

	if qe != nil {
		sb.WriteString(fmt.Sprintf("\nExecution State:\n"))
		sb.WriteString(fmt.Sprintf("  Gate: %s\n", qe.GateState))
		sb.WriteString(fmt.Sprintf("  Earliest exec height: %d\n", qe.EarliestExecHeight))
		if qe.StatusNote != "" {
			sb.WriteString(fmt.Sprintf("  Note: %s\n", qe.StatusNote))
		}
	}

	sb.WriteString("\nRespond with ONLY the JSON object (no markdown, no extra text).\n")
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
