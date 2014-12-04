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

	"github.com/golang/glog"

	"code.google.com/p/google-api-go-client/storage/v1"
	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/gs"
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
	config := auth.OAuthConfig(GSTokenPath, auth.SCOPE_READ_WRITE)
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
	// Empty the local dir.
	os.RemoveAll(localDir)
	// Create the local dir.
	os.MkdirAll(localDir, 0700)
	// Download from Google Storage.
	var wg sync.WaitGroup
	req := gs.service.Objects.List(GS_BUCKET_NAME).Prefix(gsDir + "/")
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return fmt.Errorf("Error occured while listing %s: %s", gsDir, err)
		}
		for _, result := range resp.Items {
			fileName := filepath.Base(result.Name)

			wg.Add(1)
			go func() {
				defer wg.Done()
				respBody, err := getRespBody(result, gs.client)
				if err != nil {
					glog.Errorf("Could not fetch %s: %s", result.MediaLink, err)
					return
				}
				defer respBody.Close()
				outputFile := filepath.Join(localDir, fileName)
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
				glog.Infof("Downloaded gs://%s/%s to %s", GS_BUCKET_NAME, result.Name, outputFile)
			}()
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	wg.Wait()
	return nil
}

func (gs *GsUtil) deleteRemoteDir(gsDir string) error {
	var wg sync.WaitGroup
	req := gs.service.Objects.List(GS_BUCKET_NAME).Prefix(gsDir + "/")
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return fmt.Errorf("Error occured while listing %s: %s", gsDir, err)
		}
		for _, result := range resp.Items {
			wg.Add(1)
			filePath := result.Name
			go func() {
				defer wg.Done()
				if err := gs.service.Objects.Delete(GS_BUCKET_NAME, filePath).Do(); err != nil {
					glog.Errorf("Could not delete %s: %s", filePath, err)
					return
				}
				glog.Infof("Deleted gs://%s/%s", GS_BUCKET_NAME, filePath)
			}()
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	wg.Wait()
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

	// Empty the remote dir.
	gs.deleteRemoteDir(gsDir)
	// List the local directory.
	fileInfos, err := ioutil.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("Unable to read the local dir %s: %s", localDir, err)
	}
	// Upload local files into the remote directory.
	var wg sync.WaitGroup
	for _, fileInfo := range fileInfos {
		fileName := fileInfo.Name()
		wg.Add(1)
		go func() {
			defer wg.Done()
			localFile := filepath.Join(localDir, fileName)
			gsFile := filepath.Join(gsDir, fileName)
			object := &storage.Object{Name: gsFile}
			f, err := os.Open(localFile)
			if err != nil {
				glog.Errorf("Error opening %s: %s", localFile, err)
				return
			}
			defer f.Close()
			res, err := gs.service.Objects.Insert(GS_BUCKET_NAME, object).Media(f).Do()
			if err != nil {
				glog.Errorf("Objects.Insert failed: %s", err)
				return
			}
			glog.Infof("Created object %s at location %s", res.Name, res.SelfLink)
		}()
	}
	wg.Wait()
	return nil
}
