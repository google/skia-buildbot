package main

import (
	"sort"
	"strconv"
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

// jsonJobHandler returns the current list of CT Pixel Diff jobs.
func jsonJobsHandler(w http.ResponseWriter, r *http.Request) {
	jobs := make([]string, 0)
	jobs = append(jobs, "rmistry-20170623165155")
	jobs = append(jobs, "rmistry-20170623184523")
	sendJsonResponse(w, map[string][]string{"jobs": jobs})
}

// jsonDiffHandler resets the list of diff results.
func jsonDiffHandler(w http.ResponseWriter, r *http.Request) {
	diffResults = make([]*DiffResult, 0)
}

// DiffResult encapsulates the results of a diff request.
type DiffResult struct {
	Url       string `json:"url"`
	Images    *ImageResult  `json:"images"`
	Diff      *diff.DiffMetrics  `json:"diffmetrics"`
	Rank      int  `json:"rank"`
}

// ImageResult encapsulates nopatch and withpatch images.
type ImageResult struct {
	Left   string  `json:"left"`
	Right  string  `json:"right"`
}

// jsonLoadHandler parses a start index, end index, and job from the query and
// uses them to return diff results in the specified range for the specified run.
func jsonLoadHandler(w http.ResponseWriter, r *http.Request) {
	if diffResults != nil {
		startIdx, err := strconv.Atoi(r.FormValue("startIdx"))
		if err != nil {
			sklog.Errorf("Failed to parse start index: %s", err)
		}
		endIdx, err := strconv.Atoi(r.FormValue("endIdx"))
		if err != nil {
			sklog.Errorf("Failed to parse end index: %s", err)
		}
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

		job := r.FormValue("job")

		for i := startIdx; i < endIdx; i++ {
			siteUrl := rawcsvdata[i][1]
			images := &ImageResult{
				Left: job + "--nopatch--http___www_" + strings.Replace(siteUrl, ".", "_", -1),
				Right: job + "--withpatch--http___www_" + strings.Replace(siteUrl, ".", "_", -1),
			}
			diff, err := diffStore.Get(diff.PRIORITY_NOW, images.Left, []string{images.Right})
			if err != nil {
				sklog.Errorf("Failed to calculate diffs: %s", err)
				return
			}
			rank, err := strconv.Atoi(rawcsvdata[i][0])
			if err != nil {
				sklog.Errorf("Failed to parse web page rank: %s", err)
			}
			diffResult := &DiffResult {
				Url: siteUrl,
				Images: images,
				Diff: diff[images.Right],
				Rank: rank,
			}
			diffResults = append(diffResults, diffResult)
		}
		sendJsonResponse(w, map[string]interface{}{"data": diffResults[startIdx:endIdx]})
	}
}

// jsonSortHandler sorts the list of diff results using the specified sort value
func jsonSortHandler(w http.ResponseWriter, r *http.Request) {
	sortVal, err := strconv.Atoi(r.FormValue("sortVal"))
	if err != nil {
		sklog.Errorf("Failed to parse sort value: %s", err)
	}
	switch sortVal {
	case 0:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.NumDiffPixels > diffResults[j].Diff.NumDiffPixels
		})
	case 1:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.NumDiffPixels < diffResults[j].Diff.NumDiffPixels
		})
	case 2:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.PixelDiffPercent > diffResults[j].Diff.PixelDiffPercent
		})
	case 3:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.PixelDiffPercent < diffResults[j].Diff.PixelDiffPercent
		})
	case 4:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[0] > diffResults[j].Diff.MaxRGBADiffs[0]
		})
	case 5:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[0] < diffResults[j].Diff.MaxRGBADiffs[0]
		})
	case 6:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[1] > diffResults[j].Diff.MaxRGBADiffs[1]
		})
	case 7:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[1] < diffResults[j].Diff.MaxRGBADiffs[1]
		})
	case 8:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[2] > diffResults[j].Diff.MaxRGBADiffs[2]
		})
	case 9:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[2] < diffResults[j].Diff.MaxRGBADiffs[2]
		})
	case 10:
		sort.Slice(diffResults, func(i, j int) bool {
			return diffResults[i].Rank < diffResults[j].Rank
		})
	case 11:
		sort.Slice(diffResults, func(i, j int) bool {
			return diffResults[i].Rank > diffResults[j].Rank
		})
	}
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
