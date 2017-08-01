package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"

	"github.com/stretchr/testify/assert"
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

	rs := createResultStore(t)
	resultStore = rs
	rec := &resultstore.ResultRec{}
	resultStore.Put("lchoi-20170726123456", "http://www.google.com", rec)
	resultStore.Put("rmistry-20170717202555", "http://www.google.com", rec)

	req, err := http.NewRequest("GET", "/json/runs", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonRunsHandler)
	handler.ServeHTTP(rr, req)

	expected := map[string][]string{
		"runs": []string{"lchoi-20170726123456", "rmistry-20170717202555"},
	}
	results := map[string][]string{}
	err = json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)
	assert.Equal(t, expected, results)
}

func TestJsonRenderHandler(t *testing.T) {
	testutils.MediumTest(t)

	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID:        "lchoi-20170726123456",
		URL:          "http://www.google.com",
		Rank:         1,
		NoPatchImg:   "lchoi-20170726123456/nopatch/1/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/1/http___www_google_com",
		DiffMetrics:  &diff.DiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:        "lchoi-20170726123456",
		URL:          "http://www.youtube.com",
		Rank:         2,
		NoPatchImg:   "lchoi-20170726123456/nopatch/2/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/2/http___www_google_com",
		DiffMetrics:  &diff.DiffMetrics{},
	}
	resultStore.Put("lchoi-20170726123456", "http://www.google.com", recOne)
	resultStore.Put("lchoi-20170726123456", "http://www.youtube.com", recTwo)

	req, err := http.NewRequest("GET", "/json/render", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", "lchoi-20170726123456")
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

	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID:        "lchoi-20170726123456",
		URL:          "http://www.google.com",
		Rank:         1,
		NoPatchImg:   "lchoi-20170726123456/nopatch/1/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/1/http___www_google_com",
		DiffMetrics:  &diff.DiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:        "lchoi-20170726123456",
		URL:          "http://www.youtube.com",
		Rank:         2,
		NoPatchImg:   "lchoi-20170726123456/nopatch/2/http___www_google_com",
		WithPatchImg: "lchoi-20170726123456/withpatch/2/http___www_google_com",
		DiffMetrics:  &diff.DiffMetrics{},
	}
	resultStore.Put("lchoi-20170726123456", "http://www.google.com", recOne)
	resultStore.Put("lchoi-20170726123456", "http://www.youtube.com", recTwo)

	req, err := http.NewRequest("GET", "/json/sort", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", "lchoi-20170726123456")
	q.Add("sortField", "rank")
	q.Add("sortOrder", "ascending")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonSortHandler)
	handler.ServeHTTP(rr, req)

	expected := []*resultstore.ResultRec{recTwo, recOne}
	actual, err := resultStore.GetRange("lchoi-20170726123456", 0, 2)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
