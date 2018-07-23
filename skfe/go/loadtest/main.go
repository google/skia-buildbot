/*
loadtest is a simple loadtesting tool.

The tool spins up N workers to make requests, then pumps M requests through
those workers and reports how long that took.

*/
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
)

// flags
var (
	numWorkers = flag.Int("num_workers", 100, "Maximum number of parallel inflight requests.")
	numFetches = flag.Int("num_requests", 1000, "Total number of requests to make.")
)

// Targets to exercise.
var targets = []Target{
	{
		URL:  "https://www.skia.org/",
		Code: 200,
	},
}

// Target defines a request to make to the server under load test.
type Target struct {
	URL  string
	Code int
}

// SimpleStats is simple point statistics for an array of float64s.
type SimpleStats struct {
	Min    float64
	Max    float64
	Mean   float64
	StdDev float64

	name    string
	samples []float64
}

// NewSimpleStats creates a SimpleStats from a []float64 and a display name.
func NewSimpleStats(a []float64, name string) *SimpleStats {
	sort.Float64s(a)
	sum := 0.0
	for _, x := range a {
		sum += x
	}
	mean := sum / float64(len(a))

	sum = 0.0
	for _, x := range a {
		sum += (x - mean) * (x - mean)
	}
	stddev := math.Sqrt(sum / float64(len(a)))
	return &SimpleStats{
		Min:     a[0],
		Max:     a[len(a)-1],
		Mean:    mean,
		StdDev:  stddev,
		samples: a,
		name:    name}
}

// Percentile returns the sample at the given percent from the beginning of the sorted
// samples.
func (s SimpleStats) Percentile(p float64) float64 {
	return s.samples[int(p*float64(len(s.samples)))]
}

// String returns a nicely formatted representation of the SimpleStats.
func (s SimpleStats) String() string {
	return fmt.Sprintf("%v -- Min: %.2f Max: %.2f Mean: %.2f σ: %.2f 95%%: %.2f", s.name, s.Min, s.Max, s.Mean, s.StdDev, s.Percentile(0.95))
}

// startWorkers creates a worker pool of numWorkers workers to make requests.
//
// Requests to make arrive over the targetCh channel, latency measurements in
// millis go out over the latencies channel, and wg is used to synchronize the
// workers' completion.
func startWorkers(targets <-chan Target, latencies chan<- float64, wg *sync.WaitGroup) error {

	wg.Add(*numWorkers)
	for i := 0; i < *numWorkers; i++ {
		c := httputils.NewTimeoutClient()
		go func(c *http.Client) {
			for t := range targets {
				t0 := time.Now()
				resp, err := c.Get(t.URL)
				t1 := time.Now()

				if err != nil {
					fmt.Printf("Failure for Get: %v %v\n", t.URL, err)
					continue
				}
				// TODO(jcgregorio) Add stats for failures if we start seeing them.
				if resp.StatusCode != t.Code {
					fmt.Printf("Wrong status code expected %v, got %v at %v\n", t.Code, resp.StatusCode, t.URL)
				}
				duration := t1.Sub(t0)
				latencies <- float64(duration.Nanoseconds() / 1000000)
			}
			wg.Done()
		}(c)
	}

	return nil
}

func main() {
	common.Init()

	var err error

	var wg sync.WaitGroup

	// Make a channel to deliver work.
	targetCh := make(chan Target)

	// Record the latency measurements in millis for each request.
	latencySamples := make([]float64, 0)
	latencies := make(chan float64)

	go func() {
		for m := range latencies {
			latencySamples = append(latencySamples, m)
		}
	}()

	err = startWorkers(targetCh, latencies, &wg)
	if err != nil {
		log.Fatalf("Failure starting workers: %v\n", err)
	}

	b0 := time.Now()

	// Pump requests out to all the workers to do.
	for i := 0; i < *numFetches; i++ {
		t := targets[rand.Int()%len(targets)]
		targetCh <- t
	}
	close(targetCh)

	// Wait for all HTTP requests to complete.
	wg.Wait()

	b1 := time.Now()

	fmt.Print("\n")
	fmt.Printf("Total time of run: %v\n", b1.Sub(b0))
	fmt.Printf("Average QPS: %.2f\n", float64(*numFetches)/b1.Sub(b0).Seconds())
	fmt.Println(NewSimpleStats(latencySamples, "Latency").String())
}
