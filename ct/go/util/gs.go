// Google Storage utility that contains methods for both CT master and worker
// scripts.
package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/skia-dev/glog"

	storage "code.google.com/p/google-api-go-client/storage/v1"
	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/gs"
)

const (
	GOROUTINE_POOL_SIZE = 50
	MAX_CHANNEL_SIZE    = 100000
)

type GsUtil struct {
	// The client used to connect to Google Storage.
	client  *http.Client
	service *storage.Service
}

// NewGsUtil initializes and returns a utility for CT interations with Google
// Storage. If client is nil then auth.RunFlow is invoked. if client is nil then
// the client from GetOAuthClient is used.
func NewGsUtil(client *http.Client) (*GsUtil, error) {
	if client == nil {
		oauthClient, err := GetOAuthClient()
		if err != nil {
			return nil, err
		}
		client = oauthClient
	}
	service, err := storage.New(client)
	if err != nil {
		return nil, fmt.Errorf("Failed to create interface to Google Storage: %s", err)
	}
	return &GsUtil{client: client, service: service}, nil
}

func GetOAuthClient() (*http.Client, error) {
	config := auth.OAuthConfig(GSTokenPath, auth.SCOPE_FULL_CONTROL)
	return auth.RunFlow(config)
}

// Returns the response body of the specified GS object. Tries MAX_URI_GET_TRIES
// times if download is unsuccessful. Client must close the response body when
// finished with it.
func getRespBody(res *storage.Object, client *http.Client) (io.ReadCloser, error) {
	for i := 0; i < MAX_URI_GET_TRIES; i++ {
		glog.Infof("Fetching: %s", res.Name)
		request, err := gs.RequestForStorageURL(res.MediaLink)
		if err != nil {
			glog.Warningf("Unable to create Storage MediaURI request: %s\n", err)
			continue
		}

		resp, err := client.Do(request)
		if err != nil {
			glog.Warningf("Unable to retrieve Storage MediaURI: %s", err)
			continue
		}
		if resp.StatusCode != 200 {
			glog.Warningf("Failed to retrieve: %d  %s", resp.StatusCode, resp.Status)
			resp.Body.Close()
			continue
		}
		return resp.Body, nil
	}
	return nil, fmt.Errorf("Failed fetching file after %d attempts", MAX_URI_GET_TRIES)
}

// Returns the response body of the specified GS file. Client must close the
// response body when finished with it.
func (gs *GsUtil) GetRemoteFileContents(filePath string) (io.ReadCloser, error) {
	res, err := gs.service.Objects.Get(GS_BUCKET_NAME, filePath).Do()
	if err != nil {
		return nil, fmt.Errorf("Could not get %s from GS: %s", filePath, err)
	}
	return getRespBody(res, gs.client)
}

// AreTimeStampsEqual checks whether the TIMESTAMP in the local dir matches the
// TIMESTAMP in the remote Google Storage dir.
func (gs *GsUtil) AreTimeStampsEqual(localDir, gsDir string) (bool, error) {
	// Get timestamp from the local directory.
	localTimestampPath := filepath.Join(localDir, TIMESTAMP_FILE_NAME)
	fileContent, err := ioutil.ReadFile(localTimestampPath)
	if err != nil {
		return false, fmt.Errorf("Could not read %s: %s", localTimestampPath, err)
	}
	localTimestamp := strings.Trim(string(fileContent), "\n")

	// Get timestamp from the Google Storage directory.
	gsTimestampPath := filepath.Join(gsDir, TIMESTAMP_FILE_NAME)
	respBody, err := gs.GetRemoteFileContents(gsTimestampPath)
	if err != nil {
		return false, err
	}
	defer respBody.Close()
	resp, err := ioutil.ReadAll(respBody)
	if err != nil {
		return false, err
	}
	gsTimestamp := strings.Trim(string(resp), "\n")

	// Return the comparison of the two timestamps.
	return localTimestamp == gsTimestamp, nil
}

type filePathToStorageObject struct {
	storageObject *storage.Object
	filePath      string
}

// downloadRemoteDir downloads the specified Google Storage dir to the specified
// local dir. The local dir will be emptied and recreated. Handles multiple levels
// of directories.
func (gs *GsUtil) downloadRemoteDir(localDir, gsDir string) error {
	// Empty the local dir.
	os.RemoveAll(localDir)
	// Create the local dir.
	os.MkdirAll(localDir, 0700)
	// The channel where the storage objects to be deleted will be sent to.
	chStorageObjects := make(chan filePathToStorageObject, MAX_CHANNEL_SIZE)
	req := gs.service.Objects.List(GS_BUCKET_NAME).Prefix(gsDir + "/")
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return fmt.Errorf("Error occured while listing %s: %s", gsDir, err)
		}
		for _, result := range resp.Items {
			fileName := filepath.Base(result.Name)
			// If downloading from subdir then add it to the fileName.
			fileGsDir := filepath.Dir(result.Name)
			subDirs := strings.TrimPrefix(fileGsDir, gsDir)
			if subDirs != "" {
				dirTokens := strings.Split(subDirs, "/")
				for i := range dirTokens {
					fileName = filepath.Join(dirTokens[len(dirTokens)-i-1], fileName)
				}
				// Create the local directory.
				os.MkdirAll(filepath.Join(localDir, filepath.Dir(fileName)), 0700)
			}
			chStorageObjects <- filePathToStorageObject{storageObject: result, filePath: fileName}
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	close(chStorageObjects)

	// Kick off goroutines to download the storage objects.
	var wg sync.WaitGroup
	for i := 0; i < GOROUTINE_POOL_SIZE; i++ {
		wg.Add(1)
		go func(goroutineNum int) {
			defer wg.Done()
			for obj := range chStorageObjects {
				result := obj.storageObject
				filePath := obj.filePath
				respBody, err := getRespBody(result, gs.client)
				if err != nil {
					glog.Errorf("Could not fetch %s: %s", result.MediaLink, err)
					return
				}
				defer respBody.Close()
				outputFile := filepath.Join(localDir, filePath)
				out, err := os.Create(outputFile)
				if err != nil {
					glog.Errorf("Unable to create file %s: %s", outputFile, err)
					return
				}
				defer out.Close()
				if _, err = io.Copy(out, respBody); err != nil {
					glog.Error(err)
					return
				}
				glog.Infof("Downloaded gs://%s/%s to %s with goroutine#%d", GS_BUCKET_NAME, result.Name, outputFile, goroutineNum)
			}
		}(i + 1)
	}
	wg.Wait()
	return nil
}

// DownloadChromiumBuild downloads the specified Chromium build from Google
// Storage to a local dir.
func (gs *GsUtil) DownloadChromiumBuild(chromiumBuild string) error {
	localDir := filepath.Join(ChromiumBuildsDir, chromiumBuild)
	gsDir := filepath.Join(CHROMIUM_BUILDS_DIR_NAME, chromiumBuild)
	if equal, _ := gs.AreTimeStampsEqual(localDir, gsDir); equal {
		glog.Infof("Not downloading %s because TIMESTAMPS match", gsDir)
		return nil
	}
	glog.Infof("Timestamps between %s and %s are different. Downloading from Google Storage", localDir, gsDir)
	if err := gs.downloadRemoteDir(localDir, gsDir); err != nil {
		return fmt.Errorf("Error downloading %s into %s: %s", gsDir, localDir, err)
	}
	// Downloaded chrome binary needs to be set as an executable.
	os.Chmod(filepath.Join(localDir, "chrome"), 0777)

	return nil
}

// DownloadWorkerArtifacts downloads artifacts from Google Storage to a local dir.
func (gs *GsUtil) DownloadWorkerArtifacts(dirName, pagesetType string, workerNum int) error {
	localDir := filepath.Join(StorageDir, dirName, pagesetType)
	gsDir := filepath.Join(dirName, pagesetType, fmt.Sprintf("slave%d", workerNum))

	if equal, _ := gs.AreTimeStampsEqual(localDir, gsDir); equal {
		// No need to download artifacts they already exist locally.
		glog.Infof("Not downloading %s because TIMESTAMPS match", gsDir)
		return nil
	}
	glog.Infof("Timestamps between %s and %s are different. Downloading from Google Storage", localDir, gsDir)
	return gs.downloadRemoteDir(localDir, gsDir)
}

func (gs *GsUtil) deleteRemoteDir(gsDir string) error {
	// The channel where the GS filepaths to be deleted will be sent to.
	chFilePaths := make(chan string, MAX_CHANNEL_SIZE)
	req := gs.service.Objects.List(GS_BUCKET_NAME).Prefix(gsDir + "/")
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return fmt.Errorf("Error occured while listing %s: %s", gsDir, err)
		}
		for _, result := range resp.Items {
			chFilePaths <- result.Name
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	close(chFilePaths)

	// Kick off goroutines to delete the file paths.
	var wg sync.WaitGroup
	for i := 0; i < GOROUTINE_POOL_SIZE; i++ {
		wg.Add(1)
		go func(goroutineNum int) {
			defer wg.Done()
			for filePath := range chFilePaths {
				if err := gs.service.Objects.Delete(GS_BUCKET_NAME, filePath).Do(); err != nil {
					glog.Errorf("Goroutine#%d could not delete %s: %s", goroutineNum, filePath, err)
					return
				}
				glog.Infof("Deleted gs://%s/%s with goroutine#%d", GS_BUCKET_NAME, filePath, goroutineNum)
			}
		}(i + 1)
	}
	wg.Wait()
	return nil
}

// UploadFile uploads the specified file to the remote dir in Google Storage. It
// also sets the appropriate ACLs on the uploaded file.
func (gs *GsUtil) UploadFile(fileName, localDir, gsDir string) error {
	localFile := filepath.Join(localDir, fileName)
	gsFile := filepath.Join(gsDir, fileName)
	object := &storage.Object{Name: gsFile}
	f, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("Error opening %s: %s", localFile, err)
	}
	defer f.Close()
	if _, err := gs.service.Objects.Insert(GS_BUCKET_NAME, object).Media(f).Do(); err != nil {
		return fmt.Errorf("Objects.Insert failed: %s", err)
	}
	glog.Infof("Copied %s to %s", localFile, fmt.Sprintf("gs://%s/%s", GS_BUCKET_NAME, gsFile))

	// All objects uploaded to CT's bucket via this util must be readable by
	// the google.com domain. This will be fine tuned later if required.
	objectAcl := &storage.ObjectAccessControl{
		Bucket: GS_BUCKET_NAME, Entity: "domain-google.com", Object: gsFile, Role: "READER",
	}
	if _, err := gs.service.ObjectAccessControls.Insert(GS_BUCKET_NAME, gsFile, objectAcl).Do(); err != nil {
		return fmt.Errorf("Could not update ACL of %s: %s", object.Name, err)
	}
	glog.Infof("Updated ACL of %s", fmt.Sprintf("gs://%s/%s", GS_BUCKET_NAME, gsFile))

	return nil
}

// UploadWorkerArtifacts uploads artifacts from a local dir to Google Storage.
func (gs *GsUtil) UploadWorkerArtifacts(dirName, pagesetType string, workerNum int) error {
	localDir := filepath.Join(StorageDir, dirName, pagesetType)
	gsDir := filepath.Join(dirName, pagesetType, fmt.Sprintf("slave%d", workerNum))

	if equal, _ := gs.AreTimeStampsEqual(localDir, gsDir); equal {
		glog.Infof("Not uploading %s because TIMESTAMPS match", localDir)
		return nil
	}
	glog.Infof("Timestamps between %s and %s are different. Uploading to Google Storage", localDir, gsDir)
	return gs.UploadDir(localDir, gsDir)
}

// UploadDir uploads the specified local dir into the specified Google Storage dir.
func (gs *GsUtil) UploadDir(localDir, gsDir string) error {
	// Empty the remote dir.
	gs.deleteRemoteDir(gsDir)

	// Construct a dictionary of file paths to their file infos.
	pathsToFileInfos := map[string]os.FileInfo{}
	visit := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		pathsToFileInfos[path] = f
		return nil
	}
	if err := filepath.Walk(localDir, visit); err != nil {
		return fmt.Errorf("Unable to read the local dir %s: %s", localDir, err)
	}

	// The channel where the filepaths to be uploaded will be sent to.
	chFilePaths := make(chan string, MAX_CHANNEL_SIZE)
	// File filepaths and send it to the above channel.
	for path, fileInfo := range pathsToFileInfos {
		fileName := fileInfo.Name()
		containingDir := strings.TrimSuffix(path, fileName)
		subDirs := strings.TrimPrefix(containingDir, localDir)
		if subDirs != "" {
			dirTokens := strings.Split(subDirs, "/")
			for i := range dirTokens {
				fileName = filepath.Join(dirTokens[len(dirTokens)-i-1], fileName)
			}
		}
		chFilePaths <- fileName
	}
	close(chFilePaths)

	// Kick off goroutines to upload the file paths.
	var wg sync.WaitGroup
	for i := 0; i < GOROUTINE_POOL_SIZE; i++ {
		wg.Add(1)
		go func(goroutineNum int) {
			defer wg.Done()
			for filePath := range chFilePaths {
				glog.Infof("Uploading %s to %s with goroutine#%d", filePath, gsDir, goroutineNum)
				if err := gs.UploadFile(filePath, localDir, gsDir); err != nil {
					glog.Errorf("Goroutine#%d could not upload %s to %s: %s", goroutineNum, filePath, localDir, err)
				}
			}
		}(i + 1)
	}
	wg.Wait()
	return nil
}
