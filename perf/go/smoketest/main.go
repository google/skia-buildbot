// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// perf-smoketest runs HTTP smoke tests against deployed perfserver frontends
// and gates promotion of the latest perfserver build on the resulting verdicts.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
)

const usage = `usage: perf-smoketest <command> [flags]

Commands:
  run    Run smoke checks against perfserver frontends and write verdicts.
  loop   Continuously run smoke checks on an interval and export metrics.
  gate   Evaluate smoke test verdicts for the latest perfserver build.
`

const (
	defaultVersionPollInterval = 30 * time.Second
	defaultFramePollInterval   = time.Second
	maxFrameWindow             = 24 * time.Hour
)

var (
	targetNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)
	defaultChecks     = []string{
		checkVersion,
		checkHealth,
		checkTemplates,
		checkParamset,
		checkHistoricalQuery,
		checkRegCount,
	}
	defaultPages          = []string{"/m/", "/e/", "/e2/", "/a/", "/u/"}
	defaultRequiredParams = []string{"benchmark", "bot", "test"}
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	// Block AtExit until runCLI returns because go/cleanup calls os.Exit
	// immediately after AtExit callbacks finish.
	cleanup.AtExit(func() {
		cancel()
		<-done
	})
	cleanup.Enable()

	code := runCLI(ctx, os.Args[1:], os.Stdout, os.Stderr)
	close(done)
	if ctx.Err() != nil {
		// Park so go/cleanup's signal goroutine owns process termination.
		select {}
	}
	os.Exit(code)
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runCLIWithDeps(ctx, args, stdout, stderr, nil)
}

func runCLIWithDeps(ctx context.Context, args []string, stdout, stderr io.Writer, gcsOverride gcs.GCSClient) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "run":
		return runCmd(ctx, args[1:], stdout, stderr, gcsOverride)
	case "loop":
		return loopCmd(ctx, args[1:], stdout, stderr, gcsOverride)
	case "gate":
		return gateCmd(ctx, args[1:], stdout, stderr, gcsOverride)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

type runConfig struct {
	checkNames          []string
	expectedGitHash     string
	stagesFile          string
	pages               []string
	requiredParams      []string
	healthPath          string
	frameQuery          string
	allTargetsHaveQuery bool
	frameBegin          string
	frameEnd            string
	rolloutTimeout      time.Duration
	timeout             time.Duration
}

func runCmd(ctx context.Context, args []string, stdout, stderr io.Writer, gcsOverride gcs.GCSClient) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var targetSpecs []string
	fs.Func("target", "Target frontend in name=url[,host=...][,query=...] form (repeatable).", func(v string) error {
		targetSpecs = append(targetSpecs, v)
		return nil
	})
	expectedGitHash := fs.String("expected_git_hash", "", "Expected window.perf.app_version git hash.")
	stagesFile := fs.String("stages_file", "", "Path to stages.json to read latest perfserver git_hash.")
	checkNames := common.FSNewMultiStringFlag(fs, "checks", defaultChecks, "Ordered allowlist of checks to execute.")
	pages := common.FSNewMultiStringFlag(fs, "pages", defaultPages, "Page paths for the templates check.")
	requiredParams := common.FSNewMultiStringFlag(fs, "required_params", defaultRequiredParams, "Required keys for the paramset check.")
	healthPath := fs.String("health_path", "/healthz", "Endpoint path for the health check.")
	frameQuery := fs.String("frame_query", "", "URL query string for historical_query.")
	frameBegin := fs.String("frame_begin", "", "RFC 3339 start timestamp for historical_query.")
	frameEnd := fs.String("frame_end", "", "RFC 3339 end timestamp for historical_query.")
	hostHeader := fs.String("host_header", "", "Host header override applied to every request.")
	forwardedHTTPS := fs.Bool("forwarded_proto_https", false, "Set X-Forwarded-Proto: https on every request.")
	rolloutTimeout := fs.Duration("rollout_timeout", 0, "Retry window for the version check (0s means one attempt).")
	timeout := fs.Duration("timeout", 60*time.Second, "Deadline per check.")
	verdictDir := fs.String("verdict_dir", "", "Directory or gs:// URI where <instance>.json verdicts are written.")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *verdictDir == "" {
		_, _ = fmt.Fprintln(stderr, "error: --verdict_dir is required")
		return 2
	}
	targets, err := parseTargets(targetSpecs, *hostHeader, *forwardedHTTPS)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	checks, err := buildChecks(runConfig{
		checkNames:          *checkNames,
		expectedGitHash:     *expectedGitHash,
		stagesFile:          *stagesFile,
		pages:               *pages,
		requiredParams:      *requiredParams,
		healthPath:          *healthPath,
		frameQuery:          *frameQuery,
		allTargetsHaveQuery: allTargetsHaveQuery(targets),
		frameBegin:          *frameBegin,
		frameEnd:            *frameEnd,
		rolloutTimeout:      *rolloutTimeout,
		timeout:             *timeout,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	store, err := newVerdictStore(ctx, *verdictDir, false, gcsOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}

	allPassed, err := executeTargets(ctx, targets, checks, store, stdout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	if !allPassed {
		return 1
	}
	return 0
}

func loopCmd(ctx context.Context, args []string, stdout, stderr io.Writer, gcsOverride gcs.GCSClient) int {
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var targetSpecs []string
	fs.Func("target", "Target frontend in name=url[,host=...][,query=...] form (repeatable).", func(v string) error {
		targetSpecs = append(targetSpecs, v)
		return nil
	})
	stagesURL := fs.String("stages_url", "", "Gitiles URL to stages.json to read latest perfserver git_hash.")
	stagesFile := fs.String("stages_file", "", "Local path to stages.json to read latest perfserver git_hash.")
	interval := fs.Duration("interval", 3*time.Minute, "Interval between smoke check runs.")
	iterations := fs.Int("iterations", 0, "Maximum number of loop iterations to run (0 means run until canceled).")
	promPort := fs.String("prom_port", ":20000", "Prometheus metrics listen address (empty disables HTTP server).")
	checkNames := common.FSNewMultiStringFlag(fs, "checks", defaultChecks, "Ordered allowlist of checks to execute.")
	pages := common.FSNewMultiStringFlag(fs, "pages", defaultPages, "Page paths for the templates check.")
	requiredParams := common.FSNewMultiStringFlag(fs, "required_params", defaultRequiredParams, "Required keys for the paramset check.")
	healthPath := fs.String("health_path", "/healthz", "Endpoint path for the health check.")
	frameQuery := fs.String("frame_query", "", "Default URL query string for historical_query.")
	frameBegin := fs.String("frame_begin", "", "RFC 3339 start timestamp for historical_query.")
	frameEnd := fs.String("frame_end", "", "RFC 3339 end timestamp for historical_query.")
	hostHeader := fs.String("host_header", "", "Host header override applied to every request.")
	forwardedHTTPS := fs.Bool("forwarded_proto_https", false, "Set X-Forwarded-Proto: https on every request.")
	timeout := fs.Duration("timeout", 60*time.Second, "Deadline per check.")
	verdictDir := fs.String("verdict_dir", "", "Directory or gs:// URI where <instance>.json verdicts are written.")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *verdictDir == "" {
		_, _ = fmt.Fprintln(stderr, "error: --verdict_dir is required")
		return 2
	}
	if *interval <= 0 {
		_, _ = fmt.Fprintln(stderr, "error: --interval must be > 0")
		return 2
	}
	if *iterations < 0 {
		_, _ = fmt.Fprintln(stderr, "error: --iterations must be >= 0")
		return 2
	}
	if (*stagesURL == "") == (*stagesFile == "") {
		_, _ = fmt.Fprintln(stderr, "error: loop requires exactly one of --stages_url or --stages_file")
		return 2
	}
	targets, err := parseTargets(targetSpecs, *hostHeader, *forwardedHTTPS)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	baseCfg := runConfig{
		checkNames:          *checkNames,
		expectedGitHash:     "placeholder-hash",
		pages:               *pages,
		requiredParams:      *requiredParams,
		healthPath:          *healthPath,
		frameQuery:          *frameQuery,
		allTargetsHaveQuery: allTargetsHaveQuery(targets),
		frameBegin:          *frameBegin,
		frameEnd:            *frameEnd,
		rolloutTimeout:      0,
		timeout:             *timeout,
	}
	if _, err := buildChecks(baseCfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	httpClient, err := newGitilesHTTPClient(ctx, *stagesURL, *timeout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	store, err := newVerdictStore(ctx, *verdictDir, false, gcsOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	if *promPort != "" {
		metrics2.InitPrometheus(*promPort)
	}

	for iter := 1; ; iter++ {
		if ctx.Err() != nil {
			return 0
		}
		var expected string
		if *stagesURL != "" {
			expected, err = fetchLatestPerfserverHash(ctx, httpClient, *stagesURL)
		} else {
			expected, err = latestPerfserverHash(*stagesFile)
		}
		allPassed := false
		if err != nil {
			if ctx.Err() != nil {
				return 0
			}
			_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		} else {
			cfg := baseCfg
			cfg.expectedGitHash = expected
			checks, buildErr := buildChecks(cfg)
			if buildErr != nil {
				_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(buildErr))
			} else {
				allPassed, err = executeTargets(ctx, targets, checks, store, stdout)
				if err != nil {
					if ctx.Err() != nil {
						return 0
					}
					_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
				}
			}
		}
		if *iterations > 0 && iter >= *iterations {
			if !allPassed {
				return 1
			}
			return 0
		}
		if ctx.Err() != nil {
			return 0
		}
		timer := time.NewTimer(*interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0
		case <-timer.C:
		}
	}
}

func newGitilesHTTPClient(ctx context.Context, stagesURL string, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if stagesURL == "" {
		return &http.Client{Timeout: timeout}, nil
	}
	u, err := url.Parse(stagesURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, skerr.Fmt("invalid stages_url %q", stagesURL)
	}
	if !strings.HasSuffix(u.Hostname(), ".googlesource.com") {
		return &http.Client{Timeout: timeout}, nil
	}
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting Gerrit token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	c.Timeout = timeout
	return c, nil
}

func executeTargets(ctx context.Context, targets []*target, checks []Check, store *verdictStore, stdout io.Writer) (bool, error) {
	allPassed := true
	for _, tgt := range targets {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		prev, err := store.read(ctx, tgt.name)
		if err != nil {
			return false, err
		}
		startedAt := time.Now()
		results := runChecks(ctx, tgt, checks)
		if err := ctx.Err(); err != nil {
			return false, err
		}
		finishedAt := time.Now()
		v := newVerdict(tgt.name, tgt.baseURL, tgt.appVersion, startedAt, finishedAt, results, prev)
		printRunTable(stdout, v)
		if err := store.write(ctx, v); err != nil {
			return false, err
		}
		recordMetrics(v)
		if v.Status != statusPass {
			allPassed = false
		}
	}
	return allPassed, nil
}

func recordMetrics(v Verdict) {
	instanceTags := map[string]string{"instance": v.Instance}
	success := int64(0)
	if v.Status == statusPass {
		success = 1
	}
	metrics2.GetInt64Metric("perf_smoketest_success", instanceTags).Update(success)
	metrics2.GetInt64Metric("perf_smoketest_last_run_timestamp", instanceTags).Update(v.FinishedAt.Unix())
	for _, c := range v.Checks {
		if c.Status == statusSkipped {
			continue
		}
		checkSuccess := int64(0)
		if c.Status == statusPass {
			checkSuccess = 1
		}
		metrics2.GetInt64Metric("perf_smoketest_check_success", map[string]string{
			"instance": v.Instance,
			"check":    c.Name,
		}).Update(checkSuccess)
	}
}

func printRunTable(w io.Writer, v Verdict) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INSTANCE\tCHECK\tSTATUS\tLATENCY_MS\tERROR")
	for _, c := range v.Checks {
		errText := c.Error
		if errText == "" {
			errText = "-"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", v.Instance, c.Name, c.Status, c.LatencyMS, errText)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "VERDICT: instance=%s status=%s app_version=%s consecutive_passes=%d\n",
		v.Instance, v.Status, shortHash(v.AppVersion), v.ConsecutivePasses)
}

func gateCmd(ctx context.Context, args []string, stdout, stderr io.Writer, gcsOverride gcs.GCSClient) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stagesFile := fs.String("stages_file", "", "Path to stages.json.")
	verdictDir := fs.String("verdict_dir", "", "Directory or gs:// URI containing <instance>.json verdicts.")
	instances := common.FSNewMultiStringFlag(fs, "instances", nil, "Comma-separated instance names that must pass.")
	minPasses := fs.Int("min_consecutive_passes", 3, "Minimum consecutive_passes required per instance.")
	maxAge := fs.Duration("max_age", 15*time.Minute, "Maximum age of finished_at.")
	reportFile := fs.String("report_file", "", "Optional path to write the smoke test gate report.")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *stagesFile == "" || *verdictDir == "" || len(*instances) == 0 {
		_, _ = fmt.Fprintln(stderr, "error: --stages_file, --verdict_dir, and --instances are required")
		return 2
	}
	if *minPasses < 1 || *maxAge <= 0 {
		_, _ = fmt.Fprintln(stderr, "error: --min_consecutive_passes must be >= 1 and --max_age must be > 0")
		return 2
	}
	for _, inst := range *instances {
		if !targetNamePattern.MatchString(inst) {
			_, _ = fmt.Fprintf(stderr, "error: instance name %q must match ^[a-z0-9-]+$\n", inst)
			return 2
		}
	}
	sf, err := latestStageFile(*stagesFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	latest, err := latestPerfserverHashFromStageFile(sf, *stagesFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	store, err := newVerdictStore(ctx, *verdictDir, true, gcsOverride)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
		return 2
	}
	verdicts := make(map[string]*Verdict, len(*instances))
	for _, inst := range *instances {
		v, err := store.read(ctx, inst)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "error: %s\n", errorMessage(err))
			return 2
		}
		if v != nil {
			verdicts[inst] = v
		}
	}
	decisions, overallPass := evaluateGate(latest, *instances, verdicts, *minPasses, *maxAge, time.Now())
	printGateTable(stdout, latest, decisions, overallPass)
	if *reportFile != "" {
		report := formatGateReport(sf, latest, decisions, overallPass)
		if err := os.WriteFile(*reportFile, []byte(report), 0o644); err != nil {
			_, _ = fmt.Fprintf(stderr, "error: writing report_file %s: %s\n", *reportFile, errorMessage(err))
			return 2
		}
	}
	if !overallPass {
		return 1
	}
	return 0
}

// parseTargets validates target names at the CLI boundary because each target
// name becomes the verdict file name on disk or in GCS.
func parseTargets(specs []string, hostHeader string, forwardedHTTPS bool) ([]*target, error) {
	if len(specs) == 0 {
		return nil, skerr.Fmt("--target is required")
	}
	seen := make(map[string]bool, len(specs))
	targets := make([]*target, 0, len(specs))
	for _, spec := range specs {
		name, rest, ok := strings.Cut(spec, "=")
		if !ok {
			return nil, skerr.Fmt("target %q must have the form name=url[,host=...][,query=...]", spec)
		}
		if !targetNamePattern.MatchString(name) {
			return nil, skerr.Fmt("target name %q must match ^[a-z0-9-]+$", name)
		}
		if seen[name] {
			return nil, skerr.Fmt("duplicate target name %q", name)
		}
		seen[name] = true
		rawURL, frameQuery, hasQuery := strings.Cut(rest, ",query=")
		rawURL, targetHost, hasHost := strings.Cut(rawURL, ",host=")
		if !hasHost && hasQuery {
			frameQuery, targetHost, hasHost = strings.Cut(frameQuery, ",host=")
		}
		if hasQuery && frameQuery == "" {
			return nil, skerr.Fmt("target %q has empty query", spec)
		}
		if hasHost && targetHost == "" {
			return nil, skerr.Fmt("target %q has empty host", spec)
		}
		if !hasHost {
			targetHost = hostHeader
		}
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, skerr.Fmt("invalid target url %q", rawURL)
		}
		targets = append(targets, &target{
			name:           name,
			baseURL:        strings.TrimRight(rawURL, "/"),
			frameQuery:     frameQuery,
			hostHeader:     targetHost,
			forwardedHTTPS: forwardedHTTPS,
			client:         newHTTPClient(),
		})
	}
	return targets, nil
}

func allTargetsHaveQuery(targets []*target) bool {
	if len(targets) == 0 {
		return false
	}
	for _, t := range targets {
		if t.frameQuery == "" {
			return false
		}
	}
	return true
}

// parseFrameWindow caps the historical_query range at 24h to enforce the live
// guardrail against wide Spanner scans on autopush.
func parseFrameWindow(beginStr, endStr string) (time.Time, time.Time, error) {
	if beginStr == "" || endStr == "" {
		return time.Time{}, time.Time{}, skerr.Fmt("--frame_begin and --frame_end are required for historical_query")
	}
	begin, err := time.Parse(time.RFC3339, beginStr)
	if err != nil {
		return time.Time{}, time.Time{}, skerr.Wrapf(err, "parsing --frame_begin %q", beginStr)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return time.Time{}, time.Time{}, skerr.Wrapf(err, "parsing --frame_end %q", endStr)
	}
	if !end.After(begin) {
		return time.Time{}, time.Time{}, skerr.Fmt("--frame_end (%s) must be after --frame_begin (%s)", endStr, beginStr)
	}
	if end.Sub(begin) > maxFrameWindow {
		return time.Time{}, time.Time{}, skerr.Fmt("frame window %s exceeds %s maximum", end.Sub(begin), maxFrameWindow)
	}
	return begin, end, nil
}

func buildChecks(cfg runConfig) ([]Check, error) {
	if len(cfg.checkNames) == 0 {
		return nil, skerr.Fmt("--checks must specify at least one check")
	}
	if cfg.rolloutTimeout < 0 {
		return nil, skerr.Fmt("--rollout_timeout must be >= 0")
	}
	timeout := cfg.timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	checks := make([]Check, 0, len(cfg.checkNames))
	for _, name := range cfg.checkNames {
		var c Check
		switch name {
		case checkVersion:
			if (cfg.expectedGitHash == "") == (cfg.stagesFile == "") {
				return nil, skerr.Fmt("version check requires exactly one of --expected_git_hash or --stages_file")
			}
			expected := cfg.expectedGitHash
			if cfg.stagesFile != "" {
				var err error
				expected, err = latestPerfserverHash(cfg.stagesFile)
				if err != nil {
					return nil, err
				}
			}
			c = versionCheck(expected, cfg.rolloutTimeout, defaultVersionPollInterval)
			c.Timeout = timeout + cfg.rolloutTimeout
		case checkHealth:
			if !strings.HasPrefix(cfg.healthPath, "/") {
				return nil, skerr.Fmt("--health_path %q must start with /", cfg.healthPath)
			}
			c = healthCheck(cfg.healthPath)
			c.Timeout = timeout
		case checkTemplates:
			if len(cfg.pages) == 0 {
				return nil, skerr.Fmt("--pages must specify at least one page for templates")
			}
			c = templatesCheck(cfg.pages)
			c.Timeout = timeout
		case checkParamset:
			if len(cfg.requiredParams) == 0 {
				return nil, skerr.Fmt("--required_params must specify at least one key for paramset")
			}
			c = paramsetCheck(cfg.requiredParams)
			c.Timeout = timeout
		case checkHistoricalQuery:
			if cfg.frameQuery == "" && !cfg.allTargetsHaveQuery {
				return nil, skerr.Fmt("--frame_query is required for historical_query (or set ,query=... on every --target)")
			}
			begin, end, err := parseFrameWindow(cfg.frameBegin, cfg.frameEnd)
			if err != nil {
				return nil, err
			}
			c = historicalQueryCheck(cfg.frameQuery, begin, end, defaultFramePollInterval)
			c.Timeout = timeout
		case checkRegCount:
			c = regCountCheck()
			c.Timeout = timeout
		case checkSheriffList:
			c = sheriffListCheck()
			c.Timeout = timeout
		default:
			return nil, skerr.Fmt("unknown check %q", name)
		}
		checks = append(checks, c)
	}
	return checks, nil
}
