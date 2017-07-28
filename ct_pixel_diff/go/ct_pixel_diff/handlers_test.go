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

func TestJsonLoadHandler(t *testing.T) {
	testutils.MediumTest(t)

	rs := createResultStore(t)
	resultStore = rs
	recOne := &resultstore.ResultRec{
		RunID: "lchoi-20170726123456",
		URL:   "http://www.google.com",
		Rank:  1,
	}
	resultStore.Put("lchoi-20170726123456", "http://www.youtube.com", recOne)

	rm := map[string][]*resultstore.ResultRec{}
	resultsMap = rm

	req, err := http.NewRequest("GET", "/json/load", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", "lchoi-20170726123456")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonLoadHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, 1, len(resultsMap["lchoi-20170726123456"]))
	assert.Equal(t, recOne, resultsMap["lchoi-20170726123456"][0])
}

func TestJsonRenderHandler(t *testing.T) {
	testutils.MediumTest(t)

	rm := map[string][]*resultstore.ResultRec{}
	resultsMap = rm
	recOne := &resultstore.ResultRec{
		RunID:       "lchoi-20170726123456",
		URL:         "http://www.google.com",
		Rank:        1,
		DiffMetrics: &diff.DiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:       "lchoi-20170726123456",
		URL:         "http://www.youtube.com",
		Rank:        2,
		DiffMetrics: &diff.DiffMetrics{},
	}
	resultsMap["lchoi-20170726123456"] = []*resultstore.ResultRec{recOne, recTwo}

	req, err := http.NewRequest("GET", "/json/render", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", "lchoi-20170726123456")
	q.Add("index", "0")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonRenderHandler)
	handler.ServeHTTP(rr, req)

	results := map[string]interface{}{}
	err = json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Equal(t, float64(2), results["index"])
}

func TestJsonSortHandler(t *testing.T) {
	testutils.MediumTest(t)

	rm := map[string][]*resultstore.ResultRec{}
	resultsMap = rm
	recOne := &resultstore.ResultRec{
		RunID:       "lchoi-20170726123456",
		URL:         "http://www.google.com",
		Rank:        1,
		DiffMetrics: &diff.DiffMetrics{},
	}
	recTwo := &resultstore.ResultRec{
		RunID:       "lchoi-20170726123456",
		URL:         "http://www.youtube.com",
		Rank:        2,
		DiffMetrics: &diff.DiffMetrics{},
	}
	resultsMap["lchoi-20170726123456"] = []*resultstore.ResultRec{recOne, recTwo}

	req, err := http.NewRequest("GET", "/json/render", nil)
	assert.NoError(t, err)

	q := req.URL.Query()
	q.Add("runID", "lchoi-20170726123456")
	q.Add("sortVal", "11")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(jsonSortHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, recTwo, resultsMap["lchoi-20170726123456"][0])
	assert.Equal(t, recOne, resultsMap["lchoi-20170726123456"][1])
}
