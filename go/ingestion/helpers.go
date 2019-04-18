package ingestion

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"google.golang.org/api/option"
)

const (
	// Limit the number of times the ingester tries to get a file before giving up.
	MAX_URI_GET_TRIES = 4

	// maxConcurrentDirPollers is the maximum number of concurrent go-routines that
	// read from GCS and the file system when polling a range of directories.
	maxConcurrentDirPollers = 200
)

// Constructor is the signature that has to be implemented to register a
// Processor implementation to be instantiated by name from a config struct.
//   vcs is an instance that might be shared across multiple ingesters.
//   config is ususally parsed from a JSON5 file.
//   client can be assumed to be ready to serve the needs of the resulting Processor.
//   eventBus is the eventbus to be used by the ingester (optional).
type Constructor func(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, eventBus eventbus.EventBus) (Processor, error)

// stores the constructors that register for instantiation from a config struct.
var constructors = map[string]Constructor{}

// used to synchronize constructor registration and instantiation.
var registrationMutex sync.Mutex

// Register registers the given constructor to create an instance of a Processor.
func Register(id string, constructor Constructor) {
	registrationMutex.Lock()
	defer registrationMutex.Unlock()
	constructors[id] = constructor
}

// IngestersFromConfig creates a list of ingesters from a config struct.
// Usually the struct is created from parsing a config file.
// client is assumed to be suitable for the given application. If e.g. the
// processors of the current application require an authenticated http client,
// then it is expected that client meets these requirements.
func IngestersFromConfig(ctx context.Context, config *sharedconfig.Config, client *http.Client, eventBus eventbus.EventBus, ingestionStore IngestionStore, btConf *gitstore.BTConfig) ([]*Ingester, error) {
	if client == nil {
		return nil, errors.New("httpClient cannot be nil")
	}

	registrationMutex.Lock()
	defer registrationMutex.Unlock()
	ret := []*Ingester{}

	// Make sure we have an eventbus since that is shared by ingesters and sources.
	if eventBus == nil {
		eventBus = eventbus.New()
	}

	// Set up the gitinfo object.
	var vcs vcsinfo.VCS
	var err error
	if btConf != nil {
		gitStore, err := gitstore.NewBTGitStore(ctx, btConf, config.GitRepoURL)
		if err != nil {
			return nil, skerr.Fmt("Error instantiating gitstore: %s", err)
		}

		// Set up VCS instance to track master.
		gitilesRepo := gitiles.NewRepo(config.GitRepoURL, "", client)
		if vcs, err = gitstore.NewVCS(gitStore, "master", gitilesRepo, nil, 0); err != nil {
			return nil, err
		}
		sklog.Infof("Created vcs client based on BigTable.")
	} else {
		if vcs, err = gitinfo.CloneOrUpdate(ctx, config.GitRepoURL, config.GitRepoDir, true); err != nil {
			return nil, err
		}
		sklog.Infof("Created vcs client based on local checkout.")
	}

	// Instantiate the secondary repo if one was specified.
	var secondaryVCS vcsinfo.VCS
	var extractor depot_tools.DEPSExtractor
	if config.SecondaryRepoURL != "" {
		if secondaryVCS, err = gitinfo.CloneOrUpdate(ctx, config.SecondaryRepoURL, config.SecondaryRepoDir, true); err != nil {
			return nil, err
		}
		extractor = depot_tools.NewRegExDEPSExtractor(config.SecondaryRegEx)
		vcs.(*gitinfo.GitInfo).SetSecondaryRepo(secondaryVCS, extractor)
	}

	// for each defined ingester create an instance.
	for id, ingesterConf := range config.Ingesters {
		sklog.Infof("Starting to instantiate ingester: %s", id)
		processorConstructor, ok := constructors[id]
		if !ok {
			return nil, fmt.Errorf("Unknown ingester: '%s'", id)
		}

		// Instantiate the sources
		sources := make([]Source, 0, len(ingesterConf.Sources))
		for _, dataSource := range ingesterConf.Sources {
			oneSource, err := getSource(id, dataSource, client, eventBus)
			if err != nil {
				return nil, fmt.Errorf("Error instantiating sources for ingester '%s': %s", id, err)
			}
			sources = append(sources, oneSource)
			sklog.Infof("Source %s created for ingester %s", oneSource.ID(), id)
		}

		// instantiate the processor
		processor, err := processorConstructor(vcs, ingesterConf, client, eventBus)
		if err != nil {
			return nil, err
		}
		sklog.Infof("Processor constructor for ingester %s created", id)

		// create the ingester and add it to the result.
		ingester, err := NewIngester(id, ingesterConf, vcs, sources, processor, ingestionStore, eventBus)
		if err != nil {
			return nil, err
		}
		ret = append(ret, ingester)
		sklog.Infof("Ingester %s created successfully", id)
	}

	return ret, nil
}

// getSource returns an instance of source that is either getting data from
// Google storage or the local filesystem.
func getSource(id string, dataSource *sharedconfig.DataSource, client *http.Client, eventBus eventbus.EventBus) (Source, error) {
	if dataSource.Dir == "" {
		return nil, fmt.Errorf("Datasource for %s is missing a directory.", id)
	}

	if dataSource.Bucket != "" {
		return NewGoogleStorageSource(id, dataSource.Bucket, dataSource.Dir, client, eventBus)
	}

	return nil, sklog.FmtErrorf("Unable to create source. At least a bucket and directory must be supplied")
}

// validIngestionFile returns true if the given file name matches basic rules.
func validIngestionFile(fName string) bool {
	return targetFileRegExp.Match([]byte(fName))
}

// targetFileRegExp must be matched for a file to be considered for ingestion.
var targetFileRegExp = regexp.MustCompile(`.*\.json`)

// GoogleStorageSource implements the Source interface for Google Storage.
type GoogleStorageSource struct {
	bucket        string
	rootDir       string
	id            string
	storageClient *storage.Client
	eventBus      eventbus.EventBus
}

// NewGoogleStorageSource returns a new instance of GoogleStorageSource based
// on the bucket and directory provided. The id is used to identify the Source
// and is generally the same id as the ingester.
func NewGoogleStorageSource(baseName, bucket, rootDir string, client *http.Client, eventBus eventbus.EventBus) (Source, error) {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Failed to create a Google Storage API client: %s", err)
	}

	return &GoogleStorageSource{
		bucket:        bucket,
		rootDir:       rootDir,
		id:            fmt.Sprintf("%s:gs://%s/%s", baseName, bucket, rootDir),
		storageClient: storageClient,
		eventBus:      eventBus,
	}, nil
}

// See Source interface.
func (g *GoogleStorageSource) Poll(startTime, endTime int64) <-chan ResultFileLocation {
	dirs := fileutil.GetHourlyDirs(g.rootDir, startTime, endTime)
	ch := make(chan ResultFileLocation, maxConcurrentDirPollers)
	concurrentPollers := make(chan bool, maxConcurrentDirPollers)

	go func() {
		var wg sync.WaitGroup
		for _, dir := range dirs {
			concurrentPollers <- true
			wg.Add(1)

			go func(dir string) {
				defer func() {
					<-concurrentPollers
					wg.Done()
				}()
				err := gcs.AllFilesInDir(g.storageClient, g.bucket, dir, func(item *storage.ObjectAttrs) {
					if validIngestionFile(item.Name) && (item.Updated.Unix() > startTime) {
						ch <- newGCSResultFileLocation(item.Bucket,
							item.Name,
							item.Updated.Unix(),
							hex.EncodeToString(item.MD5),
							g.storageClient)
					}
				})

				if err != nil {
					sklog.Errorf("Error occurred while retrieving files from %s/%s: %s", g.bucket, dir, err)
				}
			}(dir)
		}
		wg.Wait()
		close(ch)
	}()

	return ch
}

// See Source interface.
func (g *GoogleStorageSource) ID() string {
	return g.id
}

// SetEventChannel implements the Source interface.
func (g *GoogleStorageSource) SetEventChannel(resultCh chan<- ResultFileLocation) error {
	if g.eventBus != nil {
		eventType, err := g.eventBus.RegisterStorageEvents(g.bucket, g.rootDir, targetFileRegExp, g.storageClient)
		if err != nil {
			return sklog.FmtErrorf("Unable to register storage event: %s", err)
		}

		g.eventBus.SubscribeAsync(eventType, func(evData interface{}) {
			file := evData.(*eventbus.StorageEvent)
			resultCh <- newGCSResultFileLocation(file.BucketID, file.ObjectID, file.TimeStamp, file.MD5, g.storageClient)
		})
		sklog.Infof("Registered for storage event type: %q", eventType)
	}
	return nil
}

// gsResultFileLocation implements the ResultFileLocation for Google storage.
type gsResultFileLocation struct {
	bucket        string
	name          string
	lastUpdated   int64
	md5           string
	storageClient *storage.Client
	content       []byte
}

func newGCSResultFileLocation(bucketID, objectID string, lastUpdated int64, md5 string, storageClient *storage.Client) ResultFileLocation {
	return &gsResultFileLocation{
		bucket:        bucketID,
		name:          objectID,
		lastUpdated:   lastUpdated,
		md5:           md5,
		storageClient: storageClient,
	}
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) Open() (io.ReadCloser, error) {
	// If we have read this before, then just return a reader.
	if g.content != nil {
		return ioutil.NopCloser(bytes.NewBuffer(g.content)), nil
	}

	if g.content == nil {
		var reader io.Reader
		var err error
		for i := 0; i < MAX_URI_GET_TRIES; i++ {
			reader, err = g.storageClient.Bucket(g.bucket).Object(g.name).NewReader(context.Background())
			if err != nil {
				sklog.Errorf("New reader failed for %s/%s: %s", g.bucket, g.name, err)
				continue
			}

			// Read the entire file into memory and return a buffer.
			if g.content, err = ioutil.ReadAll(reader); err != nil {
				sklog.Errorf("Error reading content of %s/%s: %s", g.bucket, g.name, err)
				g.content = nil
				continue
			}

			sklog.Infof("GCSFILE READ %s/%s", g.bucket, g.name)
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Failed fetching %s/%s after %d attempts", g.bucket, g.name, MAX_URI_GET_TRIES)
		}
	}

	return ioutil.NopCloser(bytes.NewBuffer(g.content)), nil
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) Name() string {
	return fmt.Sprintf("gs://%s/%s", g.bucket, g.name)
}

// StorageIDs implements the ResultFileLocation interface.
func (g *gsResultFileLocation) StorageIDs() (string, string) {
	return g.bucket, g.name
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) MD5() string {
	return g.md5
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) TimeStamp() int64 {
	return g.lastUpdated
}

// See ResultFileLocation interface.
func (g *gsResultFileLocation) Content() []byte {
	return g.content
}

// TODO(stephana): Remove FileSystemSource since it's mostly used for testing, but
// ingestion has now moved to being event driven, a mode that is not supported by it.

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
func (f *FileSystemSource) Poll(startTime, endTime int64) <-chan ResultFileLocation {
	retCh := make(chan ResultFileLocation, maxConcurrentDirPollers)

	go func() {
		// Get all the directories we should walk to get the results.
		dirs := fileutil.GetHourlyDirs(f.rootDir, startTime, endTime)
		for _, dir := range dirs {
			// Inject dir into a closure.
			func(dir string) {
				walkFn := func(path string, info os.FileInfo, err error) error {
					if err != nil {
						// We swallow the error to continue processing, but make sure it's
						// shows up in the logs.
						sklog.Errorf("Error walking %s: %s", path, err)
						return nil
					}
					if info.IsDir() {
						return nil
					}

					updateTimestamp := info.ModTime().Unix()
					if validIngestionFile(path) && (updateTimestamp > startTime) {
						rf, err := FileSystemResult(path, f.rootDir)
						if err != nil {
							sklog.Errorf("Unable to create file system result: %s", err)
							return nil
						}
						retCh <- rf
					}
					return nil
				}

				// Only walk the tree if the top directory exists.
				if fileutil.FileExists(dir) {
					if err := filepath.Walk(dir, walkFn); err != nil {
						sklog.Infof("Unable to read the local dir %s: %s", dir, err)
						return
					}
				}
			}(dir)
		}
		close(retCh)
	}()

	return retCh
}

// See Source interface.
func (f *FileSystemSource) ID() string {
	return f.id
}

// SetEventChannel implements the Source interface.
func (f *FileSystemSource) SetEventChannel(resultCh chan<- ResultFileLocation) error {
	// Note: Events are not supported for the file system right now. Since it's mostly used for testing.
	return sklog.FmtErrorf("FileSystemSource does not implement SetEventChannel")
}

// fsResultFileLocation implements the ResultFileLocation interface for
// the local filesystem.
type fsResultFileLocation struct {
	path        string
	buf         []byte
	md5         string
	lastUpdated int64
}

// FileSystemResult returns a ResultFileLocation for files. path is the path
// where the target file resides and rootDir is the root of all paths.
func FileSystemResult(path, rootDir string) (ResultFileLocation, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Read file into buffer and calculate the md5 in the process.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(file)

	var buf bytes.Buffer
	md5, err := util.MD5FromReader(file, &buf)
	if err != nil {
		return nil, fmt.Errorf("Unable to get MD5 hash of %s: %s", path, err)
	}

	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return &fsResultFileLocation{
		path:        strings.TrimPrefix(absPath, absRootDir+"/"),
		buf:         buf.Bytes(),
		md5:         hex.EncodeToString(md5),
		lastUpdated: fileInfo.ModTime().Unix(),
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

// StorageIDs implements the ResultFileLocation interface.
func (f *fsResultFileLocation) StorageIDs() (string, string) {
	return "--fsResultFileLocation", f.path
}

// see ResultFileLocation interface.
func (f *fsResultFileLocation) MD5() string {
	return f.md5
}

// see ResultFileLocation interface.
func (f *fsResultFileLocation) TimeStamp() int64 {
	return f.lastUpdated
}

// see ResultFileLocation interface.
func (f *fsResultFileLocation) Content() []byte {
	return f.buf
}
