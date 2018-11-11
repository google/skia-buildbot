package goldclient

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

const (
	GOLD_SERVICE_TEST_ID = "testinstance"
)

func TestGoldClient(t *testing.T) {
	buildConf := &BuildConfig{
		Key:           map[string]string{},
		GitHash:       "",
		Issue:         0,
		Patchset:      0,
		BuildBucketID: 0,
	}

	goldClient := New(GOLD_SERVICE_TEST_ID, buildConf)

	// Fetch the uninteresting hashes
	assert.NoError(t, goldClient.FetchUninterestingHashes())
	assert.NotEqual(t, 0, len(goldClient.UninterestingHashes()))

	// Run the tests and generate the images and make sure are consistent.
	testNames, imagePaths, digests, keys := runTests(t)

	// Add the results
	for i, testName := range testNames {
		current := goldClient.Result()
		assert.NoError(t, current.Add(testName, imagePaths[i], digests[i], keys[i]))
	}

	// Make sure the client reports results.
	assert.False(t, goldClient.Empty())
	assert.NoError(t, goldClient.UploadImages())
	assert.NoError(t, goldClient.UploadResult())
	assert.NoError(t, goldClient.Finish())
}

func runTests(t *testing.T) ([]string, []string, []string, []map[string]string) {
	testNames, imagePaths, hashes, keys := []string{}, []string{}, []string{}, []map[string]string{}
	assert.NotEqual(t, 0, len(testNames))
	assert.Equal(t, len(testNames), len(imagePaths))
	assert.Equal(t, len(testNames), len(hashes))
	assert.Equal(t, len(testNames), len(keys))
	return testNames, imagePaths, hashes, keys
}
