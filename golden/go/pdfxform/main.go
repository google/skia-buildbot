// pdfxform is a server that rasterizes PDF documents into PNG
package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/pdf"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/goldingester"
	"google.golang.org/api/storage/v1"
)

////////////////////////////////////////////////////////////////////////////////

const (
	PNG_EXT = "png"
	PDF_EXT = "pdf"
)

////////////////////////////////////////////////////////////////////////////////

// md5OfFile calculates the MD5 checksum of a file.
func md5OfFile(path string) (string, error) {
	md5 := md5.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer util.Close(f)
	if _, err = io.Copy(md5, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(md5.Sum(nil)), nil
}

// removeIfExists is like util.Remove, but logs no error if the file does not exist.
func removeIfExists(path string) {
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			glog.Errorf("Failed to Remove(%s): %v", path, err)
		}
	}
}

// isPDF returns true if the path appears to point to a PDF file.
func isPDF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer util.Close(f)
	buffer := make([]byte, 4)
	if n, err := f.Read(buffer); n != 4 || err != nil {
		return false
	}
	return string(buffer) == "%PDF"
}

// writeTo opens a file and dumps the contents of the reader into it.
func writeTo(path string, reader *io.ReadCloser) error {
	defer util.Close(*reader)
	file, err := os.Create(path)
	if err == nil {
		_, err = io.Copy(file, *reader)
	}
	return err
}

////////////////////////////////////////////////////////////////////////////////

// storageClient struct is used for uploading to cloud storage
type storageClient struct {
	httpClient     *http.Client
	storageService *storage.Service
}

// getClient returns an authorized storage.Service
func getClient() (storageClient, error) {
	client, err := auth.NewClient(*local, *oauthCacheFile, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		return storageClient{}, fmt.Errorf("Failed to create an authorized client: %s", err)
	}
	gsService, err := storage.New(client)
	if err != nil {
		return storageClient{}, fmt.Errorf("Failed to create a Google Storage API client: %s", err)
	}
	return storageClient{httpClient: client, storageService: gsService}, nil
}

// gsFetch fetch the object's data from google storage
func gsFetch(object *storage.Object, sc storageClient) (io.ReadCloser, int64, error) {
	request, err := gs.RequestForStorageURL(object.MediaLink)
	if err != nil {
		return nil, -1, err
	}
	resp, err := sc.httpClient.Do(request)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return nil, -1, fmt.Errorf("Failed to retrieve: %s %d %s", object.MediaLink, resp.StatusCode, resp.Status)
	}
	return resp.Body, resp.ContentLength, nil
}

// uploadFile uploads the specified file to the remote dir in Google
// Storage. It also sets the appropriate ACLs on the uploaded file.
// If the file already exists on the server, do nothing.
func uploadFile(sc storageClient, input io.Reader, storageBucket, storagePath, accessControlEntity string) (bool, error) {
	obj, _ := sc.storageService.Objects.Get(storageBucket, storagePath).Do()
	if obj != nil {
		return false, nil // noclobber
	}
	fullPath := fmt.Sprintf("gs://%s/%s", storageBucket, storagePath)
	object := &storage.Object{Name: storagePath}
	if _, err := sc.storageService.Objects.Insert(storageBucket, object).Media(input).Do(); err != nil {
		return false, fmt.Errorf("Objects.Insert(%s) failed: %s", fullPath, err)
	}
	objectAcl := &storage.ObjectAccessControl{
		Bucket: storageBucket, Entity: accessControlEntity, Object: storagePath, Role: "READER",
	}
	if _, err := sc.storageService.ObjectAccessControls.Insert(storageBucket, storagePath, objectAcl).Do(); err != nil {
		return false, fmt.Errorf("Could not update ACL of %s: %s", fullPath, err)
	}
	return true, nil
}

////////////////////////////////////////////////////////////////////////////////

var (
	local                  = flag.Bool("local", false, "Set to true if not running in prod")
	oauthCacheFile         = flag.String("oauth_cache_file", "oauth_cache.dat", "Path to look for and store an OAuth token")
	dataDir                = flag.String("data_dir", "", "Directory to store data in.")
	failureImage           = flag.String("failure_image", "", "Location of a PNG image; must be set")
	storageBucket          = flag.String("storage_bucket", "chromium-skia-gm", "The bucket for json, pdf, and png files")
	storageJsonDirectory   = flag.String("storage_json_directory", "dm-json-v1", "The directory on bucket for json files.")
	storageImagesDirectory = flag.String("storage_images_directory", "dm-images-v1", "The directory on bucket for png and pdf files.")
	accessControlEntity    = flag.String("access_control_entity", "domain-google.com", "The entity that has permissions to manage the bucket")
	graphiteServer         = flag.String("graphite_server", "skia-monitoring:2003", "Where the Graphite metrics ingestion server is running")
)

// The pdfXformer struct holds state
type pdfXformer struct {
	client        storageClient
	rasterizers   []pdf.Rasterizer
	results       map[string]map[int]string
	counter       int
	identifier    string
	errorImageMd5 string
}

// rasterizeOnce applies a single rastetizer to the given pdf file.
// If the rasterizer fails, use the errorImage.  If everything
// succeeds, upload the PNG.
func (xformer *pdfXformer) rasterizeOnce(pdfPath string, rasterizerIndex int) (string, error) {
	rasterizer := xformer.rasterizers[rasterizerIndex]
	tempdir := filepath.Dir(pdfPath)
	pngPath := path.Join(tempdir, fmt.Sprintf("%s.%s", rasterizer.String(), PNG_EXT))
	defer removeIfExists(pngPath)
	glog.Infof("> > > > rasterizing with %s", rasterizer)
	err := rasterizer.Rasterize(pdfPath, pngPath)
	if err != nil {
		glog.Warningf("rasterizing %s with %s failed: %s", filepath.Base(pdfPath), rasterizer.String(), err)
		return xformer.errorImageMd5, nil
	}
	md5, err := md5OfFile(pngPath)
	if err != nil {
		return "", err
	}
	f, err := os.Open(pngPath)
	if err != nil {
		return "", err
	}
	defer util.Close(f)
	pngUploadPath := fmt.Sprintf("%s/%s.%s", *storageImagesDirectory, md5, PNG_EXT)
	didUpload, err := uploadFile(xformer.client, f, *storageBucket, pngUploadPath, *accessControlEntity)
	if err != nil {
		return "", err
	}
	if didUpload {
		glog.Infof("> > > > uploaded %s", pngUploadPath)
	}
	return md5, nil
}

// makeTmpDir returns a nicely-named directory for temp files in $TMPDIR
func (xformer *pdfXformer) makeTmpDir() (string, error) {
	if xformer.identifier == "" {
		var host, userName string
		if h, err := os.Hostname(); err == nil {
			host = h
			if i := strings.Index(host, "."); i >= 0 {
				host = host[:i]
			}
		}
		if currentUser, err := user.Current(); err == nil {
			userName = currentUser.Username
		}
		userName = strings.Replace(userName, `\`, "_", -1)
		xformer.identifier = fmt.Sprintf("%s.%s.%s.tmp.%d.", filepath.Base(os.Args[0]), host, userName, os.Getpid())
	}
	return ioutil.TempDir(*dataDir, xformer.identifier)
}

func newResult(key map[string]string, rasterizerName, digest string) *goldingester.Result {
	keyCopy := map[string]string{}
	for k, v := range key {
		keyCopy[k] = v
	}
	keyCopy["rasterizer"] = rasterizerName
	options := map[string]string{"ext": PNG_EXT}
	return &goldingester.Result{Key: keyCopy, Digest: digest, Options: options}
}

// processResult rasterizes a single PDF result and returns a set of new results.
func (xformer *pdfXformer) processResult(res goldingester.Result) []*goldingester.Result {
	rasterizedResults := []*goldingester.Result{}
	resultMap, found := xformer.results[res.Digest]
	if found {
		// Skip rasterizion steps: big win.
		for index, rasterizer := range xformer.rasterizers {
			digest, ok := resultMap[index]
			if ok {
				rasterizedResults = append(rasterizedResults,
					newResult(res.Key, rasterizer.String(), digest))
			} else {
				glog.Errorf("missing rasterizer %s on %s", rasterizer.String(), res.Digest)
			}
		}
		return rasterizedResults
	}

	tempdir, err := xformer.makeTmpDir()
	if err != nil {
		glog.Errorf("error making temp directory: %s", err)
		return rasterizedResults
	}
	defer util.RemoveAll(tempdir)
	pdfPath := path.Join(tempdir, fmt.Sprintf("%s.pdf", res.Digest))
	objectName := fmt.Sprintf("%s/%s.pdf", *storageImagesDirectory, res.Digest)
	storageURL := fmt.Sprintf("gs://%s/%s", *storageBucket, objectName)
	object, err := xformer.client.storageService.Objects.Get(*storageBucket, objectName).Do()
	if err != nil {
		glog.Errorf("unable to find %s: %s", storageURL, err)
		return []*goldingester.Result{}
	}
	pdfData, _, err := gsFetch(object, xformer.client)
	if err != nil {
		glog.Errorf("unable to retrieve %s: %s", storageURL, err)
		return []*goldingester.Result{}
	}
	err = writeTo(pdfPath, &pdfData)
	if err != nil {
		glog.Errorf("unable to write file %s: %s", pdfPath, err)
		return []*goldingester.Result{}
	}
	if !isPDF(pdfPath) {
		glog.Errorf("%s is not a PDF", objectName)
		return []*goldingester.Result{}
	}
	resultMap = map[int]string{}
	for index, rasterizer := range xformer.rasterizers {
		digest, err := xformer.rasterizeOnce(pdfPath, index)
		if err != nil {
			glog.Errorf("rasterizer %s failed on %s.pdf: %s", rasterizer, res.Digest, err)
			continue
		}
		rasterizedResults = append(rasterizedResults,
			newResult(res.Key, rasterizer.String(), digest))
		resultMap[index] = digest
	}
	xformer.results[res.Digest] = resultMap
	return rasterizedResults
}

// processJsonFile reads a json file and produces a new json file
// with rasterized results.
func (xformer *pdfXformer) processJsonFile(jsonFileObject *storage.Object) {
	jsonURL := fmt.Sprintf("gs://%s/%s", *storageBucket, jsonFileObject.Name)
	if jsonFileObject.Metadata["rasterized"] == "true" {
		glog.Infof("> > skipping %s (already processed) {%d}", jsonURL, xformer.counter)
		return
	}
	body, length, err := gsFetch(jsonFileObject, xformer.client)
	if err != nil {
		glog.Errorf("Failed to fetch %s", jsonURL)
		return
	}
	if 0 == length {
		util.Close(body)
		glog.Infof("> > skipping %s (empty file) {%d}", jsonURL, xformer.counter)
		return
	}
	dmstruct := goldingester.DMResults{}
	err = json.NewDecoder(body).Decode(&dmstruct)
	util.Close(body)
	if err != nil {
		glog.Errorf("Failed to parse %s", jsonURL)
		return
	}
	countPdfResults := 0
	for _, res := range dmstruct.Results {
		if res.Options["ext"] == PDF_EXT {
			countPdfResults++
		}
	}
	if 0 == countPdfResults {
		glog.Infof("> > 0 PDFs found %s {%d}", jsonURL, xformer.counter)
		xformer.setRasterized(jsonFileObject)
		return
	}

	glog.Infof("> > processing %d pdfs of %d results {%d}", countPdfResults, len(dmstruct.Results), xformer.counter)
	rasterizedResults := []*goldingester.Result{}
	i := 0
	for _, res := range dmstruct.Results {
		if res.Options["ext"] == PDF_EXT {
			i++
			glog.Infof("> > > processing %s.pdf [%d/%d] {%d}", res.Digest, i, countPdfResults, xformer.counter)
			rasterizedResults = append(rasterizedResults, xformer.processResult(*res)...)
		}
	}
	newDMStruct := goldingester.DMResults{
		BuildNumber: dmstruct.BuildNumber,
		GitHash:     dmstruct.GitHash,
		Key:         dmstruct.Key,
		Results:     rasterizedResults,
	}
	newJson, err := json.Marshal(newDMStruct)
	if err != nil {
		glog.Errorf("Unexpected json.Marshal error: %s", err)
		return
	}

	now := time.Now()
	// Change the date; leave most of the rest of the path components.
	jsonPathComponents := strings.Split(jsonFileObject.Name, "/") // []string
	if len(jsonPathComponents) < 4 {
		glog.Errorf("unexpected number of path components %q", jsonPathComponents)
		return
	}
	jsonPathComponents = jsonPathComponents[len(jsonPathComponents)-4:]
	jsonPathComponents[1] += "-pdfxformer"
	jsonUploadPath := fmt.Sprintf("%s/%d/%02d/%02d/%02d/%s",
		*storageJsonDirectory,
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		strings.Join(jsonPathComponents, "/"))

	_, err = uploadFile(xformer.client, bytes.NewReader(newJson), *storageBucket, jsonUploadPath, *accessControlEntity)
	glog.Infof("> > wrote gs://%s/%s", *storageBucket, jsonUploadPath)
	newJsonFileObject, err := xformer.client.storageService.Objects.Get(*storageBucket, jsonUploadPath).Do()
	if err != nil {
		glog.Errorf("Failed to find %s: %s", jsonUploadPath, err)
	} else {
		xformer.setRasterized(newJsonFileObject)
	}
	xformer.setRasterized(jsonFileObject)
}

// setRasterized sets the rasterized metadata flag of the given storage.Object
func (xformer *pdfXformer) setRasterized(jsonFileObject *storage.Object) {
	if nil == jsonFileObject.Metadata {
		jsonFileObject.Metadata = map[string]string{}
	}
	jsonFileObject.Metadata["rasterized"] = "true"
	_, err := xformer.client.storageService.Objects.Patch(*storageBucket, jsonFileObject.Name, jsonFileObject).Do()
	if err != nil {
		glog.Errorf("Failed to update metadata of %s: %s", jsonFileObject.Name, err)
	} else {
		glog.Infof("> > Updated metadata of %s", jsonFileObject.Name)
	}
}

// processTimeRange calls gs.GetLatestGSDirs to get a list of
func (xformer *pdfXformer) processTimeRange(start time.Time, end time.Time) {
	glog.Infof("Processing time range: (%s, %s)", start.Truncate(time.Second), end.Truncate(time.Second))
	for _, dir := range gs.GetLatestGSDirs(start.Unix(), end.Unix(), *storageJsonDirectory) {
		glog.Infof("> Reading gs://%s/%s\n", *storageBucket, dir)
		requestedObjects := xformer.client.storageService.Objects.List(*storageBucket).Prefix(dir).Fields(
			"nextPageToken", "items/updated", "items/md5Hash", "items/mediaLink", "items/name", "items/metadata")
		for requestedObjects != nil {
			responseObjects, err := requestedObjects.Do()
			if err != nil {
				glog.Errorf("request %#v failed: %s", requestedObjects, err)
			} else {
				for _, jsonObject := range responseObjects.Items {
					xformer.counter++
					glog.Infof("> > Processing object:  gs://%s/%s {%d}", *storageBucket, jsonObject.Name, xformer.counter)
					xformer.processJsonFile(jsonObject)
				}
			}
			if len(responseObjects.NextPageToken) > 0 {
				requestedObjects.PageToken(responseObjects.NextPageToken)
			} else {
				requestedObjects = nil
			}
		}
	}
	glog.Infof("finished time range.")
}

// uploadErrorImage should be run once to verify that the image is there
func (xformer *pdfXformer) uploadErrorImage(path string) error {
	if "" == path {
		glog.Fatalf("Missing --path argument")
	}
	errorImageMd5, err := md5OfFile(path)
	if err != nil {
		glog.Fatalf("Bad --path argument")
	}
	errorImageFileReader, err := os.Open(path)
	if err != nil {
		return err
	}
	defer util.Close(errorImageFileReader)
	errorImagePath := fmt.Sprintf("%s/%s.png", *storageImagesDirectory, errorImageMd5)
	_, err = uploadFile(xformer.client, errorImageFileReader, *storageBucket, errorImagePath, *accessControlEntity)
	if err != nil {
		return err
	}
	xformer.errorImageMd5 = errorImageMd5
	return nil
}

func main() {
	defer common.LogPanic()
	flag.Parse()
	common.InitWithMetrics("pdfxform", graphiteServer)

	client, err := getClient()
	if err != nil {
		glog.Fatal(err)
	}
	xformer := pdfXformer{
		client:  client,
		results: map[string]map[int]string{},
	}

	err = xformer.uploadErrorImage(*failureImage)
	if err != nil {
		// If we can't upload this, we can't upload anything.
		glog.Fatalf("Filed to upload error image: %s", err)
	}

	for _, rasterizer := range []pdf.Rasterizer{pdf.Pdfium{}, pdf.Poppler{}} {
		if rasterizer.Enabled() {
			xformer.rasterizers = append(xformer.rasterizers, rasterizer)
		} else {
			glog.Infof("rasterizer %s is disabled", rasterizer.String())
		}
	}
	if len(xformer.rasterizers) == 0 {
		glog.Fatalf("no rasterizers found")
	}

	end := time.Now()
	start := end.Add(-172 * time.Hour)
	xformer.processTimeRange(start, end)
	glog.Flush() // Flush before waiting for next tick; it may be a while.
	for _ = range time.Tick(time.Minute) {
		start, end = end, time.Now()
		xformer.processTimeRange(start, end)
		glog.Flush()
	}
}
