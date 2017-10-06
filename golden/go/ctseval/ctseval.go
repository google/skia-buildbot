package ctseval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/paramtools"

	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/fileutil"

	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"

	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/indexer"

	gstorage "cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
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
	TestRunIDPrefix  = "testrun-"

	devPathTmpl = "%s/%s-%s-en-portrait/artifacts"

	cIngesterID  = "cts-ingester"
	INGESTER_DIR = "cts-ingester"
	RESULT_DIR   = "cts-results"
)

type CTSEvaluator struct {
	bucket        string
	storageClient *gstorage.Client
	ingester      *ingestion.Ingester
	dataDir       string
	ixr           *indexer.Indexer
	storages      *storage.Storage

	mutex          sync.Mutex
	knowledgeBytes []byte
}

// CTSRun captures the high level details of one test run. A test run
// can comprise multiple devices and multiple versions for each device.
type CTSRun struct {
	ID           string `json:"id"`
	TS           int64  `json:"ts"`
	DeviceCount  int    `json:"deviceCount"`
	PassCount    int    `json:"passCount"`
	FailCount    int    `json:"failCount"`
	MetaDataFile string `json:"metaDataFile"`
}

// CTSRunDetails extends CTSRun to capture the details of the test run.
type CTSRunDetails struct {
	*CTSRun
	Devices []*Device `json:"devices"`
}

// Device captures the results for a testrun on one
type Device struct {
	*tsuite.FirebaseDevice
	VersionID        string           `json:"versionID"`
	Results          []*CTSTestResult `json:"results"`
	DevProcessingErr string           `json:"devProcessingErr"`
	PassCount        int              `json:"passCount"`
}

// CTSTestResult captures the process results of one device.
type CTSTestResult struct {
	*tsuite.TestResult
	LogURL  string `json:"logURL"`
	ImgPath string `json:"imgPath"`
	ImgID   string `json:"imgID"`
	// Diff          *search.SRDiffDigest `json:"diff"`
	// ClosestMetric string               `json:"closestMetric"`
	Diffs          []*search.SRDiffDigest `json:"diffs"`
	ProcessingErr  string                 `json:"processingErr"`
	ClosestDiffIdx int                    `json:"closestDiffIdx"`
}

func NewCTSEvalator(gsBucket, gsPath, baseDir string, httpClient *http.Client, ingestFreq time.Duration, timeWindowDays int, ixr *indexer.Indexer, storages *storage.Storage) (*CTSEvaluator, error) {
	ingesterDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, INGESTER_DIR)))
	dataDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, RESULT_DIR)))

	storageClient, err := gstorage.NewClient(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	gsSource, err := ingestion.NewGoogleStorageSource(cIngesterID, gsBucket, gsPath, httpClient)
	if err != nil {
		return nil, err
	}

	ret := &CTSEvaluator{
		bucket:        gsBucket,
		storageClient: storageClient,
		dataDir:       dataDir,
		ixr:           ixr,
		storages:      storages,
	}

	// Create an ingester.
	ingesterConf := &sharedconfig.IngesterConfig{
		RunEvery:  config.Duration{Duration: ingestFreq},
		MinDays:   timeWindowDays,
		StatusDir: ingesterDir,
	}

	sources := []ingestion.Source{gsSource}
	ret.ingester, err = ingestion.NewIngester(ctsTestRunIngesterID, ingesterConf, nil, nil, sources, ret)
	if err != nil {
		return nil, err
	}

	// Start the knowledge builder.
	ret.startKnowledgeBuilder()

	if ingestFreq != 0 {
		// Start the ingester and return the result.
		ret.ingester.Start()
	}
	return ret, nil
}

func (c *CTSEvaluator) List(offset, size int) ([]*CTSRun, error) {
	// List the directory.
	files, err := ioutil.ReadDir(c.dataDir)
	if err != nil {
		return nil, err
	}

	fNames := make([]string, 0, len(files))
	for _, oneFile := range files {
		if strings.HasPrefix(oneFile.Name(), TestRunIDPrefix) {
			fNames = append(fNames, oneFile.Name())
		}
	}

	// Sort the file names in reverse order.
	sort.Sort(sort.Reverse(sort.StringSlice(fNames)))

	// Load the desired results. Throw away the details.
	offset = util.MinInt(offset, len(fNames))
	size = util.MinInt(len(fNames)-offset, size)
	fNames = fNames[offset : offset+size]
	ret := make([]*CTSRun, 0, len(fNames))
	for _, oneResult := range c.getResults(fNames) {
		ret = append(ret, oneResult.CTSRun)
	}

	// Return the request part of the slize.
	return ret, nil
}

// Details returns the details about the specified test run.
func (c *CTSEvaluator) Details(runID string) (*CTSRunDetails, error) {
	fullPath := filepath.Join(c.dataDir, runID)
	if !strings.HasPrefix(runID, TestRunIDPrefix) || strings.Contains(runID, "/") || !fileutil.FileExists(fullPath) {
		return nil, fmt.Errorf("Unknown runID: %s", runID)
	}
	return c.getRunDetails(fullPath)
}

const GCS_SCHEME = "gs://"

func getGCSPath(fName string) string {
	if !strings.HasPrefix(fName, GCS_SCHEME) {
		return ""
	}

	bucketPath := strings.SplitN(fName[len(GCS_SCHEME):], "/", 2)
	if len(bucketPath) != 2 {
		return ""
	}
	return bucketPath[1]
}

// Process processes one test run.
func (c *CTSEvaluator) Process(resultsFile ingestion.ResultFileLocation) error {
	// Only consider files that are called meta.json.
	name := getGCSPath(resultsFile.Name())
	sklog.Infof("Input file path: %s", name)
	if !strings.HasSuffix(name, "/"+MetaDataFileName) {
		return ingestion.IgnoreResultsFileErr
	}

	readCloser, err := resultsFile.Open()
	if err != nil {
		return err
	}
	defer util.Close(readCloser)
	sklog.Infof("Processing test run: %s", name)

	// Load the meta data.
	metaData, err := tsuite.LoadTestRunMeta(readCloser)
	if err != nil {
		return err
	}

	// sklog.Infof("Metadata: %s", spew.Sdump(metaData))

	devCount := len(metaData.Devices)
	result := &CTSRunDetails{
		CTSRun: &CTSRun{
			ID:           metaData.ID,
			TS:           metaData.TS,
			DeviceCount:  devCount,
			MetaDataFile: name,
		},
		Devices: make([]*Device, 0, devCount),
	}

	// Get the list of paths with the individual device results.
	gsDir, _ := filepath.Split(name)
	gsDir = strings.TrimRight(gsDir, "/")

	// TODO(stephana): remove this fallback.
	if result.CTSRun.ID == "" {
		temp := strings.Split(gsDir, "/")
		result.CTSRun.ID = temp[len(temp)-1]
	}

	sklog.Infof("gsdir: %s", gsDir)
	for _, dev := range metaData.Devices {
		for _, versionID := range dev.Versions {
			// Derive the result directory for the device and version and process it.
			path := fmt.Sprintf(devPathTmpl, gsDir, dev.Device.ID, versionID)
			resultDev := &Device{FirebaseDevice: dev.Device, VersionID: versionID}
			if err := c.processOneDevice(path, resultDev); err != nil {
				resultDev.DevProcessingErr = err.Error()
				sklog.Errorf("Error processing device %s (version %s): %s", dev.Device.Name, versionID, err)
			}
			result.Devices = append(result.Devices, resultDev)
		}
	}

	// Get the current search index and the expectations.
	idx := c.ixr.GetIndex()
	exps, err := c.storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	// Save the result for later retrieval.
	if err := c.addResult(result, idx, exps); err != nil {
		return err
	}

	sklog.Infof("Test run %s processed sucessfuly.", result.ID)
	return nil
}

// processOneDevice processes the result of a device-version pair which is stored
// in the given dirPath in GCS. The results will be added to dev.
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
		gsPath := dirPath + "/" + testRun.Name
		imgID := diffstore.GCSPathToImageID(c.bucket, gsPath)
		dev.Results[idx] = &CTSTestResult{
			TestResult: testRun,
			LogURL:     dirPath + "/",
			ImgPath:    c.bucket + "/" + gsPath + "." + diffstore.IMG_EXTENSION,
			ImgID:      imgID,
		}
	}
	sklog.Infof("Device %s (version: %s) processed sucessfully.", dev.Name, dev.VersionID)
	return nil
}

func (c *CTSEvaluator) addResult(runDetails *CTSRunDetails, idx *indexer.SearchIndex, exps *expstorage.Expectations) error {
	// Write the entire test run to  disk even if the results were not calculated correctly.
	fName := filepath.Join(c.dataDir, runDetails.CTSRun.ID)
	sklog.Infof("OUTPUT FILENAME: %s", fName)
	return util.WithWriteFile(fName, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(runDetails)
	})
}

func (c *CTSEvaluator) calculateDiffs(runDetails *CTSRunDetails, idx *indexer.SearchIndex, exps *expstorage.Expectations) {
	// Keep track if things change.
	changed := true

	// Get the tallies to extract digests by test name.
	tallies := idx.TalliesByTest(false)
	for _, dev := range runDetails.Devices {
		devChanged := false
		for _, runResult := range dev.Results {
			// Only caclculate if we don't have values yet.
			if runResult.Diffs != nil {
				continue
			}

			// Ignore tests that failed on the device.
			if runResult.ErrorMsg != "" {
				sklog.Infof("Skipping test '%s' because it has error: %s", runResult.Name, runResult.ErrorMsg)
				continue
			}

			// Find the test we are interested in.
			testName := runResult.Name
			if _, ok := tallies[testName]; !ok {
				runResult.ProcessingErr = fmt.Sprintf("Processing error: Unknown test: %s", testName)
				continue
			}

			// Get all positive digests for the test.
			digests := make([]string, 0, len(tallies[testName]))
			for oneDigest := range tallies[testName] {
				if exps.Tests[testName][oneDigest] == types.POSITIVE {
					digests = append(digests, oneDigest)
				}
			}

			// Calculate the diffs.
			diffs, err := c.storages.DiffStore.Get(diff.PRIORITY_NOW, runResult.ImgID, digests)
			if err != nil {
				runResult.ProcessingErr = fmt.Sprintf("Unable to diff %s (%s) against known digests.", runResult.ImgPath, runResult.ImgID)
			}

			// Add the diffs to the run result.
			diffList := make([]*search.SRDiffDigest, 0, len(diffs))
			for digest, metric := range diffs {
				diffList = append(diffList, &search.SRDiffDigest{
					Test:        testName,
					Digest:      digest,
					DiffMetrics: metric.(*diff.DiffMetrics),
					ParamSet:    idx.GetParamsetSummary(testName, digest, false),
				})
			}
			runResult.Diffs = diffList
			runResult.ClosestDiffIdx = closestDiffIndex(diffList, diff.METRIC_COMBINED)
			devChanged = true
		}

		if devChanged {
			dev.summarize()
		}

		changed = changed && devChanged
	}

	if changed {
		runDetails.summarize()
	}
}

// getResults loads the given files from the data dir. If a file cannot be
// opened, an error will be logged, but the other files are still loaded.
func (c *CTSEvaluator) getResults(fNames []string) []*CTSRunDetails {
	ret := make([]*CTSRunDetails, 0, len(fNames))
	for _, fName := range fNames {
		fullPath := filepath.Join(c.dataDir, fName)
		runResult, err := c.getRunDetails(fullPath)

		// Log an error, but don't return it.
		if err != nil {
			sklog.Errorf("Error reading %s: %s", fullPath, err)
			continue
		}
		ret = append(ret, runResult)
	}

	return ret
}

// getRunDetails loads a single run result from disk.
func (c *CTSEvaluator) getRunDetails(path string) (*CTSRunDetails, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(file)
	runDetails := &CTSRunDetails{}
	if err := json.NewDecoder(file).Decode(runDetails); err != nil {
		return nil, err
	}

	// Recalc the results if necessary.
	idx := c.ixr.GetIndex()
	exps, err := c.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, err
	}
	c.calculateDiffs(runDetails, idx, exps)

	return runDetails, nil
}

func (c *CTSEvaluator) startKnowledgeBuilder() {
	go func() {
		filterQuery := paramtools.Params(map[string]string{
			types.CORPUS_FIELD: "gm",
			"os":               "Android",
			"model":            "PixelXL",
		})

		util.Repeat(15*time.Minute, nil, func() {
			sklog.Infof("Starting to build knowledge package.")
			idx := c.ixr.GetIndex()
			// talliesByTest := idx.TalliesByTest(false)
			suite := tsuite.New()

			paramSetsByTest := idx.GetParamsetSummaryByTest(false)

			for testName, digestsParamSets := range paramSetsByTest {
				// Iterate other the digests and their param sets.
				for _, params := range digestsParamSets {
					// Make sure one of the digests has params that match the filter.
					if params.Matches(filterQuery) {
						suite.Add(testName, tsuite.NewMemorizer())
						sklog.Infof("Added %s", testName)
						break
					}
				}
			}

			var buf bytes.Buffer
			if err := suite.Save(&buf); err != nil {
				sklog.Errorf("Error writing knowledge serialization: %s", err)
				return
			}

			c.mutex.Lock()
			defer c.mutex.Unlock()
			c.knowledgeBytes = buf.Bytes()
			sklog.Infof("Done building knowledge package. Length:%d", len(c.knowledgeBytes))
		})
	}()
}

func (c *CTSEvaluator) KnowledgeZip() []byte {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.knowledgeBytes
}

func (d *Device) summarize() {
	d.PassCount = 0
	for _, result := range d.Results {
		if (result.ClosestDiffIdx >= 0) && (result.Diffs[result.ClosestDiffIdx].Diffs[diff.METRIC_COMBINED] == 0) {
			d.PassCount++
		}
	}
}

func (c *CTSRunDetails) summarize() {
	c.PassCount = 0
	for _, dev := range c.Devices {
		if dev.PassCount == len(dev.Results) {
			c.PassCount++
		}
	}
	c.DeviceCount = len(c.Devices)
	c.FailCount = c.DeviceCount - c.PassCount
}

// closestDiffIndex finds the index of the diff that's the closest.
func closestDiffIndex(diffs []*search.SRDiffDigest, metric string) int {
	if len(diffs) == 0 {
		return -1
	}

	minIdx := 0
	for i := 1; i < len(diffs); i++ {
		if diffs[i].Diffs[metric] < diffs[minIdx].Diffs[metric] {
			minIdx = i
		}
	}
	return minIdx
}
