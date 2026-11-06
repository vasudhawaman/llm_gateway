// Package metrics exposes Prometheus-format metrics with no external dependencies.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var bounds = []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

type hist struct {
	counts []int64
	sum    float64
	n      int64
}

type Metrics struct {
	mu    sync.Mutex
	reqs  map[string]int64
	cost  float64
	hists map[string]*hist
}

func New() *Metrics {
	return &Metrics{reqs: map[string]int64{}, hists: map[string]*hist{}}
}

func (m *Metrics) Observe(route, status string, d time.Duration, cost float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs[route+"|"+status]++
	m.cost += cost
	h := m.hists[route]
	if h == nil {
		h = &hist{counts: make([]int64, len(bounds))}
		m.hists[route] = h
	}
	s := d.Seconds()
	h.sum += s
	h.n++
	for i, b := range bounds {
		if s <= b {
			h.counts[i]++
		}
	}
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		var sb strings.Builder
		sb.WriteString("# TYPE gateway_requests_total counter\n")
		keys := make([]string, 0, len(m.reqs))
		for k := range m.reqs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := strings.SplitN(k, "|", 2)
			fmt.Fprintf(&sb, "gateway_requests_total{route=%q,status=%q} %d\n", p[0], p[1], m.reqs[k])
		}
		sb.WriteString("# TYPE gateway_cost_total counter\n")
		fmt.Fprintf(&sb, "gateway_cost_total %g\n", m.cost)
		sb.WriteString("# TYPE gateway_request_duration_seconds histogram\n")
		routes := make([]string, 0, len(m.hists))
		for r := range m.hists {
			routes = append(routes, r)
		}
		sort.Strings(routes)
		for _, r := range routes {
			h := m.hists[r]
			for i, b := range bounds {
				fmt.Fprintf(&sb, "gateway_request_duration_seconds_bucket{route=%q,le=\"%g\"} %d\n", r, b, h.counts[i])
			}
			fmt.Fprintf(&sb, "gateway_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", r, h.n)
			fmt.Fprintf(&sb, "gateway_request_duration_seconds_sum{route=%q} %g\n", r, h.sum)
			fmt.Fprintf(&sb, "gateway_request_duration_seconds_count{route=%q} %d\n", r, h.n)
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(sb.String()))
	})
}
