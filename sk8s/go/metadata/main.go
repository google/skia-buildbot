package main

// This program emulates the meta data server in GCE and is intended to be used
// in the Skolo just for serving tokens to swarming clients. Note that the
// serving path is hard-coded to
// /computeMetadata/v1/instance/service-accounts/default/token.
//
// Note that the token comes from using Google default credentials, so the
// GOOGLE_APPLICATION_CREDENTIALS environment variable must be set.

import (
	"encoding/json"
	"flag"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
)

// refreshInterval is how often we should refresh the token.
var refreshInterval = 10 * time.Minute

// Flags.
var (
	local    = flag.Bool("local", false, "True if running locally, not in prod.")
	port     = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

type server struct {
	successfulRefresh metrics2.Counter
	failedRefresh     metrics2.Counter

	// mutex protects tokenSource and latestToken.
	mutex       sync.Mutex
	tokenSource oauth2.TokenSource
	latestToken *oauth2.Token
}

// newServer creates a new *server with a running Go routine that refreshes the token.
func newServer() (*server, error) {
	tokenSource, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create token source.")
	}

	ret := &server{
		tokenSource:       tokenSource,
		successfulRefresh: metrics2.GetCounter("metadata_successful_refresh"),
		failedRefresh:     metrics2.GetCounter("metadata_failed_refresh"),
	}

	if err := ret.step(); err != nil {
		return nil, skerr.Wrapf(err, "Failed to get token.")
	}

	go ret.refresh()

	return ret, nil
}

func (s *server) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mutex.Lock()
	t := s.latestToken
	s.mutex.Unlock()
	// Copied from
	// https://github.com/golang/oauth2/blob/f6093e37b6cb4092101a298aba5d794eb570757f/google/google.go#L185
	res := struct {
		AccessToken  string `json:"access_token"`
		ExpiresInSec int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}{
		AccessToken:  t.AccessToken,
		ExpiresInSec: int(t.Expiry.Sub(time.Now()).Seconds()),
		TokenType:    t.TokenType,
	}
	sklog.Infof("Token requested by %s, serving %s", r.RemoteAddr, res.AccessToken[len(res.AccessToken)-8:])
	if err := json.NewEncoder(w).Encode(res); err != nil {
		httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
		return
	}
}

// step does a single refresh of the oauth token.
func (s *server) step() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// We need to re-create the tokenSource to force it to fetch a fresh token.
	// See https://github.com/golang/oauth2/pull/396.
	tokenSource, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create token source.")
	}
	s.tokenSource = tokenSource

	latestToken, err := s.tokenSource.Token()
	if err != nil {
		s.failedRefresh.Inc(1)
		return skerr.Wrapf(err, "Failed to retrieve token.")
	}

	sklog.Info("Successfully refreshed.")
	s.successfulRefresh.Inc(1)
	s.latestToken = latestToken
	return nil
}

// refresh handles periodically refreshing the token.
//
// This function does not return and should be run as a Go routine.
func (s *server) refresh() {
	for range time.Tick(refreshInterval) {
		if err := s.step(); err != nil {
			sklog.Errorf("Failed to refresh token: %s")
		}
	}
}

func main() {
	common.InitWithMust(
		"metadata",
		common.PrometheusOpt(promPort),
	)

	s, err := newServer()
	if err != nil {
		sklog.Fatal(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/computeMetadata/v1/instance/service-accounts/default/token", s.handleTokenRequest)
	http.Handle("/", httputils.Healthz(httputils.LoggingGzipRequestResponse(r)))
	sklog.Infof("Ready to serve on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
