// package baseline contains functions to gather the current baseline and
// write them to GCS.
package baseline

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

type BaseLine map[string]types.TestClassification

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

type CommitableBaseLine struct {
	Commit      *tiling.Commit      `json:"commmit"`
	Master      BaseLine            `json:"master"`
	ChangeLists map[string]BaseLine `json:"changeLists"`
}

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
