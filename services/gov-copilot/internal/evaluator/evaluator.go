// Package evaluator defines the AI evaluation interface and OpenAI implementation
// for scoring PoC contributions.
package evaluator

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

	"gov-copilot/internal/fetcher"
)

// Score represents the AI's evaluation of a contribution.
type Score struct {
	Quality     float64 `json:"quality"`     // 1-10
	Originality float64 `json:"originality"` // 1-10
	Correctness float64 `json:"correctness"` // 1-10
	Weighted    float64 `json:"weighted"`    // computed weighted average
	Reasoning   string  `json:"reasoning"`   // brief explanation
}

// Decision is the endorsement decision.
type Decision int

const (
	DecisionApprove      Decision = iota // Score >= ApproveThreshold
	DecisionReject                       // Score <= RejectThreshold
	DecisionManualReview                 // In between
	DecisionAutoReject                   // Derivative auto-reject
)

func (d Decision) String() string {
	switch d {
	case DecisionApprove:
		return "APPROVE"
	case DecisionReject:
		return "REJECT"
	case DecisionManualReview:
		return "MANUAL_REVIEW"
	case DecisionAutoReject:
		return "AUTO_REJECT"
	default:
		return "UNKNOWN"
	}
}

// EvalResult is the complete evaluation output.
type EvalResult struct {
	ContributionID uint64
	Score          Score
	Decision       Decision
	IsDerivative   bool
}

// Evaluator scores contribution content.
type Evaluator interface {
	Evaluate(ctx context.Context, content *fetcher.ContentResult, isDerivative bool) (*EvalResult, error)
}

// EvalConfig holds the configuration needed by the evaluator.
// This is a subset of the main config, avoiding import cycles.
type EvalConfig struct {
	OpenAIBaseURL    string
	OpenAIModel      string
	OpenAIAPIKey     string
	AITimeout        time.Duration
	AIMaxRetries     int
	ApproveThreshold float64
	RejectThreshold  float64
}

// OpenAIEvaluator uses the OpenAI Chat Completions API (or compatible) to score content.
type OpenAIEvaluator struct {
	baseURL    string
	model      string
	apiKey     string
	timeout    time.Duration
	maxRetries int
	httpClient *http.Client

	approveThreshold float64
	rejectThreshold  float64
}

// NewOpenAIEvaluator creates an evaluator backed by OpenAI (or any compatible API).
func NewOpenAIEvaluator(cfg EvalConfig) *OpenAIEvaluator {
	return &OpenAIEvaluator{
		baseURL:          strings.TrimRight(cfg.OpenAIBaseURL, "/"),
		model:            cfg.OpenAIModel,
		apiKey:           cfg.OpenAIAPIKey,
		timeout:          cfg.AITimeout,
		maxRetries:       cfg.AIMaxRetries,
		httpClient:       &http.Client{Timeout: cfg.AITimeout},
		approveThreshold: cfg.ApproveThreshold,
		rejectThreshold:  cfg.RejectThreshold,
	}
}

const systemPrompt = `You are a blockchain contribution auditor for the Omniphi Proof-of-Contribution network.
Your job is to evaluate submitted content (code, text, or datasets) for quality, originality, and correctness.

Rate the submission on three axes, each from 1 to 10:
- Quality: How well-written, structured, and complete is the content?
- Originality: How novel is this contribution? Does it add genuine value?
- Correctness: Is the content technically accurate and free of errors?

Return ONLY valid JSON matching this exact schema:
{
  "quality": <number 1-10>,
  "originality": <number 1-10>,
  "correctness": <number 1-10>,
  "reasoning": "<one sentence explaining your rating>"
}

No markdown, no extra keys, no commentary outside the JSON.`

// Evaluate scores a contribution's content using the LLM.
// If the contribution is flagged as derivative (from Layer 2), it auto-rejects with Score 0.
func (e *OpenAIEvaluator) Evaluate(ctx context.Context, content *fetcher.ContentResult, isDerivative bool) (*EvalResult, error) {
	result := &EvalResult{
		ContributionID: content.Contribution.ID,
		IsDerivative:   isDerivative,
	}

	// Fast path: derivatives are auto-rejected
	if isDerivative {
		result.Score = Score{
			Quality:     0,
			Originality: 0,
			Correctness: 0,
			Weighted:    0,
			Reasoning:   "Auto-rejected: flagged as derivative by similarity oracle (Layer 2)",
		}
		result.Decision = DecisionAutoReject
		return result, nil
	}

	// If content couldn't be fetched, we can't evaluate
	if content.FetchError != nil {
		return nil, fmt.Errorf("cannot evaluate contribution %d: content fetch failed: %w",
			content.Contribution.ID, content.FetchError)
	}

	if content.Content == "" {
		return nil, fmt.Errorf("cannot evaluate contribution %d: empty content", content.Contribution.ID)
	}

	// Call LLM
	score, err := e.callLLM(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("LLM evaluation failed for contribution %d: %w",
			content.Contribution.ID, err)
	}

	result.Score = *score

	// Determine decision based on thresholds
	switch {
	case score.Weighted >= e.approveThreshold:
		result.Decision = DecisionApprove
	case score.Weighted <= e.rejectThreshold:
		result.Decision = DecisionReject
	default:
		result.Decision = DecisionManualReview
	}

	return result, nil
}

func (e *OpenAIEvaluator) callLLM(ctx context.Context, content *fetcher.ContentResult) (*Score, error) {
	userPrompt := buildUserPrompt(content)

	req := chatRequest{
		Model: e.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2, // Low temperature for consistent scoring
		MaxTokens:   500,
	}

	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("[evaluator] retry %d/%d after %v", attempt, e.maxRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		responseText, err := e.callAPI(ctx, req)
		if err != nil {
			lastErr = err
			log.Printf("[evaluator] API call failed: %v", err)
			continue
		}

		score, err := parseScoreResponse(responseText)
		if err != nil {
			lastErr = fmt.Errorf("parse score: %w", err)
			log.Printf("[evaluator] invalid response: %v", err)
			continue
		}

		return score, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", e.maxRetries+1, lastErr)
}

// chatRequest/chatResponse match OpenAI Chat Completions format.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
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

func (e *OpenAIEvaluator) callAPI(ctx context.Context, req chatRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := e.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func parseScoreResponse(content string) (*Score, error) {
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

	var score Score
	if err := json.Unmarshal([]byte(content), &score); err != nil {
		return nil, fmt.Errorf("invalid JSON from AI: %w\nraw: %s", err, truncate(content, 300))
	}

	// Validate ranges
	if score.Quality < 0 || score.Quality > 10 {
		return nil, fmt.Errorf("quality score %.1f out of range [0,10]", score.Quality)
	}
	if score.Originality < 0 || score.Originality > 10 {
		return nil, fmt.Errorf("originality score %.1f out of range [0,10]", score.Originality)
	}
	if score.Correctness < 0 || score.Correctness > 10 {
		return nil, fmt.Errorf("correctness score %.1f out of range [0,10]", score.Correctness)
	}

	// Compute weighted average: Quality 30%, Originality 40%, Correctness 30%
	score.Weighted = score.Quality*0.3 + score.Originality*0.4 + score.Correctness*0.3

	return &score, nil
}

func buildUserPrompt(content *fetcher.ContentResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Evaluate this %s contribution (ID: %d):\n\n",
		content.ContentType, content.Contribution.ID))

	sb.WriteString(fmt.Sprintf("Type: %s\n", content.Contribution.Ctype))
	sb.WriteString(fmt.Sprintf("Contributor: %s\n", content.Contribution.Contributor))
	sb.WriteString(fmt.Sprintf("URI: %s\n\n", content.Contribution.URI))

	// Truncate content to reasonable LLM input size (~16K chars)
	maxChars := 16000
	text := content.Content
	if len(text) > maxChars {
		text = text[:maxChars] + "\n\n[TRUNCATED — content exceeds 16K characters]"
	}

	sb.WriteString("--- BEGIN CONTENT ---\n")
	sb.WriteString(text)
	sb.WriteString("\n--- END CONTENT ---\n\n")

	sb.WriteString("Respond with ONLY the JSON object.\n")
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
