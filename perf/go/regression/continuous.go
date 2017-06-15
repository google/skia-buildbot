package regression

import (
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/stepfit"
)

// Continuous is used to run clustering on the last numCommits commits and
// look for regressions.
type Continuous struct {
	git         *gitinfo.GitInfo
	cidl        *cid.CommitIDLookup
	queries     []string
	store       *Store
	numCommits  int // Number of recent commits to do clustering over.
	radius      int
	interesting float32
	algo        clustering2.ClusterAlgo
}

// NewContinuous creates a new *Continuous.
//
// queries is a slice of URL query encoded to perform against the datastore to
// determine which traces participate in clustering.
func NewContinuous(git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, queries []string, store *Store, numCommits int, radius int, interesting float32, algo clustering2.ClusterAlgo) *Continuous {
	return &Continuous{
		git:         git,
		cidl:        cidl,
		queries:     queries,
		store:       store,
		numCommits:  numCommits,
		radius:      radius,
		interesting: interesting,
		algo:        algo,
	}
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
			for _, q := range c.queries {
				// Create ClusterRequest and run.
				req := &clustering2.ClusterRequest{
					Source: "master",
					Offset: commit.Index,
					Radius: c.radius,
					Query:  q,
					Algo:   c.algo,
				}
				sklog.Infof("Continuous: Clustering at %s for %q", details[0].Message, q)
				resp, err := clustering2.Run(req, c.git, c.cidl, c.interesting)
				if err != nil {
					sklog.Errorf("Failed while clustering %v %s", *req, err)
					continue
				}
				// Update database if regression at the midpoint is found.
				for _, cl := range resp.Summary.Clusters {
					if cl.StepPoint.Offset == int64(commit.Index) {
						if cl.StepFit.Status == stepfit.LOW {
							if err := c.store.SetLow(details[0], q, resp.Frame, cl); err != nil {
								sklog.Errorf("Failed to save newly found cluster: %s", err)
							}
							sklog.Infof("Found Low regression at %s for %q: %v", details[0].Message, q, *cl.StepFit)
						}
						if cl.StepFit.Status == stepfit.HIGH {
							if err := c.store.SetHigh(details[0], q, resp.Frame, cl); err != nil {
								sklog.Errorf("Failed to save newly found cluster: %s", err)
							}
							sklog.Infof("Found High regression at %s for %q: %v", id.ID(), q, *cl.StepFit)
						}
					}
				}
			}
		}
		clusteringLatency.Stop()
		runsCounter.Inc(1)
	}
}
