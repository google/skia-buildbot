// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// insightsserver serves perfinsights, a read-only proof of concept that lets LLM agents get Skia
// Perf anomaly insights over REST (/api/v1) and MCP (/mcp), with a landing page (/) and /llms.txt.
// Its database connections are always read-only.
//
// Run it from the perf directory against the local demo database:
//
//	insightsserver --config_filename=./configs/demo_spanner.json \
//	  --connection_string=postgresql://root@127.0.0.1:5432/demo?sslmode=disable
//
// To run it against the production V8 database, use demo_v8.sh (see README.md).
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/config/validate"
	"go.skia.org/infra/perf/go/insights"
	"go.skia.org/infra/perf/go/insights/localonly"
	"go.skia.org/infra/perf/go/insights/site"
	"go.skia.org/infra/perf/go/insights/tracereader"
	"go.skia.org/infra/perf/go/urlprovider"
)

const (
	// alertRefreshSeconds is how often alert configs are reloaded.
	alertRefreshSeconds = 600

	// startupTimeout bounds the database checks done on startup.
	startupTimeout = time.Minute

	shutdownTimeout = 10 * time.Second
)

var (
	configFilename   = flag.String("config_filename", "", "Perf instance config, e.g. ./configs/demo_spanner.json. Required.")
	connectionString = flag.String("connection_string", "", "Database connection string. Overrides the one in the config.")
	listen           = flag.String("listen", "127.0.0.1:8090", "Address to listen on. A non-loopback address turns on --require_auth, unless --uberproxy_pen is set.")
	promPort         = flag.String("prom_port", "localhost:20005", "Address to serve Prometheus metrics on.")
	publicURL        = flag.String("public_url", "", "Base URL clients use to reach this server. Defaults to http://<--listen>.")
	instanceName     = flag.String("instance_name", "", "Perf instance name shown to agents. Defaults to the config's instance_name.")
	instanceURL      = flag.String("instance_url", "", "Perf UI URL used in links. Defaults to the config's URL.")
	visibility       = flag.String("visibility", "internal", "Visibility of the data: internal or public.")
	allowNonDemoDB   = flag.Bool("allow_non_demo_db", false, "Allow databases other than the local demo database. Keep false until FIX-01 (Gate G1) lands.")
	requireAuth      = flag.Bool("require_auth", false, "Require the auth-proxy headers and the viewer role.")
	uberproxyPEN     = flag.Bool("uberproxy_pen", false, "Serve other Googlers through UberProxy PEN (https://<encoded>.proxy.googlers.com) from a Cloudtop. Needs a non-loopback --listen such as :8090, and doesn't require auth-proxy headers: access is limited to the Googlers that UberProxy lets through. For short demos only.")
	rateLimitQPS     = flag.Float64("rate_limit_qps", 5, "Requests per second allowed per caller.")
	rateLimitBurst   = flag.Int("rate_limit_burst", 20, "Request burst allowed per caller.")
	requestTimeout   = flag.Duration("request_timeout", 15*time.Second, "Timeout of each request, including its database queries.")
)

func main() {
	// common.InitWithMust is not used because it logs all environment variables, which may hold
	// secrets.
	flag.Parse()
	if err := run(); err != nil {
		sklog.Fatal(err)
	}
}

func run() error {
	if *configFilename == "" {
		return skerr.Fmt("--config_filename is required")
	}
	if *visibility != "internal" && *visibility != "public" {
		return skerr.Fmt("--visibility must be internal or public")
	}
	if *rateLimitQPS <= 0 || *rateLimitBurst <= 0 || *requestTimeout <= 0 {
		return skerr.Fmt("--rate_limit_qps, --rate_limit_burst and --request_timeout must be positive")
	}

	cfg, violations, err := validate.InstanceConfigFromFile(*configFilename)
	if err != nil {
		for _, v := range violations {
			sklog.Error(v)
		}
		return skerr.Wrapf(err, "loading the config")
	}
	if *connectionString != "" {
		cfg.DataStoreConfig.ConnectionString = *connectionString
	}
	if !cfg.UseRegression2 {
		return skerr.Fmt("perfinsights needs an instance that uses the Regressions2 schema (use_regression2_schema)")
	}
	if !*allowNonDemoDB {
		if err := localonly.CheckDemoDatabase(cfg.DataStoreConfig.ConnectionString); err != nil {
			return skerr.Wrapf(err, "refusing to start: until FIX-01 (Gate G1) lands, perfinsights only reads the local demo database")
		}
	}
	readOnly, err := readOnlyConnectionString(cfg.DataStoreConfig.ConnectionString)
	if err != nil {
		return err
	}
	cfg.DataStoreConfig.ConnectionString = readOnly
	config.Config = cfg

	hostname, err := os.Hostname()
	if err != nil {
		return skerr.Wrapf(err, "reading the hostname")
	}
	netCfg, err := newNetConfig(*listen, *publicURL, *requireAuth, *uberproxyPEN, hostname)
	if err != nil {
		return err
	}
	if *uberproxyPEN {
		// TODO(security): UberProxy doesn't forward a verifiable identity to Cloudtop servers, so
		// this mode can't tell callers apart: every Googler that UberProxy lets through can read
		// the data, and all their requests share the rate limits of UberProxy's addresses. It
		// relies on a Cloudtop's ports being reachable only through UberProxy. Demos only.
		sklog.Warningf("UberProxy PEN mode: any Googler with the URL can read this instance's data. Share it only with the intended audience, and stop the server after the demo.")
	}
	base := netCfg.PublicURL

	name := *instanceName
	if name == "" {
		name = cfg.InstanceName
	}
	uiURL := strings.TrimRight(*instanceURL, "/")
	if uiURL == "" {
		uiURL = strings.TrimRight(cfg.URL, "/")
	}

	metrics2.InitPrometheus(*promPort)

	svc, err := newService(cfg, name, uiURL)
	if err != nil {
		return err
	}
	s, err := site.New(site.Options{InstanceName: name, InstanceURL: uiURL, Visibility: *visibility, PublicURL: base})
	if err != nil {
		return skerr.Wrapf(err, "rendering the landing page")
	}
	srv := &http.Server{
		Addr: *listen,
		Handler: newHandler(svc, s, serverOptions{
			AllowedHosts:        netCfg.AllowedHosts,
			AllowedHostSuffixes: netCfg.AllowedHostSuffixes,
			AllowedOrigins:      netCfg.AllowedOrigins,
			RequireAuth:         netCfg.RequireAuth,
			RateLimitQPS:        *rateLimitQPS,
			RateLimitBurst:      *rateLimitBurst,
			RequestTimeout:      *requestTimeout,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      *requestTimeout + 15*time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	sklog.Infof("perfinsights serves the Perf instance %q at %s (MCP endpoint: %s/mcp, auth required: %t, UberProxy PEN: %t).", name, base, base, netCfg.RequireAuth, *uberproxyPEN)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return skerr.Wrap(err)
	case sig := <-stop:
		sklog.Infof("Received %s, shutting down.", sig)
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return skerr.Wrap(srv.Shutdown(ctx))
}

// newService creates the insights service on top of the Perf stores. Only read methods are
// reachable through the service's interfaces.
func newService(cfg *config.InstanceConfig, name, uiURL string) (*insights.Service, error) {
	// The builders verify the schema version on startup, and the database pool requires a
	// deadline on every query. The pool doesn't keep this context.
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()
	alertStore, err := builders.NewAlertStoreFromConfig(ctx, cfg)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating the alert store")
	}
	// The alert refresher keeps its context for the life of the server and adds a timeout to each
	// refresh, so it gets a background context.
	alertProvider, err := alerts.NewConfigProvider(context.Background(), alertStore, alertRefreshSeconds)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading the alert configs")
	}
	regressions, err := builders.NewRegressionStoreFromConfig(ctx, cfg, alertProvider)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating the regression store")
	}
	subscriptions, err := builders.NewSubscriptionStoreFromConfig(ctx, cfg)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating the subscription store")
	}
	groups, err := builders.NewAnomalyGroupStoreFromConfig(ctx, cfg)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating the anomaly group store")
	}
	culprits, err := builders.NewCulpritStoreFromConfig(ctx, cfg)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating the culprit store")
	}
	// localToProd=true: never run git updates, which write to the Commits table. The git provider
	// may keep its context (e.g. for token refreshes), so it gets a background context.
	perfGit, err := builders.NewPerfGitFromConfig(context.Background(), true, cfg)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating perfgit")
	}
	// The pool already exists, so this doesn't connect again. The trace reader runs one small,
	// read-only query per request instead of loading the trace store's index of every trace.
	db, err := builders.NewDBPoolFromConfig(ctx, cfg, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting the database pool")
	}
	urls := urlprovider.New(perfGit)
	return insights.New(insights.Stores{
		Regressions:   regressions,
		Subscriptions: subscriptions,
		AnomalyGroups: groups,
		Culprits:      culprits,
		Commits:       perfGit,
		Alerts:        alertProvider,
		Traces:        tracereader.New(db),
	}, insights.Options{
		InstanceName: name,
		InstanceURL:  uiURL,
		Visibility:   *visibility,
		ExploreURL: func(ctx context.Context, start, end int, params map[string][]string) string {
			if uiURL == "" {
				return ""
			}
			return uiURL + urls.Explore(ctx, start, end, params, false, nil)
		},
	})
}
