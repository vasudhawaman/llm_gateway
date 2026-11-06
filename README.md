# LLM Routing Gateway

A Go service that sits in front of LLM providers. It authenticates, rate-limits, caches, routes each prompt to a small or large model, retries with backoff, falls back when a model fails, and exposes Prometheus metrics. An eval harness runs in CI and blocks changes that hurt quality or raise cost.

```
client -> auth -> rate limit -> cache -> router -> provider (retry, breaker, fallback) -> metrics
```

Status: runs end to end against a **mock provider**. Add a real adapter in `internal/provider` to call a live model.

## Run it

```bash
go run ./cmd/gateway                      # embedded mock provider, port 8080
curl -s localhost:8080/v1/complete -d '{"prompt":"capital of France?"}'
curl -s localhost:8080/v1/complete -d '{"prompt":"Prove that sqrt(2) is irrational"}'
curl -s localhost:8080/metrics
```

Full stack with Prometheus (:9090) and Grafana (:3000): `make docker`.

## Develop

```bash
make test       # vet + race-enabled unit tests
make eval       # replay eval/dataset.json, compare with eval/baseline.json
make baseline   # accept the current numbers as the new baseline
```

Routing rules live in `config/routing.json`. Change a threshold, run `make eval`, and see whether quality or cost regressed before you open a PR.

## Configuration (env vars)

| Var | Default | Meaning |
|---|---|---|
| PORT | 8080 | listen port (hosts like Render and Cloud Run set it) |
| CONFIG | config/routing.json | routing rules and per-model costs |
| PROVIDER_URL | empty | external provider; empty = embedded mock |
| API_KEYS | empty | comma-separated keys; empty disables auth |
| RATE_RPS / RATE_BURST | 20 / 40 | per-key token bucket |
| LATENCY_SCALE / FAIL_RATE | 1 / 0 | mock latency multiplier and failure rate |

## Deploy

See the step-by-step guide you were given: GitHub, Render (free), Cloud Run (free tier), Kubernetes (kind), Terraform.
