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

// jsonJobHandler returns the current list of CT Pixel Diff jobs.
func jsonJobsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(lchoi): Add unit test for this function.
	// TODO(lchoi): List of jobs is hardcoded for now, but eventually we expect
	// to get them from either the list of buckets from the silverProcessor or
	// the DMStore.
	jobs := make([]string, 0)
	jobs = append(jobs, "rmistry-20170623165155")
	jobs = append(jobs, "rmistry-20170623184523")
	sendJsonResponse(w, map[string][]string{"jobs": jobs})
}

// jsonDiffHandler instantiates the map linking job strings to lists of diff
// results, and instantiates and fills in the list of diff results for a given job.
func jsonDiffHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(lchoi): Add unit test for this function.
	if (diffResultsMap == nil) {
		diffResultsMap = make(map[string][]*DiffResult)
	}

	// TODO(lchoi): Nopatch and withpatch screenshot paths in GS, as well as site
	// rank, are currently found using the CSV file of top sites.
	// (CSV data format: each row has the site rank in the first column and the
	// site url in the second column). This was just a way to see that the
	// frontend code was functional, as currently CT does not store any metadata
	// regarding what screenshots were actually taken during the pixel diff run.
	// Once rmistry implements this functionality, the silveringestion and dmstore
	// packages will be used to continuously parse the JSON metadata, calculate
	// diffs, and store the images and results in the background. Then, this
	// handler will simply reconstruct the local image paths by using the job
	// parameter to find the correct bucket in the DMStore and processing all the
	// records in that bucket. Values like the site URL, diff metrics, and site
	// rank will also be returned to the frontend by querying the DMStore. This
	// will completely eliminate the need to read and parse this CSV.
	job := r.FormValue("job")
	if (diffResultsMap[job] == nil) {
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

		// Iterate to 1000 as rmistry's test runs only took screenshots of the top
		// 1000 sites. Again, once we have metadata in GS, none of these hardcoded
		// values will be necessary.
		for i := 0; i < 1000; i++ {
			siteUrl := rawcsvdata[i][1]
			images := &ImageResult{
				Left: job + "/nopatch/http___www_" + strings.Replace(siteUrl, ".", "_", -1),
				Right: job + "/withpatch/http___www_" + strings.Replace(siteUrl, ".", "_", -1),
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
			diffResultsMap[job] = append(diffResultsMap[job], diffResult)
		}
	}
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
	// TODO(lchoi): Add unit test for this function.
	job := r.FormValue("job")
	if diffResultsMap[job] != nil {
		startIdx, err := strconv.Atoi(r.FormValue("startIdx"))
		if err != nil {
			sklog.Errorf("Failed to parse start index: %s", err)
		}
		endIdx, err := strconv.Atoi(r.FormValue("endIdx"))
		if err != nil {
			sklog.Errorf("Failed to parse end index: %s", err)
		}
		sendJsonResponse(w, map[string]interface{}{"data": diffResultsMap[job][startIdx:endIdx]})
	}
}

// jsonSortHandler sorts the list of diff results using the specified sort value
// and job.
func jsonSortHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(lchoi): Add unit test for this function.
	job := r.FormValue("job")
	diffResults := diffResultsMap[job]
	sortVal, err := strconv.Atoi(r.FormValue("sortVal"))
	if err != nil {
		sklog.Errorf("Failed to parse sort value: %s", err)
	}
	switch sortVal {
	// TODO(lchoi): Once we have metadata, none of this nil checking will be
	// necessary, as all DiffResult objects will have data. However, we should
	// add some functionality to break ties using URL and/or rank if the diff
	// metric values are equal. Will hold off on doing this as I'm not entirely
	// how much I'll have to refactor the structs in this class once we have
	// metadata to parse.
	case NUM_DIFF_PIXELS_DSC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.NumDiffPixels > diffResults[j].Diff.NumDiffPixels
		})
	case 	NUM_DIFF_PIXELS_ASC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.NumDiffPixels < diffResults[j].Diff.NumDiffPixels
		})
	case PER_DIFF_PIXELS_DSC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.PixelDiffPercent > diffResults[j].Diff.PixelDiffPercent
		})
	case PER_DIFF_PIXELS_ASC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.PixelDiffPercent < diffResults[j].Diff.PixelDiffPercent
		})
	case MAX_RED_DIFF_DSC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[0] > diffResults[j].Diff.MaxRGBADiffs[0]
		})
	case MAX_RED_DIFF_ASC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[0] < diffResults[j].Diff.MaxRGBADiffs[0]
		})
	case MAX_GREEN_DIFF_DSC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[1] > diffResults[j].Diff.MaxRGBADiffs[1]
		})
	case MAX_GREEN_DIFF_ASC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[1] < diffResults[j].Diff.MaxRGBADiffs[1]
		})
	case MAX_BLUE_DIFF_DSC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[2] > diffResults[j].Diff.MaxRGBADiffs[2]
		})
	case MAX_BLUE_DIFF_ASC:
		sort.Slice(diffResults, func(i, j int) bool {
			if diffResults[i].Diff == nil || diffResults[j].Diff == nil {
				return true
			}
			return diffResults[i].Diff.MaxRGBADiffs[2] < diffResults[j].Diff.MaxRGBADiffs[2]
		})
	case SITE_RANK_DSC:
		sort.Slice(diffResults, func(i, j int) bool {
			return diffResults[i].Rank < diffResults[j].Rank
		})
	case SITE_RANK_ASC:
		sort.Slice(diffResults, func(i, j int) bool {
			return diffResults[i].Rank > diffResults[j].Rank
		})
	}
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
