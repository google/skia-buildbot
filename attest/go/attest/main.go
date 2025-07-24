package main

import (
	"context"
	"flag"
	"net/http"

	"go.skia.org/infra/attest/go/attestation"
	"go.skia.org/infra/attest/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags.
	attestor = flag.String("attestor", "", "Fully-qualified resource name of the attestor (e.g., 'projects/my-project/attestors/my-attestor')")
	host     = flag.String("host", "localhost", "HTTP service host")
	port     = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
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
	client, err := attestation.NewClient(ctx, *attestor)
	if err != nil {
		sklog.Fatal(err)
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
