// Package ratelimit implements a simple token bucket rate limiter.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a token bucket rate limiter.
type Limiter struct {
	mu       sync.Mutex
	tokens   float64
	maxBurst float64
	rate     float64 // tokens per second
	lastTime time.Time
}

// New creates a limiter that allows rate requests per second with a burst capacity.
func New(rps float64, burst int) *Limiter {
	return &Limiter{
		tokens:   float64(burst),
		maxBurst: float64(burst),
		rate:     rps,
		lastTime: time.Now(),
	}
}

// Allow returns true if a request is allowed, consuming one token.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastTime).Seconds()
	l.lastTime = now

	// Replenish tokens
	l.tokens += elapsed * l.rate
	if l.tokens > l.maxBurst {
		l.tokens = l.maxBurst
	}

	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
