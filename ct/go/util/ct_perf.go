// Utility that contains functions to interact with ct-perf.skia.org.
package util

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingest/format"
)

// AddCTRunDataToPerf converts and uploads data from the CT run to CT's perf instance.
//
// It does the following:
// 1) Adds a commit to CT Perf's synthetic repo in https://skia.googlesource.com/perf-ct/+show/master
// 2) Constructs a results struct in the format of https://skia.googlesource.com/buildbot/+doc/master/perf/FORMAT.md
//    Ensures that the results struct has as key the runID, groupName and the git hash from (1).
//    Populates the results struct using the output CSV file from CT's run.
// 3) Create JSON file from the results struct.
// 4) Uploads the results file to Google storage bucket CT_PERF_BUCKET for ingestion by ct-perf.skia.org.
//    It is stored in location of this format: gs://<bucket>/<one or more dir names>/YYYY/MM/DD/HH/<zero or more dir names><some unique name>.json
//
// For example, it converts the following example CSV file:
//
// paint_op_count,traceUrls,pixels_rasterized,rasterize_time (ms),record_time_caching_disabled (ms),record_time_subsequence_caching_disabled (ms),painter_memory_usage (B),record_time_construction_disabled (ms),page_name
// 805.0,,1310720.0,2.449,1.128,0.283,25856.0,0.335,http://www.reuters.com (#480)
// 643.0,,1310720.0,2.894,0.998,0.209,24856.0,0.242,http://www.rediff.com (#490)
//
// into
//
//  {
//    "gitHash" : "8dcc84f7dc8523dd90501a4feb1f632808337c34",
//    "runID" : "rmistry-xyz",
//    "key" : {
//      "group_name" : "BGPT perf"
//    },
//    "results" : {
//      "http://www.reuters.com" : {
//        "default" : {
//          "paint_op_count": 805.0,
//          "pixels_rasterized": 1310720.0,
//          "rasterize_time (ms)": 2.449,
//          "record_time_caching_disabled (ms)": 1.128,
//          "record_time_subsequence_caching_disabled (ms)": 0.283,
//          "painter_memory_usage (B)": 25856.0,
//          "record_time_construction_disabled (ms)": 0.335,
//          "options" : {
//            "page_rank" : 480,
//          },
//        },
//      "http://www.rediff.com" : {
//        "default" : {
//          "paint_op_count": 643.0,
//          "pixels_rasterized": 1310720.0,
//          "rasterize_time (ms)": 2.894,
//          "record_time_caching_disabled (ms)": 0.998,
//          "record_time_subsequence_caching_disabled (ms)": 0.209,
//          "painter_memory_usage (B)": 24856.0,
//          "record_time_construction_disabled (ms)": 0.242,
//          "options" : {
//            "page_rank" : 490,
//          },
//        },
//      }
//    }
//  }
//
func AddCTRunDataToPerf(ctx context.Context, groupName, runID, pathToCSVResults, gitExec string, gs *GcsUtil) error {
	// Set uniqueID and create the workdir.
	uniqueID := fmt.Sprintf("%s-%d", runID, time.Now().Unix())
	workdir := path.Join(CTPerfWorkDir, uniqueID)
	MkdirAll(workdir, 0700)
	defer util.RemoveAll(workdir)

	// Step 1: Add a commit to CT Perf's synthetic repo in CT_PERF_REPO
	tmpDir, err := ioutil.TempDir(workdir, uniqueID)
	if err != nil {
		return skerr.Fmt("Could not create tmpDir in %s: %s", workdir, err)
	}
	checkout, err := git.NewCheckout(ctx, CT_PERF_REPO, tmpDir)
	if err != nil {
		return skerr.Fmt("Could not create %s checkout in %s: %s", CT_PERF_REPO, tmpDir, err)
	}
	hash, err := commitToSyntheticRepo(ctx, groupName, uniqueID, gitExec, checkout)
	if err != nil {
		return skerr.Fmt("Could not commit to %s: %s", CT_PERF_REPO, err)
	}

	// Step 2: Constructs a results struct in the format of https://github.com/google/skia-buildbot/blob/master/perf/FORMAT.md
	//         Ensure that the results file has as key the runID, groupName and the git hash.
	ctPerfData, err := convertCSVToBenchData(hash, groupName, runID, pathToCSVResults)
	if err != nil {
		return skerr.Fmt("Could not convert CSV from %s to BenchData: %s", pathToCSVResults, err)
	}

	// Step 3: Create JSON file from the ctPerfData struct.
	perfJson, err := json.MarshalIndent(ctPerfData, "", "  ")
	if err != nil {
		return skerr.Fmt("Could not convert %v to JSON: %s", ctPerfData, err)
	}
	jsonFile := path.Join(workdir, fmt.Sprintf("%s.json", uniqueID))
	if err := ioutil.WriteFile(jsonFile, perfJson, 0644); err != nil {
		return skerr.Fmt("Could not write to %s: %s", jsonFile, err)
	}

	// Step 4: Upload the results file to Google storage bucket CT_PERF_BUCKET for ingestion by ct-perf.skia.org.
	//         It is stored in location of this format: gs://<bucket>/<one or more dir names>/YYYY/MM/DD/HH/<zero or more dir names><some unique name>.json
	gsDir := path.Join("ingest", time.Now().UTC().Format("2006/01/02/15/"))
	if err := gs.UploadFileToBucket(filepath.Base(jsonFile), workdir, gsDir, CT_PERF_BUCKET); err != nil {
		return skerr.Fmt("Could not upload %s to gs://%s/%s: %s", jsonFile, CT_PERF_BUCKET, gsDir, err)
	}
	sklog.Infof("Successfully uploaded to gs://%s/%s/%s", CT_PERF_BUCKET, gsDir, filepath.Base(jsonFile))

	return nil
}

// commitToSyntheticRepo creates a file with the same name as the uniqueID and commits
// it into the specified repo. Returns the full hash of the commit.
func commitToSyntheticRepo(ctx context.Context, groupName, uniqueID, gitExec string, checkout *git.Checkout) (string, error) {
	// Create a new file using the uniqueID and commit it to the synthetic repo.
	if err := ioutil.WriteFile(filepath.Join(checkout.Dir(), uniqueID), []byte(uniqueID), 0644); err != nil {
		return "", skerr.Fmt("Failed to write %s: %s", uniqueID, err)
	}
	if msg, err := checkout.Git(ctx, "add", uniqueID); err != nil {
		return "", skerr.Fmt("Failed to add file %q: %s", msg, err)
	}
	output := bytes.Buffer{}
	cmd := exec.Command{
		Name:           gitExec,
		Args:           []string{"commit", "-m", fmt.Sprintf("Commit for %s by %s", groupName, uniqueID)},
		Dir:            checkout.Dir(),
		InheritEnv:     true,
		CombinedOutput: &output,
	}
	if err := exec.Run(ctx, &cmd); err != nil {
		return "", skerr.Fmt("Failed to commit updated file %q: %s", output.String(), err)
	}
	// Record the full hash to use as key in the results file.
	hashes, err := checkout.RevList(ctx, "HEAD", "-n1")
	if err != nil {
		return "", skerr.Fmt("Could not have full hash of %s: %s", checkout.Dir(), err)
	}
	hash := hashes[0]
	if msg, err := checkout.Git(ctx, "push", "origin", "master"); err != nil {
		return "", skerr.Fmt("Failed to push updated checkout %q: %s", msg, err)
	}
	return hash, nil
}

// Extend format.BenchData to include RunID.
type CTBenchData struct {
	*format.BenchData
	RunID string `json:"runID"`
}

// convertCSVToBenchData converts CT's output CSV into format.BenchData
// which will be used to ingest CT data into ct-perf.skia.org.
func convertCSVToBenchData(hash, groupName, runID, pathToCSVResults string) (*CTBenchData, error) {
	ctPerfData := &CTBenchData{
		&format.BenchData{
			Hash: hash,
			Key: map[string]string{
				"group_name": groupName,
			},
			Results: map[string]format.BenchResults{},
		},
		runID,
	}
	csvFile, err := os.Open(pathToCSVResults)
	defer util.Close(csvFile)
	reader := csv.NewReader(csvFile)
	reader.FieldsPerRecord = -1

	// Read and store the first line of headers.
	headers, err := reader.Read()
	if err != nil {
		return nil, skerr.Fmt("Could not read first line of %s: %s", pathToCSVResults, err)
	}
	// Below regex will be used to extract page name (without rank) and rank.
	rePageNameWithRank := regexp.MustCompile(`(.*) \(#([0-9]+)\)`)

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, skerr.Fmt("Error when reading CSV from %s: %s", pathToCSVResults, err)
		}

		pageNameNoRank := ""
		rank := -1
		benchResult := format.BenchResult{}
		for i := range headers {
			strLine := string(line[i])

			if headers[i] == "traceUrls" {
				// Strip out the Trace URLs since they have special treatment anyway. This will also make the JSON smaller.
				continue
			} else if headers[i] == "page_name" {
				matches := rePageNameWithRank.FindStringSubmatch(strLine)
				if len(matches) == 3 {
					pageNameNoRank = matches[1]
					rank, err = strconv.Atoi(matches[2])
					if err != nil {
						sklog.Warningf("Could not get rank out of %s: %s", strLine, err)
						continue
					}
				} else {
					pageNameNoRank = strLine
				}
			} else {
				f, err := strconv.ParseFloat(strLine, 64)
				if err != nil {
					sklog.Errorf("Couldn't parse %q as a float64: %s", strLine, err)
					continue
				}
				benchResult[headers[i]] = f
			}
		}

		if pageNameNoRank == "" {
			// We could not find the page_name. Do not add benchResult without it.
			continue
		}
		if rank != -1 {
			// Add page_rank as an option if it exists.
			benchResult["options"] = map[string]int{"page_rank": rank}
		}

		ctPerfData.Results[pageNameNoRank] = format.BenchResults{}
		ctPerfData.Results[pageNameNoRank]["default"] = benchResult
	}
	return ctPerfData, nil
}
