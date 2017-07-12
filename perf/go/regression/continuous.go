package regression

import (
	"sync"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/stepfit"
)

// ConfigProvider is a function that's called to return a slice of alerts.Config. It is passed to NewContinuous.
type ConfigProvider func() ([]*alerts.Config, error)

// Current state of looking for regressions, i.e. the current commit and alert being worked on.
type Current struct {
	Commit *cid.CommitDetail `json:"commit"`
	Alert  *alerts.Config    `json:"alert"`
}

// Continuous is used to run clustering on the last numCommits commits and
// look for regressions.
type Continuous struct {
	git        *gitinfo.GitInfo
	cidl       *cid.CommitIDLookup
	store      *Store
	numCommits int // Number of recent commits to do clustering over.
	radius     int
	provider   ConfigProvider
	notifier   *notify.Notifier
	useID      bool

	mutex   sync.Mutex // Protects current.
	current *Current
}

// NewContinuous creates a new *Continuous.
//
//   provider - Produces the slice of alerts.Config's that determine the clustering to perform.
//   numCommits - The number of commits to run the clustering over.
//   radius - The number of commits on each side of a commit to include when clustering.
func NewContinuous(git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, provider ConfigProvider, store *Store, numCommits int, radius int, notifier *notify.Notifier, useID bool) *Continuous {
	return &Continuous{
		git:        git,
		cidl:       cidl,
		store:      store,
		numCommits: numCommits,
		radius:     radius,
		provider:   provider,
		notifier:   notifier,
		useID:      useID,
		current:    &Current{},
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

// Run starts the continuous running of clustering over the last numCommits
// commits.
//
// Note that it never returns so it should be called as a Go routine.
func (c *Continuous) Run() {
	newClustersGauge := metrics2.GetInt64Metric("perf.clustering.untriaged", nil)
	runsCounter := metrics2.GetCounter("perf.clustering.runs", nil)
	clusteringLatency := metrics2.NewTimer("perf.clustering.latency", nil)

	// TODO(jcgregorio) Add liveness metrics.
	sklog.Infof("Continuous starting.")
	c.reportUntriaged(newClustersGauge)
	for range time.Tick(time.Minute) {
		clusteringLatency.Start()
		// Get the last numCommits commits.
		indexCommits := c.git.LastNIndex(c.numCommits)
		// Drop the radius most recent, since we are clustering
		// based on a radius of +/-radius commits.
		indexCommits = indexCommits[:(c.numCommits - c.radius)]
		for _, commit := range indexCommits {
			id := &cid.CommitID{
				Source: "master",
				Offset: commit.Index,
			}
			details, err := c.cidl.Lookup([]*cid.CommitID{id})
			if err != nil {
				sklog.Errorf("Failed to look up commit %v: %s", *id, err)
				continue
			}
			configs, err := c.provider()
			if err != nil {
				sklog.Errorf("Failed to load configs: %s", err)
				continue
			}
			c.mutex.Lock()
			c.current.Commit = details[0]
			c.mutex.Unlock()
			for _, cfg := range configs {
				c.mutex.Lock()
				c.current.Alert = cfg
				c.mutex.Unlock()
				// Create ClusterRequest and run.
				req := &clustering2.ClusterRequest{
					Source:      "master",
					Offset:      commit.Index,
					Radius:      c.radius,
					Query:       cfg.Query,
					Algo:        cfg.Algo,
					Interesting: cfg.Interesting,
				}
				sklog.Infof("Continuous: Clustering at %s for %q", details[0].Message, cfg.Query)
				resp, err := clustering2.Run(req, c.git, c.cidl)
				if err != nil {
					sklog.Errorf("Failed while clustering %v %s", *req, err)
					continue
				}

				key := cfg.Query
				if c.useID {
					key = cfg.IdAsString()
				}
				// Update database if regression at the midpoint is found.
				for _, cl := range resp.Summary.Clusters {
					if cl.StepPoint.Offset == int64(commit.Index) {
						if cl.StepFit.Status == stepfit.LOW && !cfg.StepUpOnly {
							sklog.Infof("Found Low regression at %s for %q: %v", details[0].Message, cfg.Query, *cl.StepFit)
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
						if cl.StepFit.Status == stepfit.HIGH {
							sklog.Infof("Found High regression at %s for %q: %v", id.ID(), cfg.Query, *cl.StepFit)
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
		clusteringLatency.Stop()
		runsCounter.Inc(1)
	}
}
