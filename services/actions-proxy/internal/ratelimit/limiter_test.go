package ratelimit_test

import (
	"testing"

	"actions-proxy/internal/ratelimit"
)

func TestLimiter_AllowsBurst(t *testing.T) {
	l := ratelimit.New(1.0, 3)

	// Should allow burst of 3
	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Fatalf("request %d should be allowed (burst)", i)
		}
	}

	// 4th should be rejected
	if l.Allow() {
		t.Fatal("4th request should be rejected after burst exhausted")
	}
}

func TestLimiter_ZeroBurst(t *testing.T) {
	l := ratelimit.New(100.0, 0)
	// With zero burst, nothing is allowed immediately
	// (tokens start at 0)
	if l.Allow() {
		t.Fatal("should not allow with zero burst tokens")
	}
}

func TestLimiter_SingleToken(t *testing.T) {
	l := ratelimit.New(0.1, 1)

	// First request uses the initial token
	if !l.Allow() {
		t.Fatal("first request should be allowed")
	}

	// Second should be denied (only 0.1 tokens/sec, need to wait)
	if l.Allow() {
		t.Fatal("second request should be denied")
	}
}
