package main

import (
	"net/http"

	"go.skia.org/infra/go/sklog"
)

const (
	// DEFAULT_PAGE_SIZE is the default page size used for pagination.
	DEFAULT_PAGE_SIZE = 20

	// MAX_PAGE_SIZE is the maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

func jsonSearchHandler(w http.ResponseWriter, r *http.Request) {
	ret := make([]*ImageResult, 0, 1)
	images := &ImageResult{Left: "7691bf3556f560442a1b3e69fc26b49b", Right: "507bf123a5ed75a2f90a7259a0ab1069"}
	ret = append(ret, images)
	sklog.Infof("%d", len(ret))
	sendJsonResponse(w, &SearchResult{
		Images: ret,
	})
}

// SearchResult encapsulates the results of a search request.
type SearchResult struct {
	Images    []*ImageResult  `json:"images"`
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
