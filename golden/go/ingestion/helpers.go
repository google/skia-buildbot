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
	"regexp"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/cenkalti/backoff"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/eventbus"
)

const (
	// Limit the ingester's attempts to get a file before giving up.
	maxReadTime = 30 * time.Second

	// maxConcurrentDirPollers is the maximum number of concurrent go-routines that
	// read from GCS and the file system when polling a range of directories.
	maxConcurrentDirPollers = 200
)

// Constructor is the signature that has to be implemented to register a
// Processor implementation to be instantiated by name from a config struct.
//   vcs is an instance that might be shared across multiple ingesters.
//   config is usually parsed from a JSON5 file.
//   client can be assumed to be ready to serve the needs of the resulting Processor.
//   eventBus is the eventbus to be used by the ingester (optional).
type Constructor func(context.Context, vcsinfo.VCS, *sharedconfig.IngesterConfig, *http.Client) (Processor, error)

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
func IngestersFromConfig(ctx context.Context, config *sharedconfig.Config, client *http.Client, eventBus eventbus.EventBus, ingestionStore IngestionStore, btConf *bt_gitstore.BTConfig) ([]*Ingester, error) {
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
	if btConf != nil {
		gitStore, err := bt_gitstore.New(ctx, btConf, config.GitRepoURL)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not instantiate gitstore for %s", config.GitRepoURL)
		}

		// Set up VCS instance to track master.
		gitilesRepo := gitiles.NewRepo(config.GitRepoURL, client)
		if vcs, err = bt_vcs.New(ctx, gitStore, "master", gitilesRepo); err != nil {
			return nil, skerr.Wrapf(err, "could not create new bt_vcs")
		}

		sklog.Infof("Created vcs client based on BigTable.")
	} else {
		var err error
		if vcs, err = gitinfo.CloneOrUpdate(ctx, config.GitRepoURL, config.GitRepoDir, true); err != nil {
			return nil, skerr.Wrapf(err, "could not clone %s locally to %s", config.GitRepoURL, config.GitRepoDir)
		}
		sklog.Infof("Created vcs client based on local checkout.")
	}

	// Instantiate the secondary repo if one was specified.
	// TODO(kjlubick): make this support bigtable git also.
	if config.SecondaryRepoURL != "" {
		// TODO(kjlubick) Check up tracestore_impl's isOnMaster to make sure it
		// works with what is put here.
		return nil, skerr.Fmt("Not yet implemented to have a secondary repo url")
	}

	// for each defined Ingester create an instance.
	for id, ingesterConf := range config.Ingesters {
		sklog.Infof("Starting to instantiate ingester: %s", id)
		processorConstructor, ok := constructors[id]
		if !ok {
			return nil, skerr.Fmt("unknown ingester: '%s'", id)
		}

		// Instantiate the sources
		sources := make([]Source, 0, len(ingesterConf.Sources))
		for _, dataSource := range ingesterConf.Sources {
			oneSource, err := getSource(ctx, id, dataSource, client, eventBus)
			if err != nil {
				return nil, skerr.Wrapf(err, "Error instantiating sources for Ingester %q", id)
			}
			sources = append(sources, oneSource)
			sklog.Infof("Source %s created for Ingester %s", oneSource.ID(), id)
		}

		// instantiate the processor
		processor, err := processorConstructor(ctx, vcs, ingesterConf, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not create processor %q", id)
		}
		sklog.Infof("Processor constructor for Ingester %s created", id)

		// create the Ingester and add it to the result.
		ingester, err := newIngester(id, ingesterConf, vcs, sources, processor, ingestionStore, eventBus)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not create Ingester %q", id)
		}
		ret = append(ret, ingester)
		sklog.Infof("Ingester %s created successfully", id)
	}

	return ret, nil
}

// getSource returns an instance of source that is either getting data from
// Google storage or the local filesystem.
func getSource(ctx context.Context, id string, dataSource *sharedconfig.DataSource, client *http.Client, eventBus eventbus.EventBus) (Source, error) {
	if dataSource.Dir == "" {
		return nil, fmt.Errorf("Datasource for %s is missing a directory.", id)
	}

	if dataSource.Bucket != "" {
		return newGoogleStorageSource(ctx, id, dataSource.Bucket, dataSource.Dir, client, eventBus)
	}

	return nil, skerr.Fmt("Unable to create source. At least a bucket and directory must be supplied")
}

// validIngestionFile returns true if the given file name matches basic rules.
func validIngestionFile(fName string) bool {
	return targetFileRegExp.Match([]byte(fName))
}

// targetFileRegExp must be matched for a file to be considered for ingestion.
var targetFileRegExp = regexp.MustCompile(`.*\.json`)

// googleStorageSource implements the Source interface for Google Storage.
type googleStorageSource struct {
	bucket        string
	rootDir       string
	id            string
	storageClient *storage.Client
	eventBus      eventbus.EventBus
}

// newGoogleStorageSource returns a new instance of GoogleStorageSource based
// on the bucket and directory provided. The id is used to identify the Source
// and is generally the same id as the Ingester.
func newGoogleStorageSource(ctx context.Context, baseName, bucket, rootDir string, client *http.Client, eventBus eventbus.EventBus) (Source, error) {
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Fmt("Failed to create a Google Storage API client: %s", err)
	}

	return &googleStorageSource{
		bucket:        bucket,
		rootDir:       rootDir,
		id:            fmt.Sprintf("%s:gs://%s/%s", baseName, bucket, rootDir),
		storageClient: storageClient,
		eventBus:      eventBus,
	}, nil
}

// Poll implements the ingestion.Source interface
func (g *googleStorageSource) Poll(startTime, endTime int64) <-chan ResultFileLocation {
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

// ID implements the ingestion.Source interface
func (g *googleStorageSource) ID() string {
	return g.id
}

// SetEventChannel implements the ingestion.Source interface.
func (g *googleStorageSource) SetEventChannel(resultCh chan<- ResultFileLocation) error {
	if g.eventBus != nil {
		eventType, err := g.eventBus.RegisterStorageEvents(g.bucket, g.rootDir, targetFileRegExp, g.storageClient)
		if err != nil {
			return skerr.Fmt("Unable to register storage event: %s", err)
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

	exp := &backoff.ExponentialBackOff{
		InitialInterval:     time.Second,
		RandomizationFactor: 0.5,
		Multiplier:          2,
		MaxInterval:         maxReadTime / 4,
		MaxElapsedTime:      maxReadTime,
		Clock:               backoff.SystemClock,
	}

	o := func() error {
		obj := g.storageClient.Bucket(g.bucket).Object(g.name)
		reader, err := obj.NewReader(context.TODO())
		if err != nil {
			return skerr.Fmt("accessing %s/%s failed: %s", g.bucket, g.name, err)
		}
		defer util.Close(reader)

		// Read the entire file into memory and return a buffer.
		if g.content, err = ioutil.ReadAll(reader); err != nil {
			g.content = nil
			return skerr.Fmt("error reading content of %s/%s: %s", g.bucket, g.name, err)
		}

		if oa, err := obj.Attrs(context.TODO()); err != nil {
			g.content = nil
			return skerr.Fmt("error reading attributes of %s/%s: %s", g.bucket, g.name, err)
		} else {
			g.md5 = fmt.Sprintf("%x", oa.MD5)
			g.lastUpdated = oa.Updated.Unix()
		}

		sklog.Debugf("Ingester read %s/%s", g.bucket, g.name)
		return nil
	}

	if g.content == nil {
		if err := backoff.Retry(o, exp); err != nil {
			return nil, skerr.Fmt("could not read gcs://%s/%s with retries: %s", g.bucket, g.name, err)
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
