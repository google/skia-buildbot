// Application that downloads PDFs and then captures SKPs from them.
// TODO(rmistry): Capturing and uploading SKPs has been temporarily disabled due
// to the comment in https://bugs.chromium.org/p/skia/issues/detail?id=5183#c34
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to download PDFs and capture their SKPs.
	WORKER_POOL_SIZE = 100
)

var (
	startRange     = flag.Int("start_range", 1, "The number this worker will capture SKPs from.")
	num            = flag.Int("num", 100, "The total number of SKPs to capture starting from the start_range.")
	pagesetType    = flag.String("pageset_type", util.PAGESET_TYPE_PDF_1m, "The type of pagesets to use for this run. Eg: PDF1m.")
	chromiumBuild  = flag.String("chromium_build", "", "The specified chromium build. This value is used while uploading the PDFs and SKPs to Google Storage.")
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Capturing SKPs from PDFs")
	defer glog.Flush()

	// Validate required arguments.
	if *runID == "" {
		glog.Fatal("Must specify --run_id")
	}
	if *chromiumBuild == "" {
		glog.Fatal("Must specify --chromium_build")
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Fatal(err)
	}

	// Download PDF pagesets if they do not exist locally.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	pagesetsToIndex, err := gs.DownloadSwarmingArtifacts(pathToPagesets, util.PAGESETS_DIR_NAME, *pagesetType, *startRange, *num)
	if err != nil {
		glog.Fatal(err)
	}
	defer skutil.RemoveAll(pathToPagesets)

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

	// TODO(rmistry): Uncomment when ready to capture SKPs.
	//// Copy over the pdfium_test binary to this slave.
	//pdfiumLocalPath := filepath.Join(os.TempDir(), util.BINARY_PDFIUM_TEST)
	//pdfiumRemotePath := filepath.Join(util.BinariesDir, *runID, util.BINARY_PDFIUM_TEST)
	//respBody, err := gs.GetRemoteFileContents(pdfiumRemotePath)
	//if err != nil {
	//	glog.Fatalf("Could not fetch %s: %s", pdfiumRemotePath, err)
	//}
	//defer skutil.Close(respBody)
	//out, err := os.Create(pdfiumLocalPath)
	//if err != nil {
	//	glog.Fatalf("Unable to create file %s: %s", pdfiumLocalPath, err)
	//}
	//defer skutil.Remove(pdfiumLocalPath)
	//if _, err = io.Copy(out, respBody); err != nil {
	//	glog.Fatal(err)
	//}
	//skutil.Close(out)
	//// Downloaded pdfium_test binary needs to be set as an executable.
	//skutil.LogErr(os.Chmod(pdfiumLocalPath, 0777))

	// TODO(rmistry): Uncomment when ready to capture SKPs.
	//timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CaptureSKPsTimeoutSecs
	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		glog.Fatalf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
	}

	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := util.GetClosedChannelOfPagesets(fileInfos)

	var wg sync.WaitGroup

	// Gather PDFs and SKPs with errors.
	erroredPDFs := []string{}
	erroredSKPs := []string{}
	// Mutexes to control access to the above.
	var erroredPDFsMutex sync.Mutex
	//var erroredSKPsMutex sync.Mutex

	// Loop through workers in the worker pool.
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures SKPs.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			// Instantiate timeout client for downloading PDFs.
			transport := &http.Transport{
				Dial: httputils.DialTimeout,
			}
			httpTimeoutClient := &http.Client{
				Transport: transport,
				Timeout:   httputils.REQUEST_TIMEOUT,
			}

			for pagesetName := range pagesetRequests {
				index := strconv.Itoa(pagesetsToIndex[path.Join(pathToPagesets, pagesetName)])

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
				pdfURL := decodedPageset.UrlsList

				skutil.LogErr(os.Chdir(pathToPdfs))

				// Download PDF.
				if err := downloadPDFs(pdfURL, index, pathToPdfs, httpTimeoutClient); err != nil {
					glog.Errorf("Could not download %s: %s", pdfURL, err)
					erroredPDFsMutex.Lock()
					erroredPDFs = append(erroredPDFs, pdfURL)
					erroredPDFsMutex.Unlock()
					continue
				}
				// By default, transport caches connections for future re-use.
				// This may leave many open connections when accessing many hosts.
				transport.CloseIdleConnections()

				// TODO(rmistry): Uncomment when ready to capture SKPs.
				// TODO(rmistry): Use erroredSKPsMutex below.
				//// Run pdfium_test to create SKPs from the PDFs.
				//pdfiumTestArgs := []string{
				//	"--skp", pdfPath,
				//}
				//if err := exec.Run(&exec.Command{
				//	Name:        pdfiumLocalPath,
				//	Args:        pdfiumTestArgs,
				//	Env:         []string{},
				//	InheritPath: true,
				//	Timeout:     time.Duration(timeoutSecs) * time.Second,
				//	LogStdout:   false,
				//	Stdout:      nil,
				//	LogStderr:   false,
				//	Stderr:      nil,
				//}); err != nil {
				//	erroredSKPs = append(erroredSKPs, pdfBase)
				//	continue
				//}
				//
				//// Move generated SKPs into the pathToSKPs directory.
				//skps, err := filepath.Glob(path.Join(pdfDirWithIndex, fmt.Sprintf("%s.*.skp", pdfBase)))
				//if err != nil {
				//	glog.Errorf("Found no SKPs for %s: %s", pdfBase, err)
				//	erroredSKPs = append(erroredSKPs, pdfBase)
				//	continue
				//}
				//for _, skp := range skps {
				//	skpBasename := path.Base(skp)
				//	destDir := path.Join(pathToSkps, index)
				//	if err := os.MkdirAll(destDir, 0700); err != nil {
				//		glog.Errorf("Could not mkdir %s: %s", destDir, err)
				//	}
				//	dest := path.Join(destDir, skpBasename)
				//	if err := os.Rename(skp, dest); err != nil {
				//		glog.Errorf("Could not move %s to %s: %s", skp, dest, err)
				//		continue
				//	}
				//}
			}
		}()
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// Check to see if there is anything in the pathToPDFs and pathToSKPs dirs.
	pdfsEmpty, err := skutil.IsDirEmpty(pathToPdfs)
	if err != nil {
		glog.Fatal(err)
	}
	if pdfsEmpty {
		glog.Fatalf("Could not download any PDF in %s", pathToPdfs)
	}
	// TODO(rmistry): Uncomment when ready to capture SKPs.
	//skpsEmpty, err := skutil.IsDirEmpty(pathToSkps)
	//if err != nil {
	//	glog.Fatal(err)
	//}
	//if skpsEmpty {
	//	glog.Fatalf("Could not create any SKP in %s", pathToSkps)
	//}
	//
	//// Move and validate all SKP files.
	//pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
	//if err := util.ValidateSKPs(pathToSkps, pathToPyFiles); err != nil {
	//	glog.Fatal(err)
	//}

	// Upload PDFs dir to Google Storage.
	if err := gs.UploadSwarmingArtifacts(util.PDFS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild)); err != nil {
		glog.Fatal(err)
	}
	// Upload SKPs dir to Google Storage.
	if err := gs.UploadSwarmingArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild)); err != nil {
		glog.Fatal(err)
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

func downloadPDFs(pdfURL, index, pathToPdfs string, httpTimeoutClient *http.Client) error {
	// Add protocol if it is missing from the URL.
	if !(strings.HasPrefix(pdfURL, "http://") || strings.HasPrefix(pdfURL, "https://")) {
		pdfURL = fmt.Sprintf("http://%s", pdfURL)
	}
	pdfBase, err := getPdfFileName(pdfURL)
	if err != nil {
		return fmt.Errorf("Could not parse the URL %s to get a PDF file name: %s", pdfURL, err)
	}
	pdfDirWithIndex := filepath.Join(pathToPdfs, index)
	if err := os.MkdirAll(pdfDirWithIndex, 0700); err != nil {
		return fmt.Errorf("Could not mkdir %s: %s", pdfDirWithIndex, err)
	}
	pdfPath := filepath.Join(pdfDirWithIndex, pdfBase)
	resp, err := httpTimeoutClient.Get(pdfURL)
	if err != nil {
		return fmt.Errorf("Could not GET %s: %s", pdfURL, err)
	}
	defer skutil.Close(resp.Body)
	out, err := os.Create(pdfPath)
	defer skutil.Close(out)
	if err != nil {
		return fmt.Errorf("Unable to create file %s: %s", pdfPath, err)
	}
	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("Unable to write to file %s: %s", pdfPath, err)
	}
	return nil
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
