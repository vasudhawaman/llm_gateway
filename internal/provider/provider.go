// Package provider defines the interface the gateway uses to call a model.
// Add a real adapter (Gemini, OpenAI, ...) by implementing Provider.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"llmgateway/internal/mock"
)

type Result struct {
	Text    string
	Quality float64
}

type Provider interface {
	Complete(ctx context.Context, model, prompt string) (Result, error)
}

// HTTP calls an external service that speaks the mock's /v1/generate protocol.
type HTTP struct {
	URL    string
	Client *http.Client
}

func (h *HTTP) Complete(ctx context.Context, model, prompt string) (Result, error) {
	body, _ := json.Marshal(map[string]string{"model": model, "prompt": prompt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL+"/v1/generate", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	var out struct {
		Text    string  `json:"text"`
		Quality float64 `json:"quality"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Result{}, err
	}
	return Result{out.Text, out.Quality}, nil
}

// Embedded runs the mock in-process, so one container is enough for a free demo deployment.
type Embedded struct{ G mock.Gen }

func (e Embedded) Complete(ctx context.Context, model, prompt string) (Result, error) {
	t, q, err := e.G.Do(ctx, model, prompt)
	return Result{t, q}, err
}
