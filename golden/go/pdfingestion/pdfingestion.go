package pdfingestion

// This packages implements an ingestion.Processor that
// rasterizes PDFs to PNG and writes them back to Google Storage.

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/pdf"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/goldingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"google.golang.org/api/option"
)

const (
	// Configuration options expected by the pdf processor.
	CONFIG_INPUT_IMAGES_BUCKET  = "InputImagesBucket"
	CONFIG_INPUT_IMAGES_DIR     = "InputImagesDir"
	CONFIG_OUTPUT_JSON_BUCKET   = "OutputJsonBucket"
	CONFIG_OUTPUT_JSON_DIR      = "OutputJsonDir"
	CONFIG_OUTPUT_IMAGES_BUCKET = "OutputImagesBucket"
	CONFIG_OUTPUT_IMAGES_DIR    = "OutputImagesDir"
	CONFIG_PDF_CACHEDIR         = "PdfCacheDir"

	PDF_EXT = "pdf"
	PNG_EXT = "png"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_PDF, newPDFProcessor)
}

// pdfProcessor ingests files and rasters PDFs to PNGs and writes them back
// to Google storage.
type pdfProcessor struct {
	client          *http.Client
	storageClient   *storage.Client
	rasterizers     []pdf.Rasterizer
	pdfCacheDir     string
	inImagesBucket  string
	inImagesDir     string
	outJsonBucket   string
	outJsonDir      string
	outImagesBucket string
	outImagesDir    string
}

// newPDFProcessor implements the ingestion.Constructor signature.
func newPDFProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, eventBus eventbus.EventBus) (ingestion.Processor, error) {
	// Parse the parameters right into the pdfProcessor instance.
	ret := &pdfProcessor{}
	err := combineErrors(
		ensureNonEmptyKey(config.ExtraParams, CONFIG_INPUT_IMAGES_BUCKET, &ret.inImagesBucket),
		ensureNonEmptyKey(config.ExtraParams, CONFIG_INPUT_IMAGES_DIR, &ret.inImagesDir),
		ensureNonEmptyKey(config.ExtraParams, CONFIG_OUTPUT_JSON_BUCKET, &ret.outJsonBucket),
		ensureNonEmptyKey(config.ExtraParams, CONFIG_OUTPUT_JSON_DIR, &ret.outJsonDir),
		ensureNonEmptyKey(config.ExtraParams, CONFIG_OUTPUT_IMAGES_BUCKET, &ret.outImagesBucket),
		ensureNonEmptyKey(config.ExtraParams, CONFIG_OUTPUT_IMAGES_DIR, &ret.outImagesDir))
	if err != nil {
		return nil, err
	}

	// Make sure there is a cachedir.
	ret.pdfCacheDir = config.ExtraParams[CONFIG_PDF_CACHEDIR]
	if ret.pdfCacheDir == "" {
		if ret.pdfCacheDir, err = ioutil.TempDir("", "pdfcache-dir"); err != nil {
			return nil, err
		}
	}

	// Create the storage service client.
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Failed to create a Google Storage API client: %s", err)
	}

	// Find rasterizers that have been enabled by the PDF packaged.
	rasterizers := pdf.GetEnabledRasterizers()
	if len(rasterizers) == 0 {
		return nil, fmt.Errorf("No rasterizers enabled.")
	}

	ret.client = client
	ret.storageClient = storageClient
	ret.rasterizers = rasterizers
	return ret, nil
}

// See ingestion.Processor interface.
func (p *pdfProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	dmResults, err := goldingestion.ParseDMResultsFromReader(r, resultsFile.Name())
	if err != nil {
		return err
	}

	// Get the results in this file that have produced a PDF.
	pdfResults := getPDFResults(dmResults)

	// If there are no PDF results we are done here.
	if len(pdfResults) == 0 {
		return nil
	}

	// Get the output name.
	outFile := resultsFile.Name()

	// check if they have been generated already (the JSON file exists)
	if p.resultExists(p.outJsonBucket, p.outJsonDir, outFile) {
		return nil
	}

	return p.rasterizeAndUpload(outFile, dmResults, pdfResults)
}

// See ingestion.Processor interface.
func (p *pdfProcessor) BatchFinished() error { return nil }

func (p *pdfProcessor) rasterizeAndUpload(dmResultName string, dmResults *goldingestion.DMResults, pdfResults []*jsonio.Result) error {
	processedResults := make([]*jsonio.Result, 0, len(pdfResults)*len(p.rasterizers))

	// Create a temporary directory to hold the rastered images.
	tempDir, err := ioutil.TempDir(p.pdfCacheDir, "pdfingestion")
	if err != nil {
		return err
	}
	defer util.RemoveAll(tempDir)

	// Go through all results that generated PDF files.
	for resultIdx, result := range pdfResults {
		// Fetch the PDF file if it's not in the cache.
		pdfFileName := fmt.Sprintf("%s.%s", result.Digest, PDF_EXT)
		pdfPath := filepath.Join(p.pdfCacheDir, pdfFileName)
		if !fileutil.FileExists(pdfPath) {
			if err = p.download(p.inImagesBucket, p.inImagesDir, pdfFileName, pdfPath); err != nil {
				sklog.Errorf("Unable to retrieve image: %s. Error: %s", pdfFileName, err)
				continue
			}
		}

		// Generate an image for each rasterizer.
		for rasterIdx, rasterizer := range p.rasterizers {
			tempName := filepath.Join(tempDir, fmt.Sprintf("rastering_%d_%d.%s", resultIdx, rasterIdx, PNG_EXT))
			err := rasterizer.Rasterize(pdfPath, tempName)
			if err != nil {
				sklog.Errorf("Rasterizing %s with %s failed: %s", filepath.Base(pdfPath), rasterizer.String(), err)
				continue
			}

			// Open the generated image and calculate the MD5.
			file, err := os.Open(tempName)
			if err != nil {
				sklog.Errorf("Unable to open generated image: %s", err)
				continue
			}

			var buf bytes.Buffer
			md5, err := util.MD5FromReader(file, &buf)
			if err != nil {
				sklog.Errorf("Unable to calculate MD5 hash of file %s. Got error: %s", tempName, err)
				continue
			}
			digest := hex.EncodeToString(md5)
			uploadFileName := fmt.Sprintf("%s.%s", digest, PNG_EXT)
			if err := p.upload(p.outImagesBucket, p.outImagesDir, uploadFileName, bytes.NewBuffer(buf.Bytes())); err != nil {
				sklog.Errorf("Unable to upload file %s. Error: %s", uploadFileName, err)
				continue
			}

			// Update the result and add it to the successfully processed results.
			result.Key["rasterizer"] = rasterizer.String()
			result.Digest = digest
			result.Options["ext"] = PNG_EXT
			processedResults = append(processedResults, result)
		}
	}

	// If we have no processed results we consider it an error.
	if len(processedResults) == 0 {
		return fmt.Errorf("No input image was processed successfully.")
	}

	// Replace the old results in the original result and write it to the cloud.
	dmResults.Results = processedResults
	jsonBytes, err := json.MarshalIndent(dmResults, "", "    ")
	if err != nil {
		return fmt.Errorf("Unable to encode JSON: %s", err)
	}

	return p.upload(p.outJsonBucket, p.outJsonDir, dmResultName, bytes.NewBuffer(jsonBytes))
}

// resultExists checks if the given file in the given bucket and directory exists in GCS.
func (p *pdfProcessor) resultExists(bucket, dir, fileName string) bool {
	objPath := dir + "/" + fileName
	_, err := p.storageClient.Bucket(bucket).Object(objPath).Attrs(context.Background())
	if err != nil {
		return false
	}
	return true
}

// upload stores the content of the given reader in bucket/dir/fileName in GCS. It will
// not upload the file if it already exists.
func (p *pdfProcessor) upload(bucket, dir, fileName string, r io.Reader) error {
	objectPath := dir + "/" + fileName
	w := p.storageClient.Bucket(bucket).Object(objectPath).NewWriter(context.Background())
	defer util.Close(w)
	_, err := io.Copy(w, r)
	return err
}

// download fetches the content of the given location in GCS and stores it at the given
// output path.
func (p *pdfProcessor) download(bucket, dir, fileName, outputPath string) error {
	objectPath := dir + "/" + fileName
	r, err := p.storageClient.Bucket(bucket).Object(objectPath).NewReader(context.Background())
	if err != nil {
		return err
	}
	defer util.Close(r)

	tempFile, err := ioutil.TempFile(p.pdfCacheDir, "pdfingestion-download")
	if err != nil {
		return err
	}

	_, err = io.Copy(tempFile, r)
	util.Close(tempFile)
	if err != nil {
		util.Remove(tempFile.Name())
		return err
	}

	if err := os.Rename(tempFile.Name(), outputPath); err != nil {
		return err
	}

	sklog.Infof("Downloaded: %s/%s", bucket, objectPath)
	return nil
}

func getPDFResults(dmResults *goldingestion.DMResults) []*jsonio.Result {
	ret := make([]*jsonio.Result, 0, len(dmResults.Results))
	for _, result := range dmResults.Results {
		if result.Options["ext"] == "pdf" {
			ret = append(ret, result)
		}
	}
	return ret
}

// combineErrors combines all non-nil errors in errs into a single error.
// If all of them are nil it returns nil.
func combineErrors(errs ...error) error {
	var buf bytes.Buffer
	for _, err := range errs {
		if err != nil {
			_, _ = buf.WriteString(err.Error() + "\n")
		}
	}
	combinedErrors := buf.String()
	if combinedErrors != "" {
		return fmt.Errorf(combinedErrors)
	}
	return nil
}

// ensureNonEmptyKey checks the given parameters for the given key and writes the
// result to target. If the value is empty it will return an error.
func ensureNonEmptyKey(params map[string]string, key string, target *string) error {
	*target = params[key]
	if *target == "" {
		return fmt.Errorf("Key %s cannot be empty.", key)
	}
	return nil
}
