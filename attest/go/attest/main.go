package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"go.skia.org/infra/attest/go/attestation"
	"go.skia.org/infra/attest/go/types"
	local_cache "go.skia.org/infra/go/cache/local"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/time/rate"
)

var (
	// Flags.
	attestor             = flag.String("attestor", "", "Fully-qualified resource name of the attestor (e.g., 'projects/my-project/attestors/my-attestor')")
	cacheSize            = flag.Int("cache_size", 10000, "Maximum number of verification results to store in the in-memory cache.")
	maxRequestsPerMinute = flag.Int("max_requests_per_minute", 50, "Per-minute rate limit on calls to attestation APIs.")
	host                 = flag.String("host", "localhost", "HTTP service host")
	port                 = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort             = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	local                = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func main() {
	common.InitWithMust(
		"attest",
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	ctx := context.Background()

	var client types.Client
	var err error
	client, err = attestation.NewClient(ctx, *attestor)
	if err != nil {
		sklog.Fatal(err)
	}

	if *maxRequestsPerMinute > 0 {
		// Our quota is based on requests per minute, but rate.Limiter uses a
		// per-second limit. Set the maximum burst to be our per-minute limit
		// (so that we can use our entire per-minute quota immediately if
		// necessary) and compute the per-second limit.
		perSecondLimit := (float64(*maxRequestsPerMinute) / float64(time.Minute)) * float64(time.Second)
		rl := rate.NewLimiter(rate.Limit(perSecondLimit), *maxRequestsPerMinute)
		client = types.WithRateLimiter(client, rl)
	}

	if *cacheSize > 0 {
		cache, err := local_cache.New(*cacheSize)
		if err != nil {
			sklog.Fatal(err)
		}
		client = types.WithCache(client, cache)
	}

	server := types.NewServer(client)

	h := httputils.LoggingRequestResponse(server)
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, h))
}
