package diffstore

import (
	"context"
	"os"
	"sync"
	"testing"

	"go.skia.org/infra/golden/go/ignore"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

const (
	TEST_FILE_NAME  = "sample.tile"
	PROCESS_N_TESTS = 5000
)

func BenchmarkMemDiffStore(b *testing.B) {
	sample := loadSample(b)

	baseDir := d_utils.TEST_DATA_BASE_DIR + "-bench-diffstore"
	client := d_utils.GetHTTPClient(b)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	assert.NoError(b, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)
	defer testutils.RemoveAll(b, baseDir)

	ignoreMatcher, err := ignore.BuildRuleMatcher(sample.IgnoreRules)
	assert.NoError(b, err)

	// Build storages and get the digests that are not ignored.
	byTest := map[types.TestName]types.DigestSet{}
	for _, trace := range sample.Tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		if _, ok := ignoreMatcher(gTrace.Keys); !ok && gTrace.Corpus() == "gm" {
			testName := gTrace.TestName()
			if found, ok := byTest[testName]; ok {
				found.AddLists(gTrace.Digests)
			} else {
				byTest[testName] = types.DigestSet{}.AddLists(gTrace.Digests)
			}
		}
	}

	mapper := disk_mapper.New(&diff.DiffMetrics{})
	mfs := &mocks.FailureStore{}
	diffStore, err := NewMemDiffStore(gcsClient, baseDir, d_utils.TEST_GCS_IMAGE_DIR, 10, mapper, mfs)
	assert.NoError(b, err)
	allDigests := make([]types.DigestSlice, 0, PROCESS_N_TESTS)
	processed := 0
	var wg sync.WaitGroup
	for _, digestSet := range byTest {
		// Remove the missing digest sentinel.
		delete(digestSet, types.MISSING_DIGEST)

		digests := digestSet.Keys()
		allDigests = append(allDigests, digests)
		diffStore.WarmDigests(diff.PRIORITY_NOW, digests, false)

		wg.Add(1)
		go func(digests types.DigestSlice) {
			defer wg.Done()
			for _, d1 := range digests {
				_, _ = diffStore.Get(diff.PRIORITY_NOW, d1, digests)
			}
		}(digests)

		processed++
		if processed >= PROCESS_N_TESTS {
			break
		}
	}
	wg.Wait()

	// Now retrieve all of them again.
	b.ResetTimer()
	for _, digests := range allDigests {
		wg.Add(1)
		go func(digests types.DigestSlice) {
			defer wg.Done()
			for _, d1 := range digests {
				_, _ = diffStore.Get(diff.PRIORITY_NOW, d1, digests)
			}
		}(digests)
	}
	wg.Wait()
}

func loadSample(t assert.TestingT) *serialize.Sample {
	file, err := os.Open(TEST_FILE_NAME)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
