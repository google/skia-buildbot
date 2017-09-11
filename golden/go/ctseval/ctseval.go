package ctseval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/types"

	gstorage "cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tsuite"
)

const (
	ctsTestRunIngesterID = "cts-run-ingester"

	MetaDataFileName = "meta.json"
	ResultFileName   = "result.json"

	devPathTmpl = "%s-%s-en-portrait"
)

type CTSEvaluator struct {
	bucket        string
	storageClient *gstorage.Client
	ingester      *ingestion.Ingester
	dataDir       string
	diffStore     diff.DiffStore
	ixr           *indexer.Indexer
	storages      *storage.Storage
}

type CTSRun struct {
	ID          string
	TS          int64
	DeviceCount int
	PassCount   int
	FailCount   int
}

type CTSRunDetails struct {
	*CTSRun
	Devices []*Device
}

type Device struct {
	*tsuite.FirebaseDevice
	Results []*CTSTestResult
}

type CTSTestResult struct {
	*tsuite.TestResult
	LogURL string
	ImgID  string
	Diff   *search.SRDiffDigest
}

// var httpClient *http.Client = nil
// gsSource, err := ingestion.NewGoogleStorageSource("basename", gsBucket, gsDir, httpClient)
// if err != nil {
// 	return nil, err
// }

func NewCTSEvalator(bucket, dataDir string, httpClient *http.Client, sources []ingestion.Source, ingestFreq time.Duration, timeWindowDays int, ixr *indexer.Indexer, diffStore diff.DiffStore, storages *storage.Storage) (*CTSEvaluator, error) {
	storageClient, err := gstorage.NewClient(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	ret := &CTSEvaluator{
		bucket:        bucket,
		storageClient: storageClient,
		dataDir:       dataDir,
		diffStore:     diffStore,
	}

	// Create an ingester.
	ingesterConf := &sharedconfig.IngesterConfig{}

	ret.ingester, err = ingestion.NewIngester(ctsTestRunIngesterID, ingesterConf, nil, sources, ret)
	if err != nil {
		return nil, err
	}

	// Start the ingester and return the result.
	ret.ingester.Start()
	return ret, nil
}

func (c *CTSEvaluator) List(offset, size int) ([]CTSRun, error) {
	// List the directory.

	// Sort the result.

	// Return the request part of the slize.
	return nil, nil
}

func (c *CTSEvaluator) Details(runID string) (*CTSRunDetails, error) {
	return &CTSRunDetails{}, nil
}

func (c *CTSEvaluator) Process(resultsFile ingestion.ResultFileLocation) error {
	// Only consider files that are called meta.json.
	name := resultsFile.Name()
	if !strings.HasSuffix(name, "/"+MetaDataFileName) {
		return ingestion.IgnoreResultsFileErr
	}

	readCloser, err := resultsFile.Open()
	if err != nil {
		return err
	}
	defer util.Close(readCloser)

	// Load the meta data.
	metaData, err := tsuite.LoadTestRunMeta(readCloser)
	if err != nil {
		return err
	}

	devCount := len(metaData.Devices)
	result := &CTSRunDetails{
		CTSRun: &CTSRun{
			ID:          metaData.ID,
			TS:          metaData.TS,
			DeviceCount: devCount,
		},
		Devices: make([]*Device, devCount, devCount),
	}

	// Get the list of paths with the individual device results.
	gsDir, _ := filepath.Split(name)
	// devicePaths := make([]string, 0, len(metaData.Devices))
	for idx, dev := range metaData.Devices {
		path := fmt.Sprintf("%s/"+devPathTmpl, gsDir, dev.ID, dev.VersionIDs)
		resultDev := &Device{FirebaseDevice: dev}
		if err := c.processOneDevice(path, resultDev); err != nil {
			return err
		}
		result.Devices[idx] = resultDev
	}

	return c.addResult(result)
}

func (c *CTSEvaluator) addResult(runDetails *CTSRunDetails) error {
	idx := c.ixr.GetIndex()
	tallies := idx.TalliesByTest(false)
	exps, err := c.storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	for _, dev := range runDetails.Devices {
		for _, runResult := range dev.Results {
			if runResult.ErrorMsg == "" {
				testName := runResult.Name
				if _, ok := tallies[testName]; !ok {
					runResult.ErrorMsg = fmt.Sprintf("Unknown test: %s", testName)
					continue
				}

				digests := make([]string, 0, len(tallies[testName]))
				for oneDigest := range tallies[testName] {
					if exps.Tests[testName][oneDigest] == types.POSITIVE {
						digests = append(digests, oneDigest)
					}
				}
				diffs, err := c.diffStore.Get(diff.PRIORITY_NOW, runResult.ImgID, digests)
				if err != nil {
					return err
				}

				diffList := make([]*search.SRDiffDigest, 0, len(diffs))
				for digest, metric := range diffs {
					diffList = append(diffList, &search.SRDiffDigest{
						Test:        testName,
						Digest:      digest,
						DiffMetrics: metric.(*diff.DiffMetrics),
						ParamSet:    idx.GetParamsetSummary(testName, digest, false),
					})
				}
			}
		}
	}

	fName := filepath.Join(c.dataDir, runDetails.CTSRun.ID)
	f, err := os.Create(fName)
	if err != nil {
		return err
	}

	return json.NewEncoder(f).Encode(runDetails)
}

func (c *CTSEvaluator) processOneDevice(dirPath string, dev *Device) error {
	resultPath := dirPath + "/" + ResultFileName

	r, err := c.storageClient.Bucket(c.bucket).Object(resultPath).NewReader(context.Background())
	if err != nil {
		return err
	}
	defer util.Close(r)

	suiteResult, err := tsuite.LoadSuiteResults(r)
	if err != nil {
		return err
	}

	nResults := len(suiteResult.Results)
	dev.Results = make([]*CTSTestResult, nResults, nResults)
	for idx, testRun := range suiteResult.Results {
		dev.Results[idx] = &CTSTestResult{
			TestResult: testRun,
			LogURL:     dirPath + "/",
			ImgID:      dirPath + "/" + testRun.Name,
		}
	}

	return nil
}
