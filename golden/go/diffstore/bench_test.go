package diffstore

import (
	"fmt"
	"os"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/types"
)

const TEST_FILE_NAME = "sample.tile"

func BenchmarkMemDiffStore(b *testing.B) {
	sample := loadSample(b)

	baseDir := TEST_DATA_BASE_DIR + "-bench-diffstore"
	client := getClient(b)
	defer testutils.RemoveAll(b, baseDir)

	memIgnoreStore := ignore.NewMemIgnoreStore()
	for _, ir := range sample.IgnoreRules {
		assert.NoError(b, memIgnoreStore.Create(ir))
	}
	matcher, err := memIgnoreStore.BuildRuleMatcher()
	assert.NoError(b, err)

	// Build storages and get the digests that are not ignored.
	byTest := map[string]util.StringSet{}
	for _, trace := range sample.Tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		if _, ok := matcher(gTrace.Params_); !ok {
			testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
			if found, ok := byTest[testName]; ok {
				found.AddLists(gTrace.Values)
			} else {
				byTest[testName] = util.NewStringSet(gTrace.Values)
			}
		}
	}

	diffStore, err := New(client, baseDir, TEST_GS_BUCKET_NAME, TEST_GS_IMAGE_DIR)
	counter := 0
	//	processed := 0
	b.ResetTimer()
	for _, digestSet := range byTest {
		// Remove the missing digest sentinel.
		delete(digestSet, types.MISSING_DIGEST)

		digests := digestSet.Keys()
		diffStore.WarmDigests(diff.PRIORITY_NOW, digests)
		diffStore.WarmDiffs(diff.PRIORITY_NOW, digests, digests)
		counter += len(digests) * (len(digests) - 1) / 2
		// processed++
		// if processed > 5 {
		// 	break
		// }
	}

	// iterate over the tests and warm the
	diffStore.(*MemDiffStore).sync()
	fmt.Printf("Comparisons: %d\n", counter)
}

func loadSample(t assert.TestingT) *serialize.Sample {
	file, err := os.Open(TEST_FILE_NAME)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
