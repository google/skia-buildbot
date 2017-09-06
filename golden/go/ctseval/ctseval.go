package ctseval

import (
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search"

	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/golden/go/tsuite"
)

const ctsTestRunIngesterID = "cts-run-ingester"

type CTSEvaluator struct {
	ingester  *ingestion.Ingester
	dataDir   string
	diffStore diff.DiffStore
}

type CTSRun struct {
	TS          int64
	DeviceCount int
	PassCount   int
	FailCount   int
}

type CTSRunDetails struct {
	*CTSRun
	Devices []Device
}

type Device struct {
	ID      string
	Name    string
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

func newCTSEvalator(dataDir string, sources []ingestion.Source, ingestFreq time.Duration, timeWindowDays int, diffStore diff.DiffStore) (*CTSEvaluator, error) {
	ret := &CTSEvaluator{
		dataDir: dataDir,
	}

	// Create an ingester.
	ingesterConf := &sharedconfig.IngesterConfig{}

	var err error
	ret.ingester, err = ingestion.NewIngester(ctsTestRunIngesterID, ingesterConf, nil, sources, ret)
	if err != nil {
		return nil, err
	}

	// Start the ingester and return the result.
	ret.ingester.Start()
	return ret, nil
}

func (c *CTSEvaluator) List(offset, size int) ([]CTSRun, error) {
	return nil, nil
}

func (c *CTSEvaluator) Details(runID string) (*CTSRunDetails, error) {
	return &CTSRunDetails{}, nil
}

func (c *CTSEvaluator) Process(resultsFile ingestion.ResultFileLocation) error {
	// Only consider files that are called meta.json.
	name := resultsFile.Name()
	if !strings.HasSuffix(name, "/result.json") {
		return ingestion.IgnoreResultsFileErr
	}

	readCloser, err := resultsFile.Open()
	if err != nil {
		return err
	}
	defer util.Close(readCloser)

	suiteResult, err := tsuite.LoadSuiteResults(readCloser)
	if err != nil {
		return err
	}

	gsDir, _ := filepath.Split(name)

	return nil
}
