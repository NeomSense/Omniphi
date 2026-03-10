// Package uri provides public URI generation for advisory reports.
package uri

import (
	"fmt"
	"strings"
)

// MakePublicURI constructs a public report URI from a base URL and proposal ID.
// The base URL trailing slashes are stripped; the result is "<base>/<id>.json".
// Returns an error if baseURL is empty or proposalID is 0.
func MakePublicURI(baseURL string, proposalID uint64) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "", fmt.Errorf("base URL cannot be empty")
	}
	if proposalID == 0 {
		return "", fmt.Errorf("proposal ID cannot be zero")
	}
	return fmt.Sprintf("%s/%d.json", baseURL, proposalID), nil
}
