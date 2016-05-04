// Application that downloads PDFs and then captures SKPs from them.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to download PDFs and capture their SKPs.
	WORKER_POOL_SIZE = 20
)

var (
	workerNum      = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType    = flag.String("pageset_type", util.PAGESET_TYPE_PDF_1m, "The type of pagesets to use for this run. Eg: PDF1m.")
	chromiumBuild  = flag.String("chromium_build", "", "The specified chromium build. This value is used to find the pdfium_test binary from Google Storage and while uploading the PDFs and SKPs to Google Storage.")
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	if !*worker_common.Local {
		defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Capturing SKPs from PDFs")
	defer glog.Flush()

	// Validate required arguments.
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Error(err)
		return
	}

	// Download PDF pagesets if they do not exist locally.
	if err := gs.DownloadWorkerArtifacts(util.PAGESETS_DIR_NAME, *pagesetType, *workerNum); err != nil {
		glog.Error(err)
		return
	}
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)

	// Create the dir that PDFs will be stored in.
	pathToPdfs := filepath.Join(util.PdfsDir, *pagesetType, *chromiumBuild)
	// Delete and remake the local PDFs directory.
	skutil.RemoveAll(pathToPdfs)
	skutil.MkdirAll(pathToPdfs, 0700)
	// Cleanup the dir after the task is done.
	defer skutil.RemoveAll(pathToPdfs)

	// Create the dir that SKPs will be stored in.
	pathToSkps := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)
	// Delete and remake the local SKPs directory.
	skutil.RemoveAll(pathToSkps)
	skutil.MkdirAll(pathToSkps, 0700)
	// Cleanup the dir after the task is done.
	defer skutil.RemoveAll(pathToSkps)

	// Copy over the pdfium_test binary to this slave.
	pdfiumLocalPath := filepath.Join(os.TempDir(), util.BINARY_PDFIUM_TEST)
	pdfiumRemotePath := filepath.Join(util.BinariesDir, *chromiumBuild, util.BINARY_PDFIUM_TEST)
	respBody, err := gs.GetRemoteFileContents(pdfiumRemotePath)
	if err != nil {
		glog.Errorf("Could not fetch %s: %s", pdfiumRemotePath, err)
		return
	}
	defer skutil.Close(respBody)
	out, err := os.Create(pdfiumLocalPath)
	if err != nil {
		glog.Errorf("Unable to create file %s: %s", pdfiumLocalPath, err)
		return
	}
	defer skutil.Remove(pdfiumLocalPath)
	if _, err = io.Copy(out, respBody); err != nil {
		glog.Error(err)
		return
	}
	skutil.Close(out)
	// Downloaded pdfium_test binary needs to be set as an executable.
	skutil.LogErr(os.Chmod(pdfiumLocalPath, 0777))

	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CaptureSKPsTimeoutSecs
	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		glog.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
		return
	}

	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := util.GetClosedChannelOfPagesets(fileInfos)

	var wg sync.WaitGroup

	// Gather PDFs and SKPs with errors.
	erroredPDFs := []string{}
	erroredSKPs := []string{}

	// Loop through workers in the worker pool.
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures SKPs.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for pagesetName := range pagesetRequests {

				// Read the pageset.
				pagesetPath := filepath.Join(pathToPagesets, pagesetName)
				decodedPageset, err := util.ReadPageset(pagesetPath)
				if err != nil {
					glog.Errorf("Could not read %s: %s", pagesetPath, err)
					continue
				}

				glog.Infof("===== Processing %s =====", pagesetPath)

				if strings.Contains(decodedPageset.UrlsList, ",") {
					glog.Errorf("capture_skps_from_pdfs does not support multiple URLs in pagesets. Found in pageset %s", pagesetPath)
					continue
				}

				skutil.LogErr(os.Chdir(pathToPdfs))

				// Download the PDFs.
				pdfBase, err := getPdfFileName(decodedPageset.UrlsList)
				if err != nil {
					glog.Errorf("Could not parse the URL %s to get a PDF file name: %s", decodedPageset.UrlsList, err)
					erroredPDFs = append(erroredPDFs, decodedPageset.UrlsList)
					continue
				}
				pdfPath := filepath.Join(pathToPdfs, pdfBase)
				wgetArgs := []string{
					"-O", pdfPath,
					decodedPageset.UrlsList,
				}
				if err := util.ExecuteCmd(util.BINARY_WGET, wgetArgs, []string{}, time.Duration(timeoutSecs)*time.Second, nil, nil); err != nil {
					glog.Errorf("Could not wget %s: %s", decodedPageset.UrlsList, err)
					erroredPDFs = append(erroredPDFs, decodedPageset.UrlsList)
					continue
				}

				// Run pdfium_test to create SKPs from the PDFs.
				pdfiumTestArgs := []string{
					"--skp", pdfPath,
				}
				if err := util.ExecuteCmd(pdfiumLocalPath, pdfiumTestArgs, []string{}, time.Duration(timeoutSecs)*time.Second, nil, nil); err != nil {
					glog.Errorf("Could not run pdfium_test on %s: %s", pdfPath, err)
					erroredSKPs = append(erroredSKPs, pdfBase)
					continue
				}

				// Move generated SKPs into the pathToSKPs directory.
				skps, err := filepath.Glob(path.Join(pathToPdfs, fmt.Sprintf("%s.*.skp", pdfBase)))
				if err != nil {
					glog.Errorf("Found no SKPs for %s: %s", pdfBase, err)
					erroredSKPs = append(erroredSKPs, pdfBase)
					continue
				}
				for _, skp := range skps {
					skpBasename := path.Base(skp)
					dest := path.Join(pathToSkps, skpBasename)
					if err := os.Rename(skp, dest); err != nil {
						glog.Errorf("Could not move %s to %s: %s", skp, dest, err)
						continue
					}
				}
			}
		}()
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// Check to see if there is anything in the pathToPDFs and pathToSKPs dirs.
	pdfsEmpty, err := skutil.IsDirEmpty(pathToPdfs)
	if err != nil {
		glog.Error(err)
		return
	}
	if pdfsEmpty {
		glog.Errorf("Could not download any PDF in %s", pathToPdfs)
		return
	}
	skpsEmpty, err := skutil.IsDirEmpty(pathToSkps)
	if err != nil {
		glog.Error(err)
		return
	}
	if skpsEmpty {
		glog.Errorf("Could not create any SKP in %s", pathToSkps)
		return
	}

	// Move and validate all SKP files.
	if err := util.ValidateSKPs(pathToSkps); err != nil {
		glog.Error(err)
		return
	}

	// Write timestamp to the PDFs dir.
	skutil.LogErr(util.CreateTimestampFile(pathToPdfs))
	// Write timestamp to the SKPs dir.
	skutil.LogErr(util.CreateTimestampFile(pathToSkps))

	// Upload PDFs dir to Google Storage.
	if err := gs.UploadWorkerArtifacts(util.PDFS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild), *workerNum); err != nil {
		glog.Error(err)
		return
	}
	// Upload SKPs dir to Google Storage.
	if err := gs.UploadWorkerArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild), *workerNum); err != nil {
		glog.Error(err)
		return
	}

	// Summarize errors.
	if len(erroredPDFs) > 0 {
		glog.Error("The Following URLs could not be downloaded as PDFs:")
		for _, erroredPDF := range erroredPDFs {
			glog.Errorf("\t%s", erroredPDF)
		}
	}
	if len(erroredSKPs) > 0 {
		glog.Error("The Following PDFs could not be converted to SKPs:")
		for _, erroredSKP := range erroredSKPs {
			glog.Errorf("\t%s", erroredSKP)
		}
	}
}

// getPdfFileName constructs a name for the locally stored PDF file from the URL.
// It strips out all "/" and replaces them with double underscores. Having double
// underscores to separate URL parts also makes it obvious what the name of the
// PDF is. Eg:
//   http://www.ada.gov/emerprepguideprt.pdf will become
//   www.ada.gov__emerprepguideprt.pdf
func getPdfFileName(u string) (string, error) {
	p, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	pdfFileName := fmt.Sprintf("%s%s", p.Host, strings.Replace(p.Path, "/", "__", -1))
	return pdfFileName, nil
}
