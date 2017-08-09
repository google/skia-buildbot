package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.skia.org/infra/ct_pixel_diff/go/dynamicdiff"
	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

const (
	// Test ResultStore data.
	TEST_RUN_ID     = "lchoi-20170726123456"
	TEST_RUN_ID_TWO = "rmistry-20170717202555"
	TEST_URL        = "http://www.google.com"
	TEST_URL_TWO    = "http://www.youtube.com"
)

func createResultStore(t *testing.T) resultstore.ResultStore {
	// Set up the temporary directory and create the ResultStore.
	diffDir, err := ioutil.TempDir("", "diffs")
	assert.NoError(t, err)
	rs, err := resultstore.NewBoltResultStore(diffDir, "diffs.db")
	assert.NoError(t, err)
	return rs
}

func TestJsonRunsHandler(t *testing.T) {
	testutils.MediumTest(t)

	// Create a ResultStore and assign it to the module level variable so that
	// the handler can interact with it.
	rs := createResultStore(t)
	resultStore = rs

	rec := &resultstore.ResultRec{}
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, rec)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID_TWO, TEST_URL_TWO, rec)
	assert.NoError(t, err)

	// Create a request to the json runs endpoint to run the jsonRunsHandler.
	req, err := http.NewRequest("GET", "/json/runs", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonRunsHandler)
	handler.ServeHTTP(rr, req)

	expected := map[string][]string{
		"runs": []string{TEST_RUN_ID, TEST_RUN_ID_TWO},
	}
	results := map[string][]string{}
	err = json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)
	assert.Equal(t, expected, results)
}

func TestJsonRenderHandler(t *testing.T) {
	testutils.MediumTest(t)

	// Create a ResultStore and assign it to the module level variable so that
	// the handler can interact with it.
	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL,
		Rank:         1,
		NoPatchImg:   "lchoi-20170726123456/nopatch/1/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/1/http___www_google_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL_TWO,
		Rank:         2,
		NoPatchImg:   "lchoi-20170726123456/nopatch/2/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/2/http___www_google_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)

	// Create a request with the appropriate query parameters to the json render
	// endpoint to run the jsonRenderHandler.
	req, err := http.NewRequest("GET", "/json/render", nil)
	assert.NoError(t, err)
	q := req.URL.Query()
	q.Add("runID", TEST_RUN_ID)
	q.Add("startIdx", "0")
	q.Add("endIdx", "2")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonRenderHandler)
	handler.ServeHTTP(rr, req)

	expected := map[string][]*resultstore.ResultRec{
		"results": []*resultstore.ResultRec{recOne, recTwo},
	}
	results := map[string][]*resultstore.ResultRec{}
	err = json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)
	assert.Equal(t, expected, results)
}

func TestJsonSortHandler(t *testing.T) {
	testutils.MediumTest(t)

	// Create a ResultStore and assign it to the module level variable so that
	// the handler can interact with it.
	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL,
		Rank:         1,
		NoPatchImg:   "lchoi-20170726123456/nopatch/1/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/1/http___www_google_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL_TWO,
		Rank:         2,
		NoPatchImg:   "lchoi-20170726123456/nopatch/2/http___www_youtube_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/2/http___www_youtube_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)

	// Create a request with the appropriate query parameters to the json sort
	// endpoint to run the jsonSortHandler.
	req, err := http.NewRequest("GET", "/json/sort", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", TEST_RUN_ID)
	q.Add("sortField", "rank")
	q.Add("sortOrder", "ascending")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonSortHandler)
	handler.ServeHTTP(rr, req)

	expected := []*resultstore.ResultRec{recTwo, recOne}
	actual, err := resultStore.GetRange(TEST_RUN_ID, 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestJsonURLsHandler(t *testing.T) {
	testutils.MediumTest(t)

	// Create a ResultStore and assign it to the module level variable so that
	// the handler can interact with it.
	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL,
		NoPatchImg:   "lchoi-20170726123456/nopatch/1/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/1/http___www_google_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL_TWO,
		Rank:         2,
		NoPatchImg:   "lchoi-20170726123456/nopatch/2/http___www_youtube_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/2/http___www_youtube_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)

	// Create a request with the appropriate query parameters to the json urls
	// endpopint to run the jsonURLsHandler.
	req, err := http.NewRequest("GET", "/json/urls", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", TEST_RUN_ID)
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonURLsHandler)
	handler.ServeHTTP(rr, req)

	expectedOne := map[string]string{
		"text":  "google.com",
		"value": "http://www.",
	}
	expectedTwo := map[string]string{
		"text":  "youtube.com",
		"value": "http://www.",
	}
	results, err := resultStore.GetURLs(TEST_RUN_ID)
	assert.NoError(t, err)
	assert.Equal(t, expectedOne, results[0])
	assert.Equal(t, expectedTwo, results[1])
}

func TestJsonSearchHandler(t *testing.T) {
	testutils.MediumTest(t)

	// Create a ResultStore and assign it to the module level variable so that
	// the handler can interact with it.
	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL,
		NoPatchImg:   "lchoi-20170726123456/nopatch/1/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/1/http___www_google_com",
		DiffMetrics:  &dynamicdiff.DynamicDiffMetrics{},
	}
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)

	// Create a request with the appropriate query parameters to the json urls
	// endpopint to run the jsonURLsHandler.
	req, err := http.NewRequest("GET", "/json/search", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", TEST_RUN_ID)
	q.Add("url", TEST_URL)
	req.URL.RawQuery = q.Encode()

	result, err := resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Equal(t, recOne, result)
}
