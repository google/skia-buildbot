package ctseval

import (
	"net/http"
	"time"

	"go.skia.org/infra/go/sharedconfig"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/golden/go/tsuite"
)

const ctsTestRunIngesterID = "cts-run-ingester"

type CTSEvaluator struct {
	ingester *ingestion.Ingester
}

type CTSRun struct {
	TS          int64
	DeviceCount int
	PassCount   int
	FailCount   int
}

type CTSRunTestResult struct {
	Name   string
	Status tsuite.ResultStatus
	LogURL string
	ErrMsg string
}

type CTSRunDetails struct {
}

func NewCTSEvalator(gsBucket, gsDir, dataDir string, ingestFreq time.Duration, timeWindowDays int) (*CTSEvaluator, error) {
	// Create an ingester.
	var httpClient *http.Client = nil
	gsSource, err := ingestion.NewGoogleStorageSource("basename", gsBucket, gsDir, httpClient)
	if err != nil {
		return nil, err
	}

	ingesterConf := &sharedconfig.IngesterConfig{}
	processor, err := newCTSTestRunProcessor(dataDir)
	if err != nil {
		return nil, err
	}

	ingester, err := ingestion.NewIngester(ctsTestRunIngesterID, ingesterConf, nil, []ingestion.Source{gsSource}, processor)
	if err != nil {
		return nil, err
	}

	return &CTSEvaluator{
		ingester: ingester,
	}, nil
}

func (c *CTSEvaluator) List(offset, size int) ([]CTSRun, error) {
	return nil, nil
}

func (c *CTSEvaluator) Details(runID string) (*CTSRunDetails, error) {
	return &CTSRunDetails{}, nil
}

// Ingester:
//
//  * loads results from disk.
//  * does the diff
//  * stores the results on disk.
type ctsTestRunProcessor struct {
}

func newCTSTestRunProcessor(dataDir string) (ingestion.Processor, error) {
	return nil, nil
}
