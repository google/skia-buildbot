package storage

import (
	"fmt"
	"testing"
	"time"

	// gstorage "cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/types"
)

const (
	// TEST_HASHES_GS_PATH is the bucket/path combination where the test file will be written.
	TEST_HASHES_GS_PATH   = "skia-infra-testdata/hash_files/testing-known-hashes.txt"
	TEST_BASELINE_GS_PATH = "skia-infra-testdata/hash_files/testing-baselines"
)

func TestGStorageClient(t *testing.T) {
	testutils.MediumTest(t)

	timeStamp := fmt.Sprintf("%032d", time.Now().UnixNano())
	opt := &GSClientOptions{
		HashesGSPath:   TEST_HASHES_GS_PATH + "-" + timeStamp,
		BaselineGSPath: TEST_BASELINE_GS_PATH,
	}
	gsClient, err := NewGStorageClient(nil, opt)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, gsClient.removeGSPath(opt.HashesGSPath))
	}()

	knownDigests := []string{
		"c003788f8d306ff1226e2a460835dae4",
		"885b31941c25efc313b0fd66d55b86d9",
		"264d0d87b12ba337f796fc592cd5357d",
		"69c2fbf8e89a48058b2f45ad4ea46a35",
		"2c4d605c16e7d5b23294c0433fa3ed17",
		"782717cf6ed9329fc43cb5a6c830cbce",
		"e143ca619f2172d06bb0dcc4d72af414",
		"26aff0619c829bc149f7c0171fcca442",
		"72d61ae8e232c3a279cc3cdbf6ef73e5",
		"f1eb049dac1cfa3c70aac8fc6ad5496f",
		timeStamp,
	}
	assert.NoError(t, gsClient.WriteKnownDigests(knownDigests))

	found, err := gsClient.loadKnownDigests()
	assert.NoError(t, err)
	assert.Equal(t, knownDigests, found)

	masterBaseline := &baseline.CommitableBaseLine{
		StartCommit: nil,
		EndCommit:   nil,
		Baseline: baseline.Baseline{
			"test-1": map[string]types.Label{"d1": types.POSITIVE},
		},
		Issue: 0,
	}

	_, err = gsClient.WriteBaseLine(masterBaseline)
	assert.NoError(t, err)

	foundBaseline, err := gsClient.ReadBaseline(0)
	assert.NoError(t, err)
	assert.Equal(t, masterBaseline, foundBaseline)

	// Add a baseline for an issue
	issueID := int64(5678)
	issueBaseline := &baseline.CommitableBaseLine{
		StartCommit: nil,
		EndCommit:   nil,
		Baseline: baseline.Baseline{
			"test-3": map[string]types.Label{"d2": types.POSITIVE},
		},
		Issue: issueID,
	}

	_, err = gsClient.WriteBaseLine(issueBaseline)
	assert.NoError(t, err)
	foundBaseline, err = gsClient.ReadBaseline(issueID)
	assert.NoError(t, err)
	assert.Equal(t, issueBaseline, foundBaseline)

	// Test reading a non-existing baseline for an issue
	foundBaseline, err = gsClient.ReadBaseline(5344)
	assert.NoError(t, err)
	assert.Equal(t, &baseline.CommitableBaseLine{Baseline: map[string]types.TestClassification{}}, foundBaseline)

	// Fetch the combined baselines
	storages := &Storage{GStorageClient: gsClient}
	combined := &baseline.CommitableBaseLine{}
	*combined = *masterBaseline
	combined.Baseline = masterBaseline.Baseline.Clone().Merge(issueBaseline.Baseline)
	assert.NotEqual(t, masterBaseline, combined)
	assert.True(t, len(masterBaseline.Baseline) > 0)
	foundBaseline, err = storages.FetchBaseline(issueID)
	assert.NoError(t, err)
	assert.Equal(t, combined, foundBaseline)

	// Fetch the combined baselines when there are no baselines for the issue
	foundBaseline, err = storages.FetchBaseline(5344)
	assert.NoError(t, err)
	assert.Equal(t, masterBaseline, foundBaseline)
}
