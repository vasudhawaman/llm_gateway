// eval replays a labeled prompt set through the gateway and fails if quality or cost regress.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"
)

type result struct {
	Quality float64 `json:"quality"`
	Cost    float64 `json:"cost"`
	P99MS   float64 `json:"p99_ms"`
}

func main() {
	url := flag.String("url", "http://localhost:8080", "gateway base URL")
	data := flag.String("data", "eval/dataset.json", "JSON array of prompts")
	basePath := flag.String("baseline", "eval/baseline.json", "baseline file")
	write := flag.Bool("write-baseline", false, "overwrite the baseline with this run")
	qTol := flag.Float64("quality-tolerance", 0.02, "allowed quality drop")
	cTol := flag.Float64("cost-tolerance", 0.05, "allowed relative cost increase")
	flag.Parse()

	var prompts []string
	b, err := os.ReadFile(*data)
	check(err)
	check(json.Unmarshal(b, &prompts))

	var q, c float64
	var lats []float64
	for _, p := range prompts {
		body, _ := json.Marshal(map[string]string{"prompt": p})
		req, _ := http.NewRequest(http.MethodPost, *url+"/v1/complete", bytes.NewReader(body))
		req.Header.Set("Cache-Control", "no-cache")
		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		check(err)
		var out struct{ Quality, Cost float64 }
		if resp.StatusCode != 200 {
			fail("gateway returned %d", resp.StatusCode)
		}
		check(json.NewDecoder(resp.Body).Decode(&out))
		resp.Body.Close()
		q += out.Quality
		c += out.Cost
		lats = append(lats, float64(time.Since(start).Milliseconds()))
	}
	sort.Float64s(lats)
	n := float64(len(prompts))
	got := result{Quality: q / n, Cost: c / n, P99MS: lats[int(float64(len(lats)-1)*0.99)]}
	fmt.Printf("quality=%.3f  cost/request=%.2f  p99=%.0fms  (n=%d)\n", got.Quality, got.Cost, got.P99MS, len(prompts))

	if *write {
		out, _ := json.MarshalIndent(got, "", "  ")
		check(os.WriteFile(*basePath, append(out, '\n'), 0o644))
		fmt.Println("baseline written to", *basePath)
		return
	}
	var base result
	bb, err := os.ReadFile(*basePath)
	check(err)
	check(json.Unmarshal(bb, &base))
	ok := true
	if got.Quality < base.Quality-*qTol {
		fmt.Printf("FAIL quality %.3f < baseline %.3f - %.2f\n", got.Quality, base.Quality, *qTol)
		ok = false
	}
	if got.Cost > base.Cost*(1+*cTol) {
		fmt.Printf("FAIL cost %.2f > baseline %.2f + %.0f%%\n", got.Cost, base.Cost, *cTol*100)
		ok = false
	}
	if !ok {
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func check(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}
