// package baseline contains functions to gather the current baseline and
// write them to GCS.
package baseline

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

// BaseLine maps test names to a set of digests with their label.
// Test names are the names of individual tests, e.g. GMs and digests are
// hashes that uniquely identify an output image.
// This is used as the export format of baseline.
type BaseLine map[string]types.TestClassification

// add adds a test/digest pair to the baseline.
func (b BaseLine) add(testName, digest string) {
	if (testName == "") || (digest == types.MISSING_DIGEST) {
		return
	}
	if found, ok := b[testName]; ok {
		found[digest] = types.POSITIVE
	} else {
		b[testName] = types.TestClassification{digest: types.POSITIVE}
	}
}

// CommitableBaseLine captures the data necessary to verify test results on the
// commit queue.
type CommitableBaseLine struct {
	// Commit is the commit for which this baseline was collected.
	Commit *tiling.Commit `json:"commit"`

	// Master captures the baseline of the current commit.
	Master BaseLine `json:"master"`

	// ChangeLists captures baselines for pending commits. These baselines have
	// been added by running baselines trybots and are commited to the master
	// baseline when the CL lands.
	ChangeLists map[string]BaseLine `json:"changeLists"`
}

// GetBaseline calculates the master baseline for the given configuration of
// expectations and the given tile. The commit of the baseline is last commit
// in tile.
func GetBaseline(exps *expstorage.Expectations, tile *tiling.Tile) *CommitableBaseLine {
	masterBaseline := BaseLine{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
		digest := gTrace.LastDigest()
		if exps.Classification(testName, digest) == types.POSITIVE {
			masterBaseline.add(testName, digest)
		}
	}

	ret := &CommitableBaseLine{
		Commit:      tile.Commits[tile.LastCommitIndex()],
		Master:      masterBaseline,
		ChangeLists: map[string]BaseLine{},
	}
	return ret
}
