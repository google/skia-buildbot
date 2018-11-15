package goldclient

import (
	"image"
	"testing"

	assert "github.com/stretchr/testify/require"
)

const (
	GOLD_SERVICE_TEST_URL = "https://gold.skia.org"
)

func TestGoldClient(t *testing.T) {
	goldClient, err := NewCloudClient(GOLD_SERVICE_TEST_URL, nil)
	assert.NoError(t, err)
	assert.Nil(t, goldClient)

	envConfig := &EnvConfig{
		GitHash:       "abc123",
		Key:           map[string]string{},
		Issue:         12345,
		BuildBucketID: 454266563345,
		Patchset:      2,
	}
	img := &image.NRGBA{}
	assert.NoError(t, goldClient.Test("some_test_name", img, envConfig))

	// // Fetch the uninteresting hashes
	// assert.NoError(t, goldClient.FetchUninterestingHashes())
	// assert.NotEqual(t, 0, len(goldClient.UninterestingHashes()))

	// // Run the tests and generate the images and make sure are consistent.
	// testNames, imagePaths, digests, keys := runTests(t)

	// // Add the results
	// for i, testName := range testNames {
	// 	current := goldClient.Result()
	// 	assert.NoError(t, current.Add(testName, imagePaths[i], digests[i], keys[i]))
	// }

	// // Make sure the client reports results.
	// assert.False(t, goldClient.Empty())
	// assert.NoError(t, goldClient.UploadImages())
	// assert.NoError(t, goldClient.UploadResult())
	// assert.NoError(t, goldClient.Finish())
}

func runTests(t *testing.T) ([]string, []string, []string, []map[string]string) {
	testNames, imagePaths, hashes, keys := []string{}, []string{}, []string{}, []map[string]string{}
	assert.NotEqual(t, 0, len(testNames))
	assert.Equal(t, len(testNames), len(imagePaths))
	assert.Equal(t, len(testNames), len(hashes))
	assert.Equal(t, len(testNames), len(keys))
	return testNames, imagePaths, hashes, keys
}
