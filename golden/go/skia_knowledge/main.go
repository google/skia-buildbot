// Queries a Gold instance and downloads all positive images for each tests.
// Stores metadata in the meta.json output file.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Default query that will get all tests.
	DEFAULT_QUERY = "fdiffmax=-1&fref=false&frgbamax=255&frgbamin=0&head=true&include=false&limit=50&match=gamma_correct&match=name&metric=combined&neg=false&offset=0&pos=true&query=source_type%3Dgm&sort=desc&unt=false&nodiff=true"

	// Default URL for the Gold instance we want to query.
	GOLD_URL = "https://gold.skia.org"

	// Search endpoint of Gold.
	SEARCH_PATH = "/json/search"

	// Template for image file names. Generally the digest is filled in.
	IMG_FILENAME_TMPL = "%s.png"

	// Base path on Gold where images are stored.
	IMG_BASE_PATH = "/img/images/"

	// Name of the output metameta data file.
	META_DATA_FILE = "meta.json"
)

var (
	baseURL    = flag.String("base_url", GOLD_URL, "Query URL to retrieve meta data.")
	imgBaseURL = flag.String("img_url", GOLD_URL, "Url to retrieve the images.")
	outputDir  = flag.String("output_dir", "", "Directory where the Gold knowledge should be written.")
)

// TestRecord accumulates the images/digests generated for one test.
// This is the format of the meta.json file.
type TestRecord struct {
	TestName string             `json:"testName"`
	Digests  []*search.SRDigest `json:"digests"`
}

func main() {
	common.Init()

	// We need at least an output directory.
	if *outputDir == "" {
		sklog.Fatal("No output directory specified.")
	}

	// Set up the http client.
	client := &http.Client{}

	// load the test meta data from Gold.
	testRecords, err := loadMetaData(client, *baseURL, DEFAULT_QUERY, META_DATA_FILE)
	if err != nil {
		sklog.Fatalf("Error loading meta data: %s", err)
	}
	sklog.Infoln("Meta data loaded from Gold.")

	// Write the meta data to disk.
	if err := writeTestRecords(*outputDir, testRecords); err != nil {
		sklog.Fatalf("Error writing output file '%s': %s", filepath.Join(*outputDir, META_DATA_FILE), err)
	}
	sklog.Infoln("Meta data written to disk.")

	// Download the images referenced in the meta data.
	if err := downloadImages(*outputDir, *imgBaseURL, client, testRecords); err != nil {
		sklog.Fatalf("Error downloading images: %s", err)
	}
	sklog.Infof("Success. Knowledge data written to %s", *outputDir)
}

// loadMetaData makes a query to a Gold instance and parses the JSON response.
// It then groups images/digests by tests and returns them.
func loadMetaData(client *http.Client, baseURL, query, metaDataFileName string) ([]*TestRecord, error) {
	url := strings.TrimRight(baseURL, "/") + SEARCH_PATH + "?" + query
	sklog.Infof("Requesting url: %s", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)

	searchResp := &search.NewSearchResponse{}
	if err := json.NewDecoder(resp.Body).Decode(searchResp); err != nil {
		return nil, err
	}
	sklog.Infof("Meta data decoded successfully.")

	// Group the results by test.
	retMap := map[string]*TestRecord{}
	for _, oneDigest := range searchResp.Digests {
		testNameVal := oneDigest.ParamSet[types.PRIMARY_KEY_FIELD]
		if len(testNameVal) == 0 {
			sklog.Errorf("Error: Digest '%s' has no primaryKey in paramset", oneDigest.Digest)
			continue
		}

		testName := oneDigest.ParamSet[types.PRIMARY_KEY_FIELD][0]
		if found, ok := retMap[testName]; ok {
			found.Digests = append(found.Digests, oneDigest)
		} else {
			retMap[testName] = &TestRecord{
				TestName: testName,
				Digests:  []*search.SRDigest{oneDigest},
			}
		}
	}

	// Write the digests to disk.
	ret := make([]*TestRecord, 0, len(retMap))
	for _, oneTestRec := range retMap {
		ret = append(ret, oneTestRec)
	}

	return ret, nil
}

// writeTestRecords writes the retrieved information about tests to disk as JSON.
func writeTestRecords(outputDir string, testRecs []*TestRecord) error {
	f, err := os.Create(filepath.Join(outputDir, META_DATA_FILE))
	if err != nil {
		return err
	}
	defer util.Close(f)

	return json.NewEncoder(f).Encode(testRecs)
}

// downloadImages downloads all images referenced in the meta data to disk.
// One directory is created for each test.
func downloadImages(baseDir, baseURL string, client *http.Client, testRecs []*TestRecord) error {
	for _, testRec := range testRecs {
		testDir := filepath.Join(baseDir, testRec.TestName)
		absDirPath, err := fileutil.EnsureDirExists(testDir)
		if err != nil {
			sklog.Errorf("Error creating directory '%s'. Skipping. Got error: %s", testDir, err)
			continue
		}

		for _, digestRec := range testRec.Digests {
			// Download the image and then write it to disk.
			imgFileName := fmt.Sprintf(IMG_FILENAME_TMPL, digestRec.Digest)
			imgURL := strings.TrimRight(baseURL, "/") + IMG_BASE_PATH + imgFileName
			resp, err := client.Get(imgURL)
			if err != nil {
				sklog.Errorf("Error retrieving file '%s'. Got error: %s", imgURL, err)
				continue
			}
			defer util.Close(resp.Body)

			// Write the image to disk.
			fileName := filepath.Join(absDirPath, imgFileName)
			func(outputFileName string, reader io.Reader) {
				f, err := os.Create(outputFileName)
				if err != nil {
					sklog.Errorf("Error opening output file '%s'. Got error: %s", outputFileName, err)
					return
				}
				defer util.Close(f)

				if _, err := io.Copy(f, reader); err != nil {
					sklog.Errorf("Error saving file '%s'. Got error: %s", outputFileName, err)
					return
				}
				sklog.Infof("Downloaded %s sucessfully.", outputFileName)
			}(fileName, resp.Body)
		}
	}
	return nil
}
