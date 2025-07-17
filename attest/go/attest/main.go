package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"regexp"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags.
	attestorProject = flag.String("attestor_project", "", "ID of the project containing the attestor")
	attestor        = flag.String("attestor", "", "ID of the attestor")
	host            = flag.String("host", "localhost", "HTTP service host")
	port            = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort        = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	// Global Binary Authorization API client.
	httpClient *http.Client
)

func checkAttestation(ctx context.Context, attestorProject, attestor, imageID string) (bool, error) {
	output, err := exec.RunSimple(ctx, fmt.Sprintf("gcloud container binauthz attestations list --attestor-project=%s --attestor=%s --artifact-url=%s", attestorProject, attestor, imageID))
	if err != nil {
		return false, skerr.Wrap(err)
	}
	// TODO(borenet): What is the behavior when there are no attestations?
	// I don't currently have access to run this command.
	if output == "" {
		return false, nil
	}
	return true, nil
}

var validImageRegex = regexp.MustCompile(`[0-9A-Za-z_.]+\/[0-9A-Za-z_-]+\/[0-9A-Za-z_-]+@sha256:[0-9a-f]{64}`)

func handler(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()["image"]
	if len(values) != 1 {
		http.Error(w, "expected a single value for `image`", http.StatusBadRequest)
		return
	}
	imageID := values[0]
	if !validImageRegex.MatchString(imageID) {
		http.Error(w, "expected image of the form gcr.io/project/repository@sha256:digest", http.StatusBadRequest)
		return
	}
	hasAttestation, err := checkAttestation(r.Context(), *attestorProject, *attestor, imageID)
	if err != nil {
		sklog.Errorf("Failed checking attestation of %s: %s", imageID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !hasAttestation {
		// TODO(borenet): We could consider using a different status code here,
		// for example 200 (or possibly 204 No Content) but still return
		// "no attestation found", to differentiate from a typical 404.
		http.Error(w, "no attestation found", http.StatusNotFound)
		return
	}
	_, _ = fmt.Fprintln(w, "found valid attestation")
}

func main() {
	common.InitWithMust(
		"attest",
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	if *attestorProject == "" {
		sklog.Fatal("--attestor_project is required.")
	}

	if *attestor == "" {
		sklog.Fatal("--attestor is required.")
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	h := httputils.LoggingRequestResponse(http.HandlerFunc(handler))
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, h))
}
