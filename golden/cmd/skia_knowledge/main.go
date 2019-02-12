// Queries a Gold instance and downloads all positive images for each tests.
// Stores metadata in the meta.json output file.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/search"
)

const (
	// Default query that will get all tests.
	DEFAULT_QUERY = "fdiffmax=-1&fref=false&frgbamax=255&frgbamin=0&head=true&include=false&limit=50&match=name&metric=combined&neg=false&offset=0&pos=true&query=source_type%3Dgm&sort=desc&unt=false&nodiff=true"

	// Default URL for the Gold instance we want to query.
	GOLD_URL = "https://gold.skia.org"

	// Search endpoint of Gold.
	SEARCH_PATH = "/json/export"

	// Template for image file names. Generally the digest is filled in.
	IMG_FILENAME_TMPL = "%s.png"

	// Name of the output metameta data file.
	META_DATA_FILE = "meta.json"
)

var (
	baseURL   = flag.String("base_url", GOLD_URL, "Query URL to retrieve meta data.")
	indexFile = flag.String("index_file", "./"+META_DATA_FILE, "Path of the index file.")
	outputDir = flag.String("output_dir", "", "Directory where images should be written. If empty no images will be written.")
)

func main() {
	common.Init()

	// We need at least the index file or an output directory.
	if *outputDir == "" && *indexFile == "" {
		sklog.Fatal("No index file or output directory specified.")
	}

	// If the index file is empty write it to output directory.
	useIndexPath := *indexFile
	if useIndexPath == "" {
		useIndexPath = filepath.Join(*outputDir, META_DATA_FILE)
	}

	// Set up the http client.
	client := httputils.DefaultClientConfig().Client()

	// load the test meta data from Gold.
	testRecords, err := loadMetaData(client, *baseURL, DEFAULT_QUERY, META_DATA_FILE)
	if err != nil {
		sklog.Fatalf("Error loading meta data: %s", err)
	}
	sklog.Infoln("Meta data loaded from Gold.")

	// Write the index file to disk.
	if err := search.WriteExportTestRecordsFile(testRecords, useIndexPath); err != nil {
		sklog.Fatalf("Error writing index file: %s", err)
	}
	sklog.Infoln("Index file written to disk.")

	// If an output directory was given, download the images referenced in the index file.
	if *outputDir != "" {
		if err := downloadImages(*outputDir, client, testRecords); err != nil {
			sklog.Fatalf("Error downloading images: %s", err)
		}
	}

	sklog.Infof("Success. Knowledge data written to %s", *outputDir)
}

// loadMetaData makes a query to a Gold instance and parses the JSON response.
func loadMetaData(client *http.Client, baseURL, query, metaDataFileName string) ([]*search.ExportTestRecord, error) {
	url := strings.TrimRight(baseURL, "/") + SEARCH_PATH + "?" + query
	sklog.Infof("Requesting url: %s", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	return search.ReadExportTestRecords(resp.Body)
}

// downloadImages downloads all images referenced in the meta data to disk.
// One directory is created for each test.
func downloadImages(baseDir string, client *http.Client, testRecs []*search.ExportTestRecord) error {
	for _, testRec := range testRecs {
		testDir := filepath.Join(baseDir, testRec.TestName)
		absDirPath, err := fileutil.EnsureDirExists(testDir)
		if err != nil {
			sklog.Errorf("Error creating directory '%s'. Skipping. Got error: %s", testDir, err)
			continue
		}

		for _, digestRec := range testRec.Digests {
			// Download the image and then write it to disk.
			resp, err := client.Get(digestRec.URL)
			if err != nil {
				sklog.Errorf("Error retrieving file '%s'. Got error: %s", digestRec.URL, err)
				continue
			}
			defer util.Close(resp.Body)

			// Write the image to disk.
			imgFileName := fmt.Sprintf(IMG_FILENAME_TMPL, digestRec.Digest)
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
