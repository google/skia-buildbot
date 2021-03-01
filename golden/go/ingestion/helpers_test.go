package ingestion

import (
	"context"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	// gcsTestDir is the directory from where to fetch GCS test data.
	gcsTestDir = "ingest-testdata/dm-json-v1"
)

var (
	beginningOfTime = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC)
	endOfTime       = time.Date(2015, time.October, 30, 0, 0, 0, 0, time.UTC)
	startTime       = time.Date(2015, time.October, 1, 0, 0, 0, 0, time.UTC)
	endTime         = time.Date(2015, time.October, 1, 23, 59, 59, 0, time.UTC)
)

func TestGoogleStorageSource(t *testing.T) {
	unittest.LargeTest(t)

	src, err := newGoogleStorageSource(context.Background(), "gs-test-src", gcs_testutils.TEST_DATA_BUCKET, gcsTestDir, http.DefaultClient, nil)
	require.NoError(t, err)
	testSource(t, src)
}

func testSource(t *testing.T, src Source) {
	testFilePaths := readTestFileNames(t)

	resultFileLocations := drainPollChannel(src.Poll(startTime, endTime))

	require.Equal(t, len(testFilePaths), len(resultFileLocations))
	sort.Sort(rflSlice(resultFileLocations))

	for idx, result := range resultFileLocations {
		require.True(t, strings.HasSuffix(result.Name(), testFilePaths[idx]))
	}

	// Make sure the narrow and wide time range produce the same result.
	allResultFileLocations := drainPollChannel(src.Poll(beginningOfTime, endOfTime))
	sort.Sort(rflSlice(allResultFileLocations))

	require.Equal(t, len(resultFileLocations), len(allResultFileLocations))
	for idx, result := range resultFileLocations {
		require.Equal(t, result.MD5(), allResultFileLocations[idx].MD5())
		require.Equal(t, result.Name(), allResultFileLocations[idx].Name())
	}
}

func readTestFileNames(t *testing.T) []string {
	// Read the expected list of files and compare them.
	content, err := ioutil.ReadFile("./testdata/filelist_2015_10_01.txt")
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	sort.Strings(lines)
	return lines
}

type rflSlice []ResultFileLocation

func (r rflSlice) Len() int           { return len(r) }
func (r rflSlice) Less(i, j int) bool { return r[i].Name() < r[j].Name() }
func (r rflSlice) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }

func drainPollChannel(ch <-chan ResultFileLocation) []ResultFileLocation {
	ret := []ResultFileLocation{}
	for rf := range ch {
		ret = append(ret, rf)
	}
	return ret
}
