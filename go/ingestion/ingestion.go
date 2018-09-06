package ingestion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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

	POLL_CHUNK_SIZE = 50
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
	// Return a list of result files that originated between the given
	// timestamps in milliseconds.
	Poll(startTime, endTime int64) ([]ResultFileLocation, error)

	// ID returns a unique identifier for this source.
	ID() string

	// SetEventChannel configures storage events and sets up routines to send
	// new results to the given channel.
	SetEventChannel(resultCh chan<- []ResultFileLocation) error
}

// ResultFileLocation is an abstract interface to a file like object that
// contains results that need to be ingested.
type ResultFileLocation interface {
	// Open returns a reader that allows to read the content of the file.
	Open() (io.ReadCloser, error)

	// Name returns the full path of the file. The last segment is usually the
	// the file name.
	Name() string

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

	// srcMetrics capture a set of metrics for each input source.
	srcMetrics []*sourceMetrics

	// pollProcessMetrics capture metrics from processing polled result files.
	pollProcessMetrics *processMetrics

	// eventProcessMetrics capture metrics from processing result files delivered by events from sources.
	eventProcessMetrics *processMetrics

	// processTimer measure the overall time it takes to process a set of files.
	processTimer metrics2.Timer
}

// NewIngester creates a new ingester with the given id and configuration around
// the supplied vcs (version control system), input sources and Processor instance.
func NewIngester(ingesterID string, ingesterConf *sharedconfig.IngesterConfig, vcs vcsinfo.VCS, sources []Source, processor Processor, ingestionStore IngestionStore) (*Ingester, error) {

	// TODO(stephana): Remove once all instances have been moved to BigTable.
	var statusDB *bolt.DB
	var err error
	if ingestionStore == nil {
		statusDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(ingesterConf.StatusDir, ingesterID)))
		dbName := filepath.Join(statusDir, fmt.Sprintf("%s-status.db", ingesterID))
		statusDB, err = bolt.Open(dbName, 0600, &bolt.Options{Timeout: 1 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("Unable to open db at %s. Got error: %s", dbName, err)
		}
	}

	ret := &Ingester{
		id:             ingesterID,
		vcs:            vcs,
		nCommits:       ingesterConf.NCommits,
		minDuration:    time.Duration(ingesterConf.MinDays) * time.Hour * 24,
		runEvery:       ingesterConf.RunEvery.Duration,
		sources:        sources,
		processor:      processor,
		statusDB:       statusDB,
		ingestionStore: ingestionStore,
	}
	ret.setupMetrics()
	return ret, nil
}

// setupMetrics instantiates and registers the metrics instances used by the Ingester.
func (i *Ingester) setupMetrics() {
	i.pollProcessMetrics = newProcessMetrics(i.id, "poll")
	i.eventProcessMetrics = newProcessMetrics(i.id, "event")
	i.srcMetrics = newSourceMetrics(i.id, i.sources)
	i.processTimer = metrics2.NewTimer("ingestion_process", map[string]string{"id": i.id})
}

// Start starts the ingester in a new goroutine.
func (i *Ingester) Start(ctx context.Context) error {
	pollChan, eventChan, err := i.getInputChannels(ctx)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving input channels: %s", err)
	}
	go func(doneCh <-chan bool) {
		var resultFiles []ResultFileLocation = nil
		var useMetrics *processMetrics

		for {
			select {
			case resultFiles = <-pollChan:
				useMetrics = i.pollProcessMetrics
			case resultFiles = <-eventChan:
				useMetrics = i.eventProcessMetrics
			case <-doneCh:
				return
			}
			i.processResults(ctx, resultFiles, useMetrics)
		}
	}(i.doneCh)
	return nil
}

// stop stops the ingestion process. Currently only used for testing.
func (i *Ingester) stop() {
	close(i.doneCh)
}

// rflQueue is a helper type that implements a very simple queue to buffer ResultFileLocations.
type rflQueue []ResultFileLocation

// push appends the given result file locations to the queue.
func (q *rflQueue) push(items []ResultFileLocation) {
	*q = append(*q, items...)
}

// clear removes all elements from the queue.
func (q *rflQueue) clear() {
	*q = rflQueue{}
}

func (i *Ingester) getInputChannels(ctx context.Context) (<-chan []ResultFileLocation, <-chan []ResultFileLocation, error) {
	pollChan := make(chan []ResultFileLocation)
	eventChan := make(chan []ResultFileLocation)
	i.doneCh = make(chan bool)

	for idx, source := range i.sources {
		go func(source Source, srcMetrics *sourceMetrics, doneCh <-chan bool) {
			util.Repeat(i.runEvery, doneCh, func() {
				srcMetrics.pollTimer.Start()
				var startTime, endTime int64 = 0, 0
				startTime, endTime, err := i.getCommitRangeOfInterest(ctx)
				if err != nil {
					sklog.Errorf("Unable to retrieve the start and end time. Got error: %s", err)
					return
				}

				sklog.Infof("Polling range: %s - %s", time.Unix(startTime, 0), time.Unix(endTime, 0))
				// measure how long the polling takes.
				resultFiles, err := source.Poll(startTime, endTime)
				if err != nil {
					// Indicate that there was an error in polling the source.
					srcMetrics.pollError.Update(1)
					sklog.Errorf("Error polling data source '%s': %s", source.ID(), err)
					return
				}

				sklog.Infof("Sending pollChan from %s for %d files.", source.ID(), len(resultFiles))
				// Indicate that the polling was successful.
				srcMetrics.pollError.Update(0)
				for len(resultFiles) > 0 {
					chunkSize := util.MinInt(POLL_CHUNK_SIZE, len(resultFiles))
					pollChan <- resultFiles[:chunkSize]
					resultFiles = resultFiles[chunkSize:]
				}
				srcMetrics.liveness.Reset()
				srcMetrics.pollTimer.Stop()
			})
		}(source, i.srcMetrics[idx], i.doneCh)

		if err := source.SetEventChannel(eventChan); err != nil {
			return nil, nil, sklog.FmtErrorf("Error setting event channel: %s", err)
		}
	}
	return pollChan, eventChan, nil
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
		return ret
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
func (i *Ingester) addToProcessedFiles(md5s []string) {
	if i.ingestionStore != nil {
		var egroup errgroup.Group
		for _, md5 := range md5s {
			func(md5 string) {
				egroup.Go(func() error {
					return i.ingestionStore.SetResultFileHash(md5)
				})
			}(md5)
		}
		if err := egroup.Wait(); err != nil {
			sklog.Errorf("Error setting md5 in ingestionstore: %s", err)
		}
		return
	}

	updateFn := func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(PROCESSED_FILES_BUCKET))
		if err != nil {
			return err
		}

		for _, md5 := range md5s {
			if err := bucket.Put([]byte(md5), []byte{}); err != nil {
				return err
			}
		}
		return nil
	}

	if err := i.statusDB.Update(updateFn); err != nil {
		sklog.Errorf("Error writing to bucket %s/%v: %s", PROCESSED_FILES_BUCKET, md5s, err)
	}
}

// processResults ingests a set of result files.
func (i *Ingester) processResults(ctx context.Context, resultFiles []ResultFileLocation, targetMetrics *processMetrics) {

	var mutex sync.Mutex // Protects access to the following vars.
	processedMD5s := make([]string, 0, len(resultFiles))
	var processedCounter int64 = 0
	var ignoredCounter int64 = 0
	var errorCounter int64 = 0

	// time how long the overall process takes.
	i.processTimer.Start()
	var wg sync.WaitGroup
	for _, resultLocation := range resultFiles {
		if i.inProcessedFiles(resultLocation.MD5()) {
			mutex.Lock()
			ignoredCounter++
			mutex.Unlock()
			continue
		}
		wg.Add(1)
		go func(resultLocation ResultFileLocation) {
			defer wg.Done()
			defer metrics2.NewTimer("ingestion_process_file", map[string]string{"id": i.id}).Stop()
			err := i.processor.Process(ctx, resultLocation)

			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				if err == IgnoreResultsFileErr {
					ignoredCounter++
				} else {
					errorCounter++
					sklog.Errorf("Failed to ingest %s: %s", resultLocation.Name(), err)
					return
				}
			}

			// Gather all successfully processed MD5s
			processedCounter++
			processedMD5s = append(processedMD5s, resultLocation.MD5())
		}(resultLocation)
	}
	wg.Wait()
	targetMetrics.liveness.Reset()

	// Update the timer and the gauges that measure how the ingestion works
	// for the input type.
	i.processTimer.Stop()
	targetMetrics.totalFilesGauge.Update(int64(len(resultFiles)) + targetMetrics.totalFilesGauge.Get())
	targetMetrics.processedGauge.Update(processedCounter + targetMetrics.processedGauge.Get())
	targetMetrics.ignoredGauge.Update(ignoredCounter + targetMetrics.ignoredGauge.Get())
	targetMetrics.errorGauge.Update(errorCounter + targetMetrics.errorGauge.Get())

	i.addToProcessedFiles(processedMD5s)
}

// getCommitRangeOfInterest returns the time range (start, end) that
// we are interested in. This method assumes that UpdateCommitInfo
// has been called and therefore reading the tile should not fail.
func (i *Ingester) getCommitRangeOfInterest(ctx context.Context) (int64, int64, error) {
	// If there is no vcs, use the minDuration field of the ingester to calculate
	// the start time.
	if i.vcs == nil {
		return time.Now().Add(-i.minDuration).Unix(), time.Now().Unix(), nil
	}

	// Make sure the VCS is up to date.
	if err := i.vcs.Update(ctx, true, false); err != nil {
		return 0, 0, err
	}

	// Get the desired number of commits in the desired time frame.
	delta := -i.minDuration
	hashes := i.vcs.From(time.Now().Add(delta))
	if len(hashes) == 0 {
		return 0, 0, fmt.Errorf("No commits found.")
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
		return 0, 0, err
	}

	return detail.Timestamp.Unix(), time.Now().Unix(), nil
}

// Shorthand type to define helpers.
type tags map[string]string

// processMetrics contains the metrics we are interested for processing results.
// We have one instance for polled result files and one for files that were
// delievered via events.
type processMetrics struct {
	totalFilesGauge metrics2.Int64Metric
	processedGauge  metrics2.Int64Metric
	ignoredGauge    metrics2.Int64Metric
	errorGauge      metrics2.Int64Metric
	liveness        metrics2.Liveness
}

// newProcessMetrics instantiates the metrics to track processing and registers them
// with the metrics package.
func newProcessMetrics(id, subtype string) *processMetrics {
	commonTags := tags{TAG_INGESTER_ID: id, TAG_INGESTER_SOURCE: subtype}
	return &processMetrics{
		totalFilesGauge: metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, tags{TAG_INGESTION_METRIC: "total"}),
		processedGauge:  metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, tags{TAG_INGESTION_METRIC: "processed"}),
		ignoredGauge:    metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, tags{TAG_INGESTION_METRIC: "ignored"}),
		errorGauge:      metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, tags{TAG_INGESTION_METRIC: "errors"}),
		liveness:        metrics2.NewLiveness(id, tags{TAG_INGESTER_SOURCE: subtype, TAG_INGESTION_METRIC: "since-last-run"}),
	}
}

// sourceMetrics tracks metrics for one input source.
type sourceMetrics struct {
	liveness       metrics2.Liveness
	pollTimer      metrics2.Timer
	pollError      metrics2.Int64Metric
	eventsReceived metrics2.Int64Metric
}

// newSourceMetrics instantiates a set of metrics for an input source.
func newSourceMetrics(id string, sources []Source) []*sourceMetrics {
	ret := make([]*sourceMetrics, len(sources))
	commonTags := tags{TAG_INGESTER_ID: id}
	for idx, source := range sources {
		srcTags := tags{TAG_INGESTER_SOURCE: source.ID()}
		ret[idx] = &sourceMetrics{
			liveness:       metrics2.NewLiveness(id, srcTags, tags{TAG_INGESTION_METRIC: "src-last-run"}),
			pollTimer:      metrics2.NewTimer(MEASUREMENT_INGESTION, commonTags, srcTags, tags{TAG_INGESTION_METRIC: "poll_timer"}),
			pollError:      metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, srcTags, tags{TAG_INGESTION_METRIC: "poll_error"}),
			eventsReceived: metrics2.GetInt64Metric(MEASUREMENT_INGESTION, commonTags, srcTags, tags{TAG_INGESTION_METRIC: "events"}),
		}
	}
	return ret
}
