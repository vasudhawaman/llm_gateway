package ratelimit

import (
	"testing"
	"time"
)

func TestBucket(t *testing.T) {
	l := New(1, 2)
	now := time.Now()
	l.now = func() time.Time { return now }
	if !l.Allow("a") || !l.Allow("a") {
		t.Fatal("burst of 2 should pass")
	}
	if l.Allow("a") {
		t.Fatal("third request should be limited")
	}
	if !l.Allow("b") {
		t.Fatal("keys are independent")
	}
	now = now.Add(1100 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("token should refill")
	}
}
