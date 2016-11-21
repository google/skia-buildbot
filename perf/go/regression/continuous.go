package regression

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
)

const (
	NUM_COMMITS = 50
	RADIUS      = 5
)

type Continuous struct {
	git     *gitinfo.GitInfo
	cidl    *cid.CommitIDLookup
	queries []string
	store   *Store
}

func NewContinuous(git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, queries []string, store *Store) *Continuous {
	return &Continuous{
		git:     git,
		cidl:    cidl,
		queries: queries,
		store:   store,
	}
}

func (c *Continuous) Run() {
	// TODO(jcgregorio) Add liveness metrics.
	glog.Infof("Continuous starting.")
	for _ = range time.Tick(time.Minute) {
		// Get the last 50 commits.
		indexCommits := c.git.LastNIndex(NUM_COMMITS)
		// Drop the 5 most recent, since we are clustering
		// based on a radius of +/-5 commits.
		indexCommits = indexCommits[:45]
		for _, commit := range indexCommits {
			id := &cid.CommitID{
				Source: "master",
				Offset: commit.Index,
			}
			details, err := c.cidl.Lookup([]*cid.CommitID{id})
			if err != nil {
				glog.Errorf("Failed to look up commit %v: %s", *id, err)
			}
			for _, q := range c.queries {
				// Create ClusterRequest and run.
				req := &clustering2.ClusterRequest{
					Source: "master",
					Offset: commit.Index,
					Radius: RADIUS,
					Query:  q,
				}
				glog.Infof("Continuous: Clustering at %s for %q", id.ID(), q)

				resp, err := clustering2.Run(req, c.git, c.cidl)
				if err != nil {
					glog.Errorf("Failed while clustering %v %s", *req, err)
					continue
				}
				// Update database if regression at the midpoint is found.
				for _, cl := range resp.Summary.Clusters {
					if cl.StepPoint.Offset == int64(commit.Index) && cl.StepFit.Status != "Uninterestng" {
						if cl.StepFit.Status == "Low" {
							c.store.SetLow(details[0], q, resp.Frame, cl)
							glog.Infof("Found Low regression at %s for %q: %v", id.ID(), q, *cl.StepFit)
						}
						if cl.StepFit.Status == "High" {
							c.store.SetHigh(details[0], q, resp.Frame, cl)
							glog.Infof("Found High regression at %s for %q: %v", id.ID(), q, *cl.StepFit)
						}
					}
				}
			}
		}
	}
}
