// Package fetcher retrieves contribution metadata from the chain (via posd CLI)
// and fetches content from IPFS/HTTP storage pointers.
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"gov-copilot/internal/pocconfig"
)

// Contribution mirrors the on-chain Contribution struct (read-only fields).
type Contribution struct {
	ID           uint64 `json:"id"`
	Contributor  string `json:"contributor"`
	Ctype        string `json:"ctype"`
	URI          string `json:"uri"`
	Verified     bool   `json:"verified"`
	BlockHeight  int64  `json:"block_height"`
	IsDerivative bool   `json:"is_derivative"`
}

// ContentResult holds the fetched content and metadata.
type ContentResult struct {
	Contribution Contribution
	Content      string // raw text/code content from IPFS
	ContentType  string // "text", "code", "dataset"
	FetchError   error  // non-nil if content couldn't be fetched (endorsement still possible based on metadata)
}

// Fetcher queries the chain for contribution details and fetches content from IPFS.
type Fetcher struct {
	cfg        *pocconfig.Config
	httpClient *http.Client
}

// New creates a content fetcher.
func New(cfg *pocconfig.Config) *Fetcher {
	return &Fetcher{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.FetchTimeout,
		},
	}
}

// FetchContribution queries the chain for a contribution by ID and returns its metadata.
func (f *Fetcher) FetchContribution(ctx context.Context, contributionID uint64) (*Contribution, error) {
	args := []string{
		"query", "poc", "contribution", fmt.Sprintf("%d", contributionID),
		"--node", f.cfg.NodeRPCURL,
		"--chain-id", f.cfg.ChainID,
		"-o", "json",
	}

	cmd := exec.CommandContext(ctx, "posd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("posd query poc contribution %d: %w\n%s", contributionID, err, string(out))
	}

	// The response wraps the contribution in a "contribution" field
	var result struct {
		Contribution struct {
			ID           string `json:"id"`
			Contributor  string `json:"contributor"`
			Ctype        string `json:"ctype"`
			URI          string `json:"uri"`
			Verified     bool   `json:"verified"`
			BlockHeight  string `json:"block_height"`
			IsDerivative bool   `json:"is_derivative"`
		} `json:"contribution"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse contribution response: %w\nraw: %s", err, truncate(string(out), 200))
	}

	var id uint64
	fmt.Sscanf(result.Contribution.ID, "%d", &id)
	var height int64
	fmt.Sscanf(result.Contribution.BlockHeight, "%d", &height)

	return &Contribution{
		ID:           id,
		Contributor:  result.Contribution.Contributor,
		Ctype:        result.Contribution.Ctype,
		URI:          result.Contribution.URI,
		Verified:     result.Contribution.Verified,
		BlockHeight:  height,
		IsDerivative: result.Contribution.IsDerivative,
	}, nil
}

// FetchContent retrieves the raw content from the contribution's storage pointer.
// Supports IPFS (ipfs://), HTTP(S), and returns an error for unsupported schemes.
func (f *Fetcher) FetchContent(ctx context.Context, contrib *Contribution) (*ContentResult, error) {
	result := &ContentResult{
		Contribution: *contrib,
		ContentType:  normalizeType(contrib.Ctype),
	}

	uri := contrib.URI
	if uri == "" {
		result.FetchError = fmt.Errorf("contribution %d has empty URI", contrib.ID)
		return result, nil
	}

	fetchURL, err := resolveURI(uri, f.cfg.IPFSGatewayURL)
	if err != nil {
		result.FetchError = fmt.Errorf("resolve URI %q: %w", uri, err)
		return result, nil
	}

	log.Printf("[fetcher] fetching content for contribution %d from %s", contrib.ID, truncate(fetchURL, 80))

	content, err := f.httpFetch(ctx, fetchURL)
	if err != nil {
		result.FetchError = fmt.Errorf("HTTP fetch %q: %w", fetchURL, err)
		return result, nil
	}

	result.Content = content
	return result, nil
}

// httpFetch performs an HTTP GET with timeout and size limiting.
func (f *Fetcher) httpFetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// Read with size limit to prevent OOM
	limitedReader := io.LimitReader(resp.Body, f.cfg.MaxContentBytes)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// resolveURI converts an IPFS URI to an HTTP gateway URL, or validates HTTP(S) URLs.
func resolveURI(uri, ipfsGateway string) (string, error) {
	switch {
	case strings.HasPrefix(uri, "ipfs://"):
		// ipfs://QmXxx... → https://ipfs.io/ipfs/QmXxx...
		cid := strings.TrimPrefix(uri, "ipfs://")
		if cid == "" {
			return "", fmt.Errorf("empty IPFS CID")
		}
		gateway := strings.TrimRight(ipfsGateway, "/")
		return fmt.Sprintf("%s/%s", gateway, cid), nil

	case strings.HasPrefix(uri, "http://"), strings.HasPrefix(uri, "https://"):
		return uri, nil

	default:
		return "", fmt.Errorf("unsupported URI scheme: %s", uri)
	}
}

func normalizeType(ctype string) string {
	ctype = strings.ToLower(strings.TrimSpace(ctype))
	switch ctype {
	case "code", "source", "patch":
		return "code"
	case "dataset", "data":
		return "dataset"
	default:
		return "text"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// WaitForIPFS waits until the IPFS gateway is reachable or timeout expires.
func (f *Fetcher) WaitForIPFS(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url := strings.TrimRight(f.cfg.IPFSGatewayURL, "/") + "/api/v0/version"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil // IPFS health check is best-effort
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Printf("[fetcher] IPFS gateway not reachable at %s (will retry on demand)", f.cfg.IPFSGatewayURL)
		return nil
	}
	resp.Body.Close()
	return nil
}
