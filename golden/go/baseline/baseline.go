// package baseline contains functions to gather the current baseline and
// write them to GCS.
package baseline

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

type BaselineVal struct {
	Label    types.Label    `json:"label"`
	TraceIDs util.StringSet `json:"traceIds"`
}

func newBaselineVal(traceID string) *BaselineVal {
	return &BaselineVal{Label: types.POSITIVE, TraceIDs: util.StringSet{traceID: true}}
}

// BaseLine maps test names to a set of digests with their label.
// Test names are the names of individual tests, e.g. GMs and digests are
// hashes that uniquely identify an output image.
// This is used as the export format of baseline.
type BaseLine map[string]map[string]*BaselineVal

// add adds a test/digest pair to the baseline.
func (b BaseLine) add(testName, digest, traceID string) {
	if (testName == "") || (digest == types.MISSING_DIGEST) {
		return
	}
	if found, ok := b[testName]; ok {
		if baselineVal, ok := found[digest]; ok {
			baselineVal.TraceIDs[traceID] = true
		} else {
			found[digest] = newBaselineVal(traceID)
		}
	} else {
		b[testName] = map[string]*BaselineVal{digest: newBaselineVal(traceID)}
	}
}

// CommitableBaseLine captures the data necessary to verify test results on the
// commit queue.
type CommitableBaseLine struct {
	// Start commit covered by these baselines.
	StartCommit *tiling.Commit `json:"startCommit"`

	// Commit is the commit for which this baseline was collected.
	EndCommit *tiling.Commit `json:"endCommit"`

	// Baseline captures the baseline of the current commit.
	Baseline BaseLine `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. 0 indicates the master branch.
	Issue int64
}

// GetBaseline calculates the master baseline for the given configuration of
// expectations and the given tile. The commit of the baseline is last commit
// in tile.
func GetBaseline(exps *expstorage.Expectations, tile *tiling.Tile) *CommitableBaseLine {
	commits := tile.Commits
	var startCommit *tiling.Commit = nil
	var endCommit *tiling.Commit = nil

	masterBaseline := BaseLine{}
	for traceID, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
		if idx := gTrace.LastIndex(); idx >= 0 {
			digest := gTrace.Values[idx]
			if exps.Classification(testName, digest) == types.POSITIVE {
				masterBaseline.add(testName, digest, traceID)
			}

			c := commits[idx]
			if startCommit == nil || c.CommitTime < startCommit.CommitTime {
				startCommit = c
			}

			if endCommit == nil || c.CommitTime > endCommit.CommitTime {
				endCommit = c
			}
		}
	}

	ret := &CommitableBaseLine{
		Commit:   tile.Commits[tile.LastCommitIndex()],
		Baseline: masterBaseline,
		Issue:    0,
	}
	return ret
}

func GetBaselineForIssue(exp *expstorage.Expectations, talliesByTest map[string]tally.Tally) *CommitableBaseLine {
	return nil
}
