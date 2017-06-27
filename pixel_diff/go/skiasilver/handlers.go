package main

import (
	"strings"
	"os"
	"encoding/csv"
	"net/http"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/go/sklog"
)

const (
	// DEFAULT_PAGE_SIZE is the default page size used for pagination.
	DEFAULT_PAGE_SIZE = 20

	// MAX_PAGE_SIZE is the maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

func jsonSearchHandler(w http.ResponseWriter, r *http.Request) {
	csvfile, err := os.Open("/usr/local/google/home/lchoi/Downloads/top1m.csv")
	if err != nil {
		sklog.Error(err)
		return
	}
	defer csvfile.Close()

	reader := csv.NewReader(csvfile)
	rawcsvdata, err := reader.ReadAll()
	if err != nil {
		sklog.Error(err)
		return
	}

	searchResults := make([]*SearchResult, 0)
	for i := 0; i < 1000; i++ {
		siteUrl := rawcsvdata[i][1]
		images := &ImageResult{
			Left: "nopatch-http___www_" + strings.Replace(siteUrl, ".", "_", -1),
			Right: "withpatch-http___www_" + strings.Replace(siteUrl, ".", "_", -1),
		}
		diff, err := diffStore.Get(diff.PRIORITY_NOW, images.Left, []string{images.Right})
		if err != nil {
			sklog.Errorf("Failed to calculate diffs: %s", err)
			return
		}
		searchResult := &SearchResult {
			Images: images,
			Diff: diff[images.Right],
		}
		searchResults = append(searchResults, searchResult)
	}

	sendJsonResponse(w, map[string]interface{}{"data": searchResults})
}

// SearchResult encapsulates the results of a search request.
type SearchResult struct {
	Images    *ImageResult  `json:"images"`
	Diff      *diff.DiffMetrics  `json:"diffmetrics"`
}

type ImageResult struct {
	Left   string  `json:"left"`
	Right  string  `json:"right"`
}

// jsonStatusHandler returns the current status of with respect to HEAD.
func jsonStatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJsonResponse(w, statusWatcher.GetStatus())
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
