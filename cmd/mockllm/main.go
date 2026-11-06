package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"llmgateway/internal/mock"
)

func f(k string, d float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return v
	}
	return d
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}
	g := mock.Gen{Scale: f("LATENCY_SCALE", 1), FailRate: f("FAIL_RATE", 0)}
	log.Printf("mock LLM on :%s (scale=%g fail=%g)", port, g.Scale, g.FailRate)
	log.Fatal(http.ListenAndServe(":"+port, g.Handler()))
}
