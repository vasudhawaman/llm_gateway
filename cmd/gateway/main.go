package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"llmgateway/internal/cache"
	"llmgateway/internal/gateway"
	"llmgateway/internal/metrics"
	"llmgateway/internal/mock"
	"llmgateway/internal/provider"
	"llmgateway/internal/ratelimit"
	"llmgateway/internal/router"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envF(k string, d float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return v
	}
	return d
}

func main() {
	cfg, err := router.Load(env("CONFIG", "config/routing.json"))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	var prov provider.Provider
	if u := os.Getenv("PROVIDER_URL"); u != "" {
		prov = &provider.HTTP{URL: u, Client: &http.Client{}}
		log.Printf("provider: %s", u)
	} else {
		prov = provider.Embedded{G: mock.Gen{Scale: envF("LATENCY_SCALE", 1), FailRate: envF("FAIL_RATE", 0)}}
		log.Print("provider: embedded mock")
	}
	s := gateway.New(cfg, prov, cache.New(10*time.Minute, 10000),
		ratelimit.New(envF("RATE_RPS", 20), envF("RATE_BURST", 40)), metrics.New(),
		strings.Split(os.Getenv("API_KEYS"), ","))
	srv := &http.Server{
		Addr: ":" + env("PORT", "8080"), Handler: s.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sc)
	}()
	log.Printf("gateway listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
