.PHONY: run test eval baseline docker
run:      ; go run ./cmd/gateway
test:     ; go vet ./... && go test -race ./...
eval:     ; ./scripts/eval.sh
baseline: ; ./scripts/eval.sh -write-baseline
docker:   ; docker compose up --build
