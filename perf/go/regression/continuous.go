package regression

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
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

// Current state of looking for regressions, i.e. the current commit and alert being worked on.
type Current struct {
	Commit *cid.CommitDetail `json:"commit"`
	Alert  *alerts.Config    `json:"alert"`
}

// Continuous is used to run clustering on the last numCommits commits and
// look for regressions.
type Continuous struct {
	git            *gitinfo.GitInfo
	cidl           *cid.CommitIDLookup
	store          *Store
	numCommits     int // Number of recent commits to do clustering over.
	radius         int
	provider       ConfigProvider
	notifier       *notify.Notifier
	paramsProvider ParamsetProvider
	dfBuilder      dataframe.DataFrameBuilder

	mutex   sync.Mutex // Protects current.
	current *Current
}

// NewContinuous creates a new *Continuous.
//
//   provider - Produces the slice of alerts.Config's that determine the clustering to perform.
//   numCommits - The number of commits to run the clustering over.
//   radius - The number of commits on each side of a commit to include when clustering.
func NewContinuous(git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, provider ConfigProvider, store *Store, numCommits int, radius int, notifier *notify.Notifier, paramsProvider ParamsetProvider, dfBuilder dataframe.DataFrameBuilder) *Continuous {
	return &Continuous{
		git:            git,
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

func (c *Continuous) reportRegressions(ctx context.Context, resps []*ClusterResponse, cfg *alerts.Config) {
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
			// Update database if regression at the midpoint is found.
			if cl.StepPoint.Offset == midOffset {
				if cl.StepFit.Status == stepfit.LOW && len(cl.Keys) >= cfg.MinimumNum && (cfg.Direction == alerts.DOWN || cfg.Direction == alerts.BOTH) {
					sklog.Infof("Found Low regression at %s: %v", details[0].Message, *cl.StepFit)
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
					sklog.Infof("Found High regression at %s: %v", id.ID(), *cl.StepFit)
					isNew, err := c.store.SetHigh(details[0], key, resp.Frame, cl)
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
			}
		}
	}
}

// Run starts the continuous running of clustering over the last numCommits
// commits.
//
// Note that it never returns so it should be called as a Go routine.
func (c *Continuous) Run(ctx context.Context) {
	newClustersGauge := metrics2.GetInt64Metric("perf_clustering_untriaged", nil)
	runsCounter := metrics2.GetCounter("perf_clustering_runs", nil)
	clusteringLatency := metrics2.NewTimer("perf_clustering_latency", nil)

	// TODO(jcgregorio) Add liveness metrics.
	sklog.Infof("Continuous starting.")
	c.reportUntriaged(newClustersGauge)
	for range time.Tick(time.Second) {
		clusteringLatency.Start()
		configs, err := c.provider()
		if err != nil {
			// TODO(jcgregorio) Float these errors up to the UI.
			sklog.Errorf("Failed to load configs: %s", err)
			continue
		}
		for _, cfg := range configs {
			c.mutex.Lock()
			c.current.Alert = cfg
			c.mutex.Unlock()

			clusterResponseProcessor := func(resps []*ClusterResponse) {
				c.reportRegressions(ctx, resps, cfg)
			}
			if cfg.Radius == 0 {
				cfg.Radius = c.radius
			}
			RegressionsForAlert(ctx, cfg, c.paramsProvider(), clusterResponseProcessor, c.numCommits, time.Now(), c.git, c.cidl, c.dfBuilder)
		}
		clusteringLatency.Stop()
		runsCounter.Inc(1)
	}
}
