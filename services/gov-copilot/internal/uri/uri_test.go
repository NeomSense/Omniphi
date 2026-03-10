package uri_test

import (
	"testing"

	"gov-copilot/internal/uri"
)

func TestMakePublicURI_Basic(t *testing.T) {
	got, err := uri.MakePublicURI("https://copilot.omniphi.org/reports", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://copilot.omniphi.org/reports/42.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMakePublicURI_TrailingSlash(t *testing.T) {
	got, err := uri.MakePublicURI("https://example.com/reports/", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://example.com/reports/7.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMakePublicURI_MultipleTrailingSlashes(t *testing.T) {
	got, err := uri.MakePublicURI("https://example.com///", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://example.com/1.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMakePublicURI_EmptyBaseURL(t *testing.T) {
	_, err := uri.MakePublicURI("", 1)
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
}

func TestMakePublicURI_ZeroProposalID(t *testing.T) {
	_, err := uri.MakePublicURI("https://example.com", 0)
	if err == nil {
		t.Fatal("expected error for zero proposal ID")
	}
}

func TestMakePublicURI_LocalhostDev(t *testing.T) {
	got, err := uri.MakePublicURI("http://127.0.0.1:8088/reports", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://127.0.0.1:8088/reports/99.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
