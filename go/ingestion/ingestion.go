package ingestion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// BoltDB bucket where MD5 hashes of processed files are stored.
const PROCESSED_FILES_BUCKET = "processed_files"

// Tag names used to collect metrics.
const (
	MEASUREMENT_INGESTION = "ingestion"
	TAG_INGESTION_METRIC  = "metric"
	TAG_INGESTER_ID       = "ingester"
	TAG_INGESTER_SOURCE   = "source"

	// nConcurrentProcessors is the maximum number of go-routines that run Processors.
	// The number is chosen experimentally and should be adjusted to optimize throughput.
	nConcurrentProcessors = 256

	// eventChanSize is the buffer size of the events channel. Most of the time
	// that channel should be almost empty, but this ensures we buffer events if
	// processing input files take longer or there is a large number of concurrent events.
	eventChanSize = 500
)

var (
	// IgnoreResultsFileErr can be returned by the Process function of a processor to
	// indicated that this file should be considered ignored. It is up to the processor
	// to write to the log.
	IgnoreResultsFileErr = errors.New("Ignore this file.")
)

// Source defines an ingestion source that returns lists of result files
// either through polling or in an event driven mode.
type Source interface {
	// ID returns a unique identifier for this source.
	ID() string

	// Poll returns a channel to read all the result files that originated between
	// the given timestamps in seconds since the epoch.
	Poll(startTime, endTime int64) <-chan ResultFileLocation

	// SetEventChannel configures storage events and sets up routines to send
	// new results to the given channel.
	SetEventChannel(resultCh chan<- ResultFileLocation) error
}

// ResultFileLocation is an abstract interface to a file like object that
// contains results that need to be ingested.
type ResultFileLocation interface {
	// Open returns a reader that allows to read the content of the file.
	Open() (io.ReadCloser, error)

	// Name returns the full path of the file. The last segment is usually the
	// the file name.
	Name() string

	// StorageIDs return the bucket and object ID for the given location.
	StorageIDs() (string, string)

	// MD5 returns the MD5 hash of the content of the file.
	MD5() string

	// Timestamp returns the timestamp when the file was last updated.
	TimeStamp() int64

	// Content returns the content of the file if has been read or nil otherwise.
	Content() []byte
}

// Processor is the core of an ingester. It takes instances of ResultFileLocation
// and ingests them. It is responsible for the storage of ingested data.
type Processor interface {
	// Process ingests a single result file.
	Process(ctx context.Context, resultsFile ResultFileLocation) error
}

// IngestionStore keeps track of files being ingested based on their MD5 hashes.
type IngestionStore interface {
	// Clear completely clears the datastore. Mostly used for testing.
	Clear() error

	// SetResultFileHash sets the given md5 hash in the database.
	SetResultFileHash(md5 string) error

	// ContainsResultFileHash returns true if the provided md5 hash is in the DB.
	ContainsResultFileHash(md5 string) (bool, error)
}

// Ingester is the main type that drives ingestion for a single type.
type Ingester struct {
	id             string
	vcs            vcsinfo.VCS
	nCommits       int
	minDuration    time.Duration
	runEvery       time.Duration
	sources        []Source
	processor      Processor
	doneCh         chan bool
	statusDB       *bolt.DB // Deprecated: To be removed when all ingesters use the IngestionStore
	ingestionStore IngestionStore
	eventBus       eventbus.EventBus

	// eventProcessMetrics contains all events we are interested in.
	eventProcessMetrics *processMetrics
}

// NewIngester creates a new ingester with the given id and configuration around
// the supplied vcs (version control system), input sources and Processor instance.
// The ingester is event driven by storage events with a background process that polls
// the storage locations. The given eventBus cannot be nil and must be shared with the sources
// that are passed. To only do polling-based ingestion use an in-memory eventbus
// (created via eventbus.New()). To drive ingestion from storage events use a PubSub-based
// eventbus (created via the gevent.New(...) function).
//
func NewIngester(ingesterID string, ingesterConf *sharedconfig.IngesterConfig, vcs vcsinfo.VCS, sources []Source, processor Processor, ingestionStore IngestionStore, eventBus eventbus.EventBus) (*Ingester, error) {
	if eventBus == nil {
		return nil, sklog.FmtErrorf("eventBus cannot be nil")
	}

	// TODO(stephana): Remove once all instances have been moved to BigTable and
	// all live data have moved to BT.
	var statusDB *bolt.DB
	var err error
	if ingesterConf.StatusDir != "" {
		statusDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(ingesterConf.StatusDir, ingesterID)))
		dbName := filepath.Join(statusDir, fmt.Sprintf("%s-status.db", ingesterID))
		statusDB, err = bolt.Open(dbName, 0600, &bolt.Options{Timeout: 1 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("Unable to open db at %s. Got error: %s", dbName, err)
		}
	}

	ret := &Ingester{
		id:                  ingesterID,
		vcs:                 vcs,
		nCommits:            ingesterConf.NCommits,
		minDuration:         time.Duration(ingesterConf.MinDays) * time.Hour * 24,
		runEvery:            ingesterConf.RunEvery.Duration,
		sources:             sources,
		processor:           processor,
		statusDB:            statusDB,
		ingestionStore:      ingestionStore,
		eventBus:            eventBus,
		eventProcessMetrics: newProcessMetrics(ingesterID),
	}
	return ret, nil
}

// Start starts the ingester in a new goroutine.
func (i *Ingester) Start(ctx context.Context) error {
	concurrentProc := make(chan bool, nConcurrentProcessors)
	resultChan, err := i.getInputChannel(ctx)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving input channel: %s", err)
	}

	// Continuously catch events from all input sources and push the data to the processor.
	go func(doneCh <-chan bool) {
		var resultFile ResultFileLocation = nil
		for {
			select {
			case resultFile = <-resultChan:
			case <-doneCh:
				return
			}

			// get a slot in line to call Process
			concurrentProc <- true
			go func(resultFile ResultFileLocation) {
				defer func() { <-concurrentProc }()
				i.processResult(ctx, resultFile)
			}(resultFile)
		}
	}(i.doneCh)
	return nil
}

// stop stops the ingestion process. Currently only used for testing.
func (i *Ingester) stop() {
	close(i.doneCh)
}

func (i *Ingester) getInputChannel(ctx context.Context) (<-chan ResultFileLocation, error) {
	eventChan := make(chan ResultFileLocation, eventChanSize)
	i.doneCh = make(chan bool)

	for _, source := range i.sources {
		if err := source.SetEventChannel(eventChan); err != nil {
			return nil, sklog.FmtErrorf("Error setting event channel: %s", err)
		}

		// Watch the source and feed anything not found in the IngestionStore
		go i.watchSource(source)
	}
	return eventChan, nil
}

// watchSource starts a background process that poll the given source in
// scheduled intervals (controlled by i.runEvery) and generates synthetic
// storage events if the files in the source have not been ingested yet.
func (i *Ingester) watchSource(source Source) {
	sklog.Infof("Watching source %s", source.ID())
	ticker := time.NewTicker(i.runEvery)
	defer ticker.Stop()

	for {
		select {
		case <-i.doneCh:
			return
		case <-ticker.C:
			// Get the start of the time range that we are polling.
			startTime, err := i.getStartTimeOfInterest(context.TODO())
			if err != nil {
				sklog.Errorf("Unable to get commit range of interest: %s", err)
				return
			}

			rfCh := source.Poll(startTime, time.Now().Unix())
			processed := int64(0)
			ignored := int64(0)
			for rf := range rfCh {
				if i.inProcessedFiles(rf.MD5()) {
					ignored++
					continue
				}
				processed++

				bucketID, objectID := rf.StorageIDs()
				i.eventBus.PublishStorageEvent(eventbus.NewStorageEvent(bucketID, objectID,
					rf.TimeStamp(), rf.MD5()))
			}
			i.eventProcessMetrics.ignoredByPollingGauge.Update(ignored)
			i.eventProcessMetrics.processedByPollingGauge.Update(processed)
			sklog.Infof("Watcher received/ignored: %d/%d", ignored+processed, ignored)
		}
	}
}

// inProcessedFiles returns true if the given md5 hash is in the list of
// already processed files.
func (i *Ingester) inProcessedFiles(md5 string) bool {
	if i.ingestionStore != nil {
		ret, err := i.ingestionStore.ContainsResultFileHash(md5)
		if err != nil {
			sklog.Errorf("Error checking ingestionstore: %s", err)
			return false
		}

		// If we have confirmed the file exists then return true otherwise see
		// if it's in the deprecated database.
		if ret {
			return true
		}
	}

	if i.statusDB == nil {
		return false
	}

	ret := false
	getFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(PROCESSED_FILES_BUCKET))
		if bucket == nil {
			return nil
		}

		ret = bucket.Get([]byte(md5)) != nil
		return nil
	}

	if err := i.statusDB.View(getFn); err != nil {
		sklog.Errorf("Error reading from bucket %s: %s", PROCESSED_FILES_BUCKET, err)
	}
	return ret
}

// addToProcessedFiles adds the given list of md5 hashes to the list of
// file that have been already processed.
func (i *Ingester) addToProcessedFiles(md5 string) {
	if i.ingestionStore != nil {
		if err := i.ingestionStore.SetResultFileHash(md5); err != nil {
			sklog.Errorf("Error setting md5 in ingestionstore: %s", err)
		}
		return
	}

	if i.statusDB == nil {
		sklog.Errorf("No viable database configured for ingestion.")
		return
	}

	updateFn := func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(PROCESSED_FILES_BUCKET))
		if err != nil {
			return err
		}

		if err := bucket.Put([]byte(md5), []byte{}); err != nil {
			return err
		}
		return nil
	}

	if err := i.statusDB.Update(updateFn); err != nil {
		sklog.Errorf("Error writing to bucket %s/%s: %s", PROCESSED_FILES_BUCKET, md5, err)
	}
}

// processResult processes a single result file.
func (i *Ingester) processResult(ctx context.Context, resultLocation ResultFileLocation) {
	resultMD5 := resultLocation.MD5()
	if i.inProcessedFiles(resultMD5) {
		return
	}

	err := i.processor.Process(ctx, resultLocation)
	if err != nil {
		if err != IgnoreResultsFileErr {
			sklog.Errorf("Failed to ingest %s: %s", resultLocation.Name(), err)
			return
		}
	}
	i.addToProcessedFiles(resultMD5)
	i.eventProcessMetrics.liveness.Reset()
}

// getStartTimeOfInterest returns the start time of input files we are interested in.
// We will then poll for input files from startTime to now. This method assumes that
// UpdateCommitInfo has been called and therefore reading the tile should not fail.
func (i *Ingester) getStartTimeOfInterest(ctx context.Context) (int64, error) {
	// If there is no vcs, use the minDuration field of the ingester to calculate
	// the start time.
	if i.vcs == nil {
		return time.Now().Add(-i.minDuration).Unix(), nil
	}

	// Make sure the VCS is up to date.
	if err := i.vcs.Update(ctx, true, false); err != nil {
		return 0, err
	}

	// Get the desired number of commits in the desired time frame.
	delta := -i.minDuration
	hashes := i.vcs.From(time.Now().Add(delta))
	if len(hashes) == 0 {
		return 0, fmt.Errorf("No commits found.")
	}

	// If the number of required commits is not covered by this time
	// frame then keep adding more.
	if len(hashes) < i.nCommits {
		for len(hashes) < i.nCommits {
			delta *= 2
			moreHashes := i.vcs.From(time.Now().Add(delta))
			if len(moreHashes) == len(hashes) {
				hashes = moreHashes
				break
			}
			hashes = moreHashes
		}

		// In case we have retrieved to many commits.
		if len(hashes) > i.nCommits {
			hashes = hashes[len(hashes)-i.nCommits:]
		}
	}

	// Get the commit time of the first commit of interest.
	detail, err := i.vcs.Details(ctx, hashes[0], true)
	if err != nil {
		return 0, err
	}

	return detail.Timestamp.Unix(), nil
}

// Shorthand type to define helpers.
type tags map[string]string

// processMetrics contains the metrics we are interested for processing results.
type processMetrics struct {
	ignoredByPollingGauge   metrics2.Int64Metric
	processedByPollingGauge metrics2.Int64Metric
	liveness                metrics2.Liveness
}

// TODO(stephana): Remove the "poll" value below, this is to have continuity with existing metrics.

// newProcessMetrics instantiates the metrics to track processing and registers them
// with the metrics package.
func newProcessMetrics(id string) *processMetrics {
	commonTags := tags{TAG_INGESTER_ID: id, TAG_INGESTER_SOURCE: "poll"}
	return &processMetrics{
		ignoredByPollingGauge:   metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, tags{TAG_INGESTION_METRIC: "ignored"}),
		processedByPollingGauge: metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, tags{TAG_INGESTION_METRIC: "processed"}),
		liveness:                metrics2.NewLiveness(id, tags{TAG_INGESTER_SOURCE: "poll", TAG_INGESTION_METRIC: "since-last-run"}),
	}
}
