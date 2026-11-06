// Package mock is a fake LLM used for local runs, CI, load tests and free hosting demos.
package mock

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var ErrFail = errors.New("mock provider failure")

var hardWords = []string{"prove", "step by step", "derive", "refactor", "optimize", "architecture", "debug"}

// IsHard is the mock's ground truth for how difficult a prompt is.
func IsHard(prompt string) bool {
	if len(prompt) > 400 {
		return true
	}
	l := strings.ToLower(prompt)
	for _, w := range hardWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func jitter(prompt string) float64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(prompt))
	return float64(h.Sum32()%1000) / 1000
}

// Quality is deterministic per (model, prompt) so eval runs are repeatable.
func Quality(model, prompt string) float64 {
	j := jitter(prompt) * 0.04
	switch {
	case model == "small" && IsHard(prompt):
		return 0.55 + j
	case model == "small":
		return 0.93 + j
	default:
		return 0.95 + j
	}
}

type Gen struct{ Scale, FailRate float64 }

func (g Gen) Do(ctx context.Context, model, prompt string) (string, float64, error) {
	if rand.Float64() < g.FailRate {
		return "", 0, ErrFail
	}
	base := 300.0
	if model == "large" {
		base = 1100
	}
	d := time.Duration(base * g.Scale * (0.7 + rand.Float64()*0.9) * float64(time.Millisecond))
	select {
	case <-time.After(d):
	case <-ctx.Done():
		return "", 0, ctx.Err()
	}
	return "mock answer from " + model, Quality(model, prompt), nil
}

func (g Gen) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/generate", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Model, Prompt string }
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		text, q, err := g.Do(r.Context(), in.Model, in.Prompt)
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"text": text, "quality": q})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	return mux
}
