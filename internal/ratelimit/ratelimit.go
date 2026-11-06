// Package ratelimit implements a per-key token bucket.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

type Limiter struct {
	mu    sync.Mutex
	b     map[string]*bucket
	rate  float64 // tokens per second
	burst float64
	now   func() time.Time
}

func New(rate, burst float64) *Limiter {
	return &Limiter{b: map[string]*bucket{}, rate: rate, burst: burst, now: time.Now}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.b[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.b[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
