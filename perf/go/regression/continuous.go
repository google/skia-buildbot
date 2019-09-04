package regression

import (
	"context"
	"math/rand"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/stepfit"
)

// ConfigProvider is a function that's called to return a slice of alerts.Config. It is passed to NewContinuous.
type ConfigProvider func() ([]*alerts.Config, error)

// ParamsetProvider is a function that's called to return the current paramset. It is passed to NewContinuous.
type ParamsetProvider func() paramtools.ParamSet

// StepProvider if a func that's called to return the current step within a config we're clustering.
type StepProvider func(step, total int)

// Current state of looking for regressions, i.e. the current commit and alert being worked on.
type Current struct {
	Commit *cid.CommitDetail `json:"commit"`
	Alert  *alerts.Config    `json:"alert"`
	Step   int               `json:"step"`
	Total  int               `json:"total"`
}

// Continuous is used to run clustering on the last numCommits commits and
// look for regressions.
type Continuous struct {
	vcs                    vcsinfo.VCS
	cidl                   *cid.CommitIDLookup
	store                  *Store
	numCommits             int // Number of recent commits to do clustering over.
	radius                 int
	eventDriven            bool
	pubSubSubscriptionName string
	provider               ConfigProvider
	notifier               *notify.Notifier
	paramsProvider         ParamsetProvider
	dfBuilder              dataframe.DataFrameBuilder

	mutex   sync.Mutex // Protects current.
	current *Current
}

// NewContinuous creates a new *Continuous.
//
//   provider - Produces the slice of alerts.Config's that determine the clustering to perform.
//   numCommits - The number of commits to run the clustering over.
//   radius - The number of commits on each side of a commit to include when clustering.
func NewContinuous(vcs vcsinfo.VCS, cidl *cid.CommitIDLookup, provider ConfigProvider, store *Store, numCommits int, radius int, notifier *notify.Notifier, paramsProvider ParamsetProvider, dfBuilder dataframe.DataFrameBuilder) *Continuous {
	return &Continuous{
		vcs:            vcs,
		cidl:           cidl,
		store:          store,
		numCommits:     numCommits,
		radius:         radius,
		provider:       provider,
		notifier:       notifier,
		current:        &Current{},
		paramsProvider: paramsProvider,
		dfBuilder:      dfBuilder,
	}
}

func (c *Continuous) CurrentStatus() Current {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return *c.current
}

func (c *Continuous) Untriaged() (int, error) {
	return c.store.Untriaged()
}

func (c *Continuous) reportUntriaged(newClustersGauge metrics2.Int64Metric) {
	go func() {
		for range time.Tick(time.Minute) {
			if count, err := c.store.Untriaged(); err == nil {
				newClustersGauge.Update(int64(count))
			} else {
				sklog.Errorf("Failed to get untriaged count: %s", err)
			}
		}
	}()
}

func (c *Continuous) reportRegressions(ctx context.Context, req *ClusterRequest, resps []*ClusterResponse, cfg *alerts.Config) {
	key := cfg.IdAsString()
	for _, resp := range resps {
		headerLength := len(resp.Frame.DataFrame.Header)
		midPoint := headerLength / 2

		midOffset := resp.Frame.DataFrame.Header[midPoint].Offset

		id := &cid.CommitID{
			Source: "master",
			Offset: int(midOffset),
		}

		details, err := c.cidl.Lookup(ctx, []*cid.CommitID{id})
		if err != nil {
			sklog.Errorf("Failed to look up commit %v: %s", *id, err)
			continue
		}
		for _, cl := range resp.Summary.Clusters {
			// Zero out the DataFrame ParamSet since it is never used.
			resp.Frame.DataFrame.ParamSet = paramtools.ParamSet{}
			// Update database if regression at the midpoint is found.
			if cl.StepPoint.Offset == midOffset {
				if cl.StepFit.Status == stepfit.LOW && len(cl.Keys) >= cfg.MinimumNum && (cfg.Direction == alerts.DOWN || cfg.Direction == alerts.BOTH) {
					sklog.Infof("Found Low regression at %s: StepFit: %v Shortcut: %s AlertID: %d %d req: %#v", details[0].Message, *cl.StepFit, cl.Shortcut, cfg.ID, c.current.Alert.ID, *req)
					isNew, err := c.store.SetLow(details[0], key, resp.Frame, cl)
					if err != nil {
						sklog.Errorf("Failed to save newly found cluster: %s", err)
						continue
					}
					if isNew {
						if err := c.notifier.Send(details[0], cfg, cl); err != nil {
							sklog.Errorf("Failed to send notification: %s", err)
						}
					}
				}
				if cl.StepFit.Status == stepfit.HIGH && len(cl.Keys) >= cfg.MinimumNum && (cfg.Direction == alerts.UP || cfg.Direction == alerts.BOTH) {
					sklog.Infof("Found High regression at %s: StepFit: %v Shortcut: %s AlertID: %d %d req: %#v", details[0].Message, *cl.StepFit, cl.Shortcut, cfg.ID, c.current.Alert.ID, *req)
					isNew, err := c.store.SetHigh(details[0], key, resp.Frame, cl)
					if err != nil {
						sklog.Errorf("Failed to save newly found cluster for alert %q length=%d: %s", key, len(cl.Keys), err)
						continue
					}
					if isNew {
						if err := c.notifier.Send(details[0], cfg, cl); err != nil {
							sklog.Errorf("Failed to send notification: %s", err)
						}
					}
				}
			}
		}
	}
}

func (c *Continuous) setCurrentConfig(cfg *alerts.Config) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.current.Alert = cfg
}

func (c *Continuous) setCurrentStep(step, total int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.current.Step = step
	c.current.Total = total
}

// Run starts the continuous running of clustering over the last numCommits
// commits.
//
// Note that it never returns so it should be called as a Go routine.
func (c *Continuous) Run(ctx context.Context) {
	newClustersGauge := metrics2.GetInt64Metric("perf_clustering_untriaged", nil)
	runsCounter := metrics2.GetCounter("perf_clustering_runs", nil)
	clusteringLatency := metrics2.NewTimer("perf_clustering_latency", nil)
	configsCounter := metrics2.GetCounter("perf_clustering_configs", nil)

	// TODO(jcgregorio) Add liveness metrics.
	sklog.Infof("Continuous starting.")
	c.reportUntriaged(newClustersGauge)

	// Instead of ranging over time, we should be ranging over PubSub events
	// that list the ids of the last file that was ingested. Then we should loop
	// over each config and see if that list of trace ids matches any configs,
	// and if so at that point we start running the regresions. But we also want
	// to preserve continuous regression detection for the cases where it makes
	// sense, e.g. Skia.
	//
	// So we can actually range over a channel here that supplies a slice of
	// configs and a paramset representing all the traceids we should be running
	// over. If this is just a timer then the paramset is the full paramset and
	// the slice of configs is just the full slice of configs. If it is PubSub
	// driven then the paramset is built from the list of trace ids we received
	// and the list of configs is built by matching the full list of configs
	// against the list of incoming trace ids.
	//
	for range time.Tick(time.Second) {
		clusteringLatency.Start()
		configs, err := c.provider()

		// Shuffle the order of the configs.
		rand.Shuffle(len(configs), func(i, j int) {
			configs[i], configs[j] = configs[j], configs[i]
		})

		if err != nil {
			// TODO(jcgregorio) Float these errors up to the UI.
			sklog.Errorf("Failed to load configs: %s", err)
			continue
		}
		sklog.Infof("Clustering over %d configs.", len(configs))
		for _, cfg := range configs {
			c.setCurrentConfig(cfg)

			// Smoketest the query, but only if we are not in event driven mode.
			if cfg.GroupBy != "" && !c.eventDriven {
				sklog.Infof("Alert contains a GroupBy, doing a smoketest first: %q", cfg.DisplayName)
				u, err := url.ParseQuery(cfg.Query)
				if err != nil {
					sklog.Warningf("Alert failed smoketest: Alert contains invalid query: %q: %s", cfg.Query, err)
					continue
				}
				q, err := query.New(u)
				if err != nil {
					sklog.Warningf("Alert failed smoketest: Alert contains invalid query: %q: %s", cfg.Query, err)
					continue
				}
				// Should be changed to PreflightQuery.
				df, err := c.dfBuilder.NewNFromQuery(context.Background(), time.Time{}, q, 20, nil)
				if err != nil {
					sklog.Warningf("Alert failed smoketest: %q Failed while trying generic query: %s", cfg.DisplayName, err)
					continue
				}
				if len(df.TraceSet) == 0 {
					sklog.Warningf("Alert failed smoketest: %q Failed to get any traces for generic query.", cfg.DisplayName)
					continue
				}
				sklog.Infof("Alert %q passed smoketest.", cfg.DisplayName)
			}

			clusterResponseProcessor := func(req *ClusterRequest, resps []*ClusterResponse) {
				c.reportRegressions(ctx, req, resps, cfg)
			}
			if cfg.Radius == 0 {
				cfg.Radius = c.radius
			}
			RegressionsForAlert(ctx, cfg, c.paramsProvider(), clusterResponseProcessor, c.numCommits, time.Time{}, c.vcs, c.cidl, c.dfBuilder, c.setCurrentStep)
			configsCounter.Inc(1)
		}
		clusteringLatency.Stop()
		runsCounter.Inc(1)
		configsCounter.Reset()
	}
}
