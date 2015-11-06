package ingestion

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	storage "google.golang.org/api/storage/v1"
)

// Limit the number of times the ingester tries to get a file before giving up.
const MAX_URI_GET_TRIES = 4

// Constructor is the signature that has to be implemented to register a
// Processor implementation to be instantiated by name from a config struct.
type Constructor func(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig) (Processor, error)

// stores the constructors that register for instantiation from a config struct.
var constructors = map[string]Constructor{}

// used to synchronize constructor registration and instantiation.
var registrationMutex sync.Mutex

// Register registers the given constructor to create an instance of a Processor.
func Register(name string, constructor Constructor) {
	registrationMutex.Lock()
	defer registrationMutex.Unlock()
	constructors[name] = constructor
}

// IngestersFromConfig creates a list of ingesters from a config struct.
// Usually the struct is created from parsing a config file.
func IngestersFromConfig(config *sharedconfig.Config, client *http.Client) ([]*Ingester, error) {
	ret := []*Ingester{}

	// Set up the gitinfo object.
	var vcs vcsinfo.VCS
	var err error
	if vcs, err = gitinfo.CloneOrUpdate(config.GitRepoURL, config.GitRepoDir, true); err != nil {
		return nil, err
	}

	// Set up the Google storage client.
	if client == nil {
		client = &http.Client{
			Transport: util.NewBackOffTransport(),
		}
	}

	// for each defined ingester create an instance.
	for id, ingesterConf := range config.Ingesters {
		processorConstructor, ok := constructors[id]
		if !ok {
			return nil, fmt.Errorf("Unknow ingester: '%s'", id)
		}

		// Instantiate the sources
		sources := make([]Source, 0, len(ingesterConf.Sources))
		for _, dataSource := range ingesterConf.Sources {
			oneSource, err := getSource(id, dataSource, client)
			if err != nil {
				return nil, fmt.Errorf("Error instantiating sources for ingester '%s': %s", id, err)
			}
			sources = append(sources, oneSource)
		}

		// instantiate the processor
		processor, err := processorConstructor(vcs, ingesterConf)
		if err != nil {
			return nil, err
		}

		// create the ingester and add it to the result.
		ingester, err := NewIngester(id, ingesterConf, vcs, sources, processor)
		if err != nil {
			return nil, err
		}
		ret = append(ret, ingester)
	}

	return ret, nil
}

// getSource returns an instance of source that is either getting data from
// Google storage or the local fileystem.
func getSource(id string, dataSource *sharedconfig.DataSource, client *http.Client) (Source, error) {
	if dataSource.Dir == "" {
		return nil, fmt.Errorf("Datasource for %s is missing a directory.", id)
	}

	if dataSource.Bucket != "" {
		return NewGoogleStorageSource(id, dataSource.Bucket, dataSource.Dir, client)
	}
	return NewFileSystemSource(id, dataSource.Dir)
}

// GoogleStorageSource implementes the Source interface for Google Storage.
type GoogleStorageSource struct {
	bucket   string
	rootDir  string
	id       string
	gStorage *storage.Service
	client   *http.Client
}

// NewGoogleStorageSource returns a new instance of GoogleStorageSource based
// on the bucket and directory provided. The id is used to identify the Source
// and is generally the same id as the ingester.
func NewGoogleStorageSource(baseName, bucket, rootDir string, client *http.Client) (Source, error) {
	gStorage, err := storage.New(client)
	if err != nil {
		return nil, err
	}

	return &GoogleStorageSource{
		bucket:   bucket,
		rootDir:  rootDir,
		id:       fmt.Sprintf("%s:gs://%s/%s", baseName, bucket, rootDir),
		client:   client,
		gStorage: gStorage,
	}, nil
}

// See Source interface.
func (g *GoogleStorageSource) Poll(startTime, endTime int64) ([]ResultFileLocation, error) {
	dirs := gs.GetLatestGSDirs(startTime, endTime, g.rootDir)
	retval := []ResultFileLocation{}
	for _, dir := range dirs {
		glog.Infof("Opening bucket/directory: %s/%s", g.bucket, dir)

		req := g.gStorage.Objects.List(g.bucket).Prefix(dir).Fields("nextPageToken", "items/updated", "items/md5Hash", "items/mediaLink", "items/name")
		for req != nil {
			resp, err := req.Do()
			if err != nil {
				return nil, fmt.Errorf("Error occurred while listing JSON files: %s", err)
			}
			for _, result := range resp.Items {
				updateDate, err := time.Parse(time.RFC3339, result.Updated)
				if err != nil {
					glog.Errorf("Unable to parse date %s: %s", result.Updated, err)
					continue
				}
				updateTimestamp := updateDate.Unix()
				if updateTimestamp > startTime {
					// Decode the MD5 hash from base64.
					md5Bytes, err := base64.StdEncoding.DecodeString(result.Md5Hash)
					if err != nil {
						glog.Errorf("Unable to decode base64-encoded MD5: %s", err)
						continue
					}
					// We re-encode the md5 hash as a hex string to make debugging and testing easier.
					retval = append(retval, newGSResultFileLocation(result, updateTimestamp, hex.EncodeToString(md5Bytes), g.client))
				}
			}
			if len(resp.NextPageToken) > 0 {
				req.PageToken(resp.NextPageToken)
			} else {
				req = nil
			}
		}
	}
	return retval, nil
}

// TODO(stephana): Add the event channel once it's tested in the Ingester type.
// See Source interface.
func (g *GoogleStorageSource) EventChan() <-chan []ResultFileLocation {
	return nil
}

// See Source interface.
func (g *GoogleStorageSource) ID() string {
	return g.id
}

// gsResultFileLocation implements the ResultFileLocation for Google storage.
type gsResultFileLocation struct {
	uri         string
	name        string
	lastUpdated int64
	md5         string
	client      *http.Client
}

func newGSResultFileLocation(result *storage.Object, updateTS int64, md5 string, client *http.Client) ResultFileLocation {
	return &gsResultFileLocation{
		uri:         result.MediaLink,
		name:        result.Name,
		lastUpdated: updateTS,
		md5:         md5,
		client:      client,
	}
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) Open() (io.ReadCloser, error) {
	for i := 0; i < MAX_URI_GET_TRIES; i++ {
		request, err := gs.RequestForStorageURL(g.uri)
		if err != nil {
			glog.Errorf("Unable to create Storage MediaURI request: %s\n", err)
			continue
		}
		resp, err := g.client.Do(request)
		if err != nil {
			glog.Errorf("Unable to retrieve URI while creating file iterator: %s", err)
			continue
		}
		if resp.StatusCode != 200 {
			glog.Errorf("Failed to retrieve %s. Error: %d  %s", g.uri, resp.StatusCode, resp.Status)
		}
		glog.Infof("GSFILE READ %s", g.name)
		return resp.Body, nil
	}
	return nil, fmt.Errorf("Failed fetching %s after %d attempts", g.uri, MAX_URI_GET_TRIES)
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) Name() string {
	return g.name
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) MD5() string {
	return g.md5
}

// FileSystemSource implements the Source interface to read from the local
// file system.
type FileSystemSource struct {
	rootDir string
	id      string
}

func NewFileSystemSource(baseName, rootDir string) (Source, error) {
	return &FileSystemSource{
		rootDir: rootDir,
		id:      fmt.Sprintf("%s:fs:%s", baseName, rootDir),
	}, nil
}

// See Source interface.
func (f *FileSystemSource) Poll(startTime, endTime int64) ([]ResultFileLocation, error) {
	retval := []ResultFileLocation{}

	// although GetLatestGSDirs is in the "gs" package, there's nothing specific about
	// its operation that makes it not re-usable here.
	dirs := gs.GetLatestGSDirs(startTime, endTime, f.rootDir)
	for _, dir := range dirs {
		// Inject dir into a closure.
		func(dir string) {
			walkFn := func(path string, info os.FileInfo, err error) error {
				if err != nil {
					// We swallow the error to continue processing, but make sure it's
					// shows up in the logs.
					glog.Errorf("Error walking %s: %s", path, err)
					return nil
				}
				if info.IsDir() {
					return nil
				}

				updateTimestamp := info.ModTime().Unix()
				if updateTimestamp > startTime {
					rf, err := FileSystemResult(path)
					if err != nil {
						glog.Errorf("Unable to create file system result: %s", err)
						return nil
					}
					retval = append(retval, rf)
				}
				return nil
			}

			// Only walk the tree if the top directory exists.
			if fileutil.FileExists(dir) {
				if err := filepath.Walk(dir, walkFn); err != nil {
					glog.Infof("Unable to read the local dir %s: %s", dir, err)
					return
				}
			}
		}(dir)
	}

	return retval, nil
}

// See Source interface.
func (f *FileSystemSource) EventChan() <-chan []ResultFileLocation {
	return nil
}

// See Source interface.
func (f *FileSystemSource) ID() string {
	return f.id
}

// fsResultFileLocation implements the ResultFileLocation interface for
// the local filesystem.
type fsResultFileLocation struct {
	path string
	buf  []byte
	md5  string
}

func FileSystemResult(path string) (ResultFileLocation, error) {
	// Read file into buffer and calculate the md5 in the process.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(file)

	hashWriter := md5.New()
	var buf bytes.Buffer
	tempOut := io.MultiWriter(&buf, hashWriter)
	if _, err := io.Copy(tempOut, file); err != nil {
		return nil, fmt.Errorf("Unable to get MD5 hash of %s: %s", path, err)
	}

	return &fsResultFileLocation{
		path: path,
		buf:  buf.Bytes(),
		md5:  hex.EncodeToString(hashWriter.Sum(nil)),
	}, nil
}

// see ResultFileLocation interface.
func (f *fsResultFileLocation) Open() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewBuffer(f.buf)), nil
}

// see ResultFileLocation interface.
func (f *fsResultFileLocation) Name() string {
	return f.path
}

// see ResultFileLocation interface.
func (f *fsResultFileLocation) MD5() string {
	return f.md5
}
