package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"llmgateway/internal/cache"
	"llmgateway/internal/metrics"
	"llmgateway/internal/provider"
	"llmgateway/internal/ratelimit"
	"llmgateway/internal/router"
)

type fake struct {
	calls     atomic.Int32
	failModel string
}

func (f *fake) Complete(_ context.Context, model, _ string) (provider.Result, error) {
	f.calls.Add(1)
	if model == f.failModel {
		return provider.Result{}, errors.New("boom")
	}
	return provider.Result{Text: "ok from " + model, Quality: 0.9}, nil
}

func newServer(f *fake, keys []string, burst float64) *Server {
	cfg := router.Config{MaxSmallChars: 100, HardKeywords: []string{"prove"}, Costs: map[string]float64{"small": 1, "large": 12}}
	s := New(cfg, f, cache.New(time.Minute, 100), ratelimit.New(1000, burst), metrics.New(), keys)
	s.Retries = 1
	return s
}

func post(h http.Handler, prompt, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/v1/complete", strings.NewReader(`{"prompt":"`+prompt+`"}`))
	if key != "" {
		r.Header.Set("X-API-Key", key)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRoutesAndCaches(t *testing.T) {
	f := &fake{}
	h := newServer(f, nil, 100).Handler()
	if w := post(h, "hello", ""); w.Header().Get("X-Model") != "small" {
		t.Fatalf("want small, got %q", w.Header().Get("X-Model"))
	}
	if w := post(h, "prove it", ""); w.Header().Get("X-Model") != "large" {
		t.Fatal("hard prompt should go large")
	}
	post(h, "hello", "")
	if f.calls.Load() != 2 {
		t.Fatalf("second identical prompt should hit cache, provider calls=%d", f.calls.Load())
	}
}

func TestFallback(t *testing.T) {
	f := &fake{failModel: "small"}
	w := post(newServer(f, nil, 100).Handler(), "hello", "")
	if w.Code != 200 || w.Header().Get("X-Model") != "large" {
		t.Fatalf("expected fallback to large, got %d %q", w.Code, w.Header().Get("X-Model"))
	}
}

func TestAuthAndRateLimit(t *testing.T) {
	h := newServer(&fake{}, []string{"k1"}, 1).Handler()
	if post(h, "a", "").Code != 401 {
		t.Fatal("missing key should be 401")
	}
	if post(h, "a", "k1").Code != 200 {
		t.Fatal("valid key should pass")
	}
	if post(h, "b", "k1").Code != 429 {
		t.Fatal("burst of 1 should rate limit second call")
	}
}
