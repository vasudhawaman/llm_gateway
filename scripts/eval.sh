#!/usr/bin/env bash
# Starts mock LLM + gateway locally, runs the eval, tears everything down.
# Pass -write-baseline to record a new baseline; set CONFIG to test a different routing file.
set -euo pipefail
mkdir -p bin
go build -o bin/mockllm ./cmd/mockllm
go build -o bin/gateway ./cmd/gateway
go build -o bin/eval ./cmd/eval
LATENCY_SCALE=0.02 PORT=9000 ./bin/mockllm >/dev/null 2>&1 & M=$!
PROVIDER_URL=http://localhost:9000 PORT=8080 RATE_RPS=10000 RATE_BURST=10000 ./bin/gateway >/dev/null 2>&1 & G=$!
trap 'kill $M $G 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -sf localhost:8080/healthz >/dev/null && break; sleep 0.2; done
./bin/eval -url http://localhost:8080 -data eval/dataset.json -baseline eval/baseline.json "$@"
