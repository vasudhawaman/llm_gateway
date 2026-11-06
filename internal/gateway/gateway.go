// Package gateway wires auth, rate limiting, cache, routing, retries, fallback and metrics.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"llmgateway/internal/cache"
	"llmgateway/internal/metrics"
	"llmgateway/internal/provider"
	"llmgateway/internal/ratelimit"
	"llmgateway/internal/router"
)

type Breaker struct {
	mu        sync.Mutex
	fails     int
	openUntil time.Time
	threshold int
	cooldown  time.Duration
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return time.Now().After(b.openUntil)
}

func (b *Breaker) Record(ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ok {
		b.fails = 0
		return
	}
	b.fails++
	if b.fails >= b.threshold {
		b.openUntil = time.Now().Add(b.cooldown)
		b.fails = 0
	}
}

type Server struct {
	Router   router.Config
	Prov     provider.Provider
	Cache    *cache.Cache
	Lim      *ratelimit.Limiter
	M        *metrics.Metrics
	Keys     map[string]bool // empty = auth disabled
	Retries  int
	Timeout  time.Duration
	breakers map[string]*Breaker
}

func New(r router.Config, p provider.Provider, c *cache.Cache, l *ratelimit.Limiter, m *metrics.Metrics, keys []string) *Server {
	km := map[string]bool{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			km[k] = true
		}
	}
	br := map[string]*Breaker{}
	for _, n := range []string{"small", "large"} {
		br[n] = &Breaker{threshold: 5, cooldown: 10 * time.Second}
	}
	return &Server{Router: r, Prov: p, Cache: c, Lim: l, M: m, Keys: km, Retries: 2, Timeout: 5 * time.Second, breakers: br}
}

type request struct {
	Prompt string `json:"prompt"`
}

type response struct {
	Text      string  `json:"text"`
	Model     string  `json:"model"`
	Cached    bool    `json:"cached"`
	Cost      float64 `json:"cost"`
	Quality   float64 `json:"quality"`
	LatencyMS int64   `json:"latency_ms"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/complete", s.complete)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.Handle("/metrics", s.M.Handler())
	return mux
}

func (s *Server) fail(w http.ResponseWriter, start time.Time, code int, msg string) {
	s.M.Observe("none", strconv.Itoa(code), time.Since(start), 0)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) complete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		s.fail(w, start, http.StatusMethodNotAllowed, "use POST")
		return
	}
	key := r.Header.Get("X-API-Key")
	if len(s.Keys) > 0 && !s.Keys[key] {
		s.fail(w, start, http.StatusUnauthorized, "invalid API key")
		return
	}
	id := key
	if id == "" {
		id, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	if !s.Lim.Allow(id) {
		w.Header().Set("Retry-After", "1")
		s.fail(w, start, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		s.fail(w, start, http.StatusBadRequest, `body must be {"prompt": "..."}`)
		return
	}

	ck := cache.Key(req.Prompt)
	if !strings.Contains(r.Header.Get("Cache-Control"), "no-cache") {
		if b, ok := s.Cache.Get(ck); ok {
			var hit response
			if json.Unmarshal(b, &hit) == nil {
				hit.Cached, hit.Cost, hit.LatencyMS = true, 0, time.Since(start).Milliseconds()
				s.M.Observe("cache", "200", time.Since(start), 0)
				write(w, hit)
				return
			}
		}
	}

	primary := s.Router.Choose(req.Prompt)
	var res provider.Result
	used := ""
	for _, m := range []string{primary, router.Other(primary)} {
		b := s.breakers[m]
		if !b.Allow() {
			continue
		}
		out, err := s.call(r.Context(), m, req.Prompt)
		b.Record(err == nil)
		if err == nil {
			res, used = out, m
			break
		}
	}
	if used == "" {
		s.fail(w, start, http.StatusBadGateway, "all providers failed")
		return
	}
	resp := response{Text: res.Text, Model: used, Cost: s.Router.Costs[used], Quality: res.Quality}
	if b, err := json.Marshal(resp); err == nil {
		s.Cache.Set(ck, b)
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	s.M.Observe(used, "200", time.Since(start), resp.Cost)
	write(w, resp)
}

func (s *Server) call(ctx context.Context, model, prompt string) (provider.Result, error) {
	last := errors.New("no attempts")
	for i := 0; i < s.Retries; i++ {
		cctx, cancel := context.WithTimeout(ctx, s.Timeout)
		res, err := s.Prov.Complete(cctx, model, prompt)
		cancel()
		if err == nil {
			return res, nil
		}
		last = err
		backoff := time.Duration(float64(50*time.Millisecond) * math.Pow(2, float64(i)) * (0.5 + rand.Float64()))
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return provider.Result{}, ctx.Err()
		}
	}
	return provider.Result{}, last
}

func write(w http.ResponseWriter, v response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Model", v.Model)
	_ = json.NewEncoder(w).Encode(v)
}
