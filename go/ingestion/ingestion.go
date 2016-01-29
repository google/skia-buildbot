package ingestion

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/fileutil"
	smetrics "go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// BoltDB bucket where MD5 hashes of processed files are stored.
const PROCESSED_FILES_BUCKET = "processed_files"

// Source defines an ingestion source that returns lists of result files
// either through polling or in an event driven mode.
type Source interface {
	// Return a list of result files that originated between the given
	// timestamps in milliseconds.
	Poll(startTime, endTime int64) ([]ResultFileLocation, error)

	// EventChan returns a channel that sends lists of result files when they
	// are ready for processing. If this source does not support events it should
	// return nil.
	EventChan() <-chan []ResultFileLocation

	// ID returns a unique identifier for this source.
	ID() string
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
}

// Processor is the core of an ingester. It takes instances of ResultFileLocation
// and ingests them. It is responsible for the storage of ingested data.
type Processor interface {
	// Process ingests a single result file. It is either stores the file
	// immediately or updates the internal state of the processor and writes
	// data during the BatchFinished call.
	Process(resultsFile ResultFileLocation) error

	// BatchFinished is called when the current batch is finished. This is
	// to cover the case when ingestion is better done for the whole batch
	// This should reset the internal state of the Processor instance.
	BatchFinished() error
}

// Ingester is the main type that drives ingestion for a single type.
type Ingester struct {
	id          string
	vcs         vcsinfo.VCS
	nCommits    int
	minDuration time.Duration
	runEvery    time.Duration
	sources     []Source
	processor   Processor
	doneCh      chan bool
	statusDB    *bolt.DB

	// srcMetrics capture a set of metrics for each input source.
	srcMetrics []*sourceMetrics

	// pollProcessMetrics capture metrics from processing polled result files.
	pollProcessMetrics *processMetrics

	// eventProcessMetrics capture metrics from processing result files delivered by events from sources.
	eventProcessMetrics *processMetrics

	// processTimer measure the overall time it takes to process a set of files.
	processTimer metrics.Timer

	// processFileTimer measures how long it takes to process an individual file.
	processFileTimer metrics.Timer
}

// NewIngester creates a new ingester with the given id and configuration around
// the supplied vcs (version control system), input sources and Processor instance.
func NewIngester(ingesterID string, ingesterConf *sharedconfig.IngesterConfig, vcs vcsinfo.VCS, sources []Source, processor Processor) (*Ingester, error) {
	statusDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(ingesterConf.StatusDir, ingesterID)))
	dbName := filepath.Join(statusDir, fmt.Sprintf("%s-status.db", ingesterID))
	statusDB, err := bolt.Open(dbName, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("Unable to open db at %s. Got error: %s", dbName, err)
	}

	ret := &Ingester{
		id:          ingesterID,
		vcs:         vcs,
		nCommits:    ingesterConf.NCommits,
		minDuration: time.Duration(ingesterConf.MinDays) * time.Hour * 24,
		runEvery:    ingesterConf.RunEvery.Duration,
		sources:     sources,
		processor:   processor,
		statusDB:    statusDB,
	}
	ret.setupMetrics()
	return ret, nil
}

// setupMetrics instantiates and registers the metrics instances used by the Ingester.
func (i *Ingester) setupMetrics() {
	i.pollProcessMetrics = newProcessMetrics(i.id, "poll")
	i.eventProcessMetrics = newProcessMetrics(i.id, "event")
	i.srcMetrics = newSourceMetrics(i.id, i.sources)
	i.processTimer = metrics.NewRegisteredTimer(fmt.Sprintf("%s.process", i.id), metrics.DefaultRegistry)
	i.processFileTimer = metrics.NewRegisteredTimer(fmt.Sprintf("%s.process-file", i.id), metrics.DefaultRegistry)
}

// Start starts the ingester in a new goroutine.
func (i *Ingester) Start() {
	pollChan, eventChan := i.getInputChannels()
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
			i.processResults(resultFiles, useMetrics)
		}
	}(i.doneCh)
}

// stop stops the ingestion process. Currently only used for testing.
func (i *Ingester) stop() {
	close(i.doneCh)
}

// rflQueue is a helper type that implements a very simple queue to buffer ResultFileLcoations.
type rflQueue []ResultFileLocation

// push appends the given result file locations to the queue.
func (q *rflQueue) push(items []ResultFileLocation) {
	*q = append(*q, items...)
}

// clear removes all elements from the queue.
func (q *rflQueue) clear() {
	*q = rflQueue{}
}

func (i *Ingester) getInputChannels() (<-chan []ResultFileLocation, <-chan []ResultFileLocation) {
	pollChan := make(chan []ResultFileLocation)
	eventChan := make(chan []ResultFileLocation)
	i.doneCh = make(chan bool)

	for idx, source := range i.sources {
		go func(source Source, srcMetrics *sourceMetrics, doneCh <-chan bool) {
			util.Repeat(i.runEvery, doneCh, func() {
				var startTime, endTime int64 = 0, 0
				startTime, endTime, err := i.getCommitRangeOfInterest()
				if err != nil {
					glog.Errorf("Unable to retrieve the start and end time. Got error: %s", err)
					return
				}

				// measure how long the polling takes.
				pollStart := time.Now()
				resultFiles, err := source.Poll(startTime, endTime)
				srcMetrics.pollTimer.UpdateSince(pollStart)
				if err != nil {
					// Indicate that there was an error in polling the source.
					srcMetrics.pollError.Update(1)
					glog.Errorf("Error polling data source '%s': %s", source.ID(), err)
					return
				}

				// Indicate that the polling was successful.
				srcMetrics.pollError.Update(0)
				pollChan <- resultFiles
				srcMetrics.liveness.Update()
			})
		}(source, i.srcMetrics[idx], i.doneCh)

		if ch := source.EventChan(); ch != nil {
			go func(ch <-chan []ResultFileLocation, doneCh <-chan bool) {
				queue := rflQueue{}
				for {
					// If the queue is not empty, try to send the data while we wait for more data.
					if len(queue) > 0 {
						select {
						case eventChan <- queue:
							queue.clear()
						case results := <-ch:
							queue.push(results)
						case <-doneCh:
							return
						}
					} else {
						// if the queue is empty we just wait for data.
						select {
						case results := <-ch:
							queue.push(results)
						case <-doneCh:
							return
						}

					}
				}
			}(ch, i.doneCh)
		}
	}
	return pollChan, eventChan
}

// inProcessedFiles returns true if the given md5 hash is in the list of
// already processed files.
func (i *Ingester) inProcessedFiles(md5 string) bool {
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
		glog.Errorf("Error reading from bucket %s: %s", PROCESSED_FILES_BUCKET, err)
	}
	return ret
}

// addToProcessedFiles adds the given list of md5 hashes to the list of
// file that have been already processed.
func (i *Ingester) addToProcessedFiles(md5s []string) {
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
		glog.Errorf("Error writing to bucket %s/%v: %s", PROCESSED_FILES_BUCKET, md5s, err)
	}
}

// processResults ingests a set of result files.
func (i *Ingester) processResults(resultFiles []ResultFileLocation, targetMetrics *processMetrics) {
	glog.Infof("Start ingester: %s", i.id)

	processedMD5s := make([]string, 0, len(resultFiles))
	processedCounter, ignoredCounter, errorCounter := 0, 0, 0

	// time how long the overall process takes.
	processStart := time.Now()
	for _, resultLocation := range resultFiles {
		if !i.inProcessedFiles(resultLocation.MD5()) {
			// time how long it takes to process a file.
			processFileStart := time.Now()
			err := i.processor.Process(resultLocation)
			i.processFileTimer.UpdateSince(processFileStart)

			if err != nil {
				errorCounter++
				glog.Errorf("Failed to ingest %s: %s", resultLocation.Name(), err)
				continue
			}

			// Gather all successfully processed MD5s
			processedCounter++
			processedMD5s = append(processedMD5s, resultLocation.MD5())
		} else {
			ignoredCounter++
		}
	}

	// Update the timer and the gauges that measure how the ingestion works
	// for the input type.
	i.processTimer.UpdateSince(processStart)
	targetMetrics.totalFilesGauge.Update(int64(len(resultFiles)))
	targetMetrics.processedGauge.Update(int64(processedCounter))
	targetMetrics.ignoredGauge.Update(int64(ignoredCounter))
	targetMetrics.errorGauge.Update(int64(errorCounter))

	// Notify the ingester that the batch has finished and cause it to reset its
	// state and do any pending ingestion.
	if err := i.processor.BatchFinished(); err != nil {
		glog.Errorf("Batchfinished failed: %s", err)
	} else {
		i.addToProcessedFiles(processedMD5s)
	}

	glog.Infof("Finish ingester: %s", i.id)
}

// getCommitRangeOfInterest returns the time range (start, end) that
// we are interested in. This method assumes that UpdateCommitInfo
// has been called and therefore reading the tile should not fail.
func (i *Ingester) getCommitRangeOfInterest() (int64, int64, error) {
	// Make sure the VCS is up to date.
	if err := i.vcs.Update(true, false); err != nil {
		return 0, 0, err
	}

	// Get the desired numbr of commits in the desired time frame.
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
	detail, err := i.vcs.Details(hashes[0], true)
	if err != nil {
		return 0, 0, err
	}

	return detail.Timestamp.Unix(), time.Now().Unix(), nil
}

// processMetrics contains the metrics we are interested for processing results.
// We have one instance for polled result files and one for files that were
// delievered via events.
type processMetrics struct {
	totalFilesGauge metrics.Gauge
	processedGauge  metrics.Gauge
	ignoredGauge    metrics.Gauge
	errorGauge      metrics.Gauge
}

// newProcessMetrics instantiates the metrics to track processing and registers them
// with the metrics package.
func newProcessMetrics(id, subtype string) *processMetrics {
	prefix := fmt.Sprintf("%s.%s", id, subtype)
	return &processMetrics{
		totalFilesGauge: metrics.NewRegisteredGauge(prefix+".total", metrics.DefaultRegistry),
		processedGauge:  metrics.NewRegisteredGauge(prefix+".processed", metrics.DefaultRegistry),
		ignoredGauge:    metrics.NewRegisteredGauge(prefix+".ignored", metrics.DefaultRegistry),
		errorGauge:      metrics.NewRegisteredGauge(prefix+".errors", metrics.DefaultRegistry),
	}
}

// sourceMetrics tracks metrics for one input source.
type sourceMetrics struct {
	liveness       *smetrics.Liveness
	pollTimer      metrics.Timer
	pollError      metrics.Gauge
	eventsReceived metrics.Meter
}

// newSourceMetrics instantiates a set of metrics for an input source.
func newSourceMetrics(id string, sources []Source) []*sourceMetrics {
	ret := make([]*sourceMetrics, len(sources))
	for idx, source := range sources {
		prefix := fmt.Sprintf("%s.%s", id, source.ID())
		ret[idx] = &sourceMetrics{
			liveness:       smetrics.NewLiveness(prefix + ".poll-liveness"),
			pollTimer:      metrics.NewRegisteredTimer(prefix+".poll-timer", metrics.DefaultRegistry),
			pollError:      metrics.NewRegisteredGauge(prefix+".poll-error", metrics.DefaultRegistry),
			eventsReceived: metrics.NewRegisteredMeter(prefix+".events-received", metrics.DefaultRegistry),
		}
	}
	return ret
}
