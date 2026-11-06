// Package cache is a small in-memory TTL cache keyed by a prompt hash.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type entry struct {
	v   []byte
	exp time.Time
}

type Cache struct {
	mu  sync.Mutex
	m   map[string]entry
	ttl time.Duration
	max int
}

func New(ttl time.Duration, max int) *Cache {
	return &Cache{m: map[string]entry{}, ttl: ttl, max: max}
}

func Key(prompt string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(prompt))))
	return hex.EncodeToString(h[:])
}

func (c *Cache) Get(k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Now().After(e.exp) {
		delete(c.m, k)
		return nil, false
	}
	return e.v, true
}

func (c *Cache) Set(k string, v []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		now := time.Now()
		for kk, e := range c.m {
			if now.After(e.exp) {
				delete(c.m, kk)
			}
		}
		for kk := range c.m { // still full: evict an arbitrary entry
			if len(c.m) < c.max {
				break
			}
			delete(c.m, kk)
		}
	}
	c.m[k] = entry{v: v, exp: time.Now().Add(c.ttl)}
}
