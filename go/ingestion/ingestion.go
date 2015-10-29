package ingestion

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/fileutil"
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
	Open() io.ReadCloser

	// Name returns the full path of the file. The last segment is usually the
	// the file name.
	Name() string

	// MD5 returns the MD5 hash of the content of the file.
	MD5() string
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

// Constructor is the signature that has to be implemented to register a
// Processor implementation to be instantiated by name from a config struct.
type Constructor func(config *sharedconfig.IngesterConfig) (Processor, error)

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

// Ingester is the main type that drives ingestion for a single type.
type Ingester struct {
	id           string
	vcs          vcsinfo.VCS
	nCommits     int
	minDuration  time.Duration
	runEvery     time.Duration
	sources      []Source
	processor    Processor
	stopChannels []chan<- bool
	statusDB     *bolt.DB
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

	return ret, nil
}

// Start starts the ingester in a new goroutine.
func (i *Ingester) Start() {
	pollChan, eventChan := i.getInputChannels()
	stopCh := make(chan bool)
	i.stopChannels = append(i.stopChannels, stopCh)

	go func(stopCh <-chan bool) {
		var resultFiles []ResultFileLocation = nil
		var fromPolling bool

	MainLoop:
		for {
			select {
			case resultFiles = <-pollChan:
				fromPolling = true
			case resultFiles = <-eventChan:
				fromPolling = false
			case <-stopCh:
				break MainLoop
			}
			i.processResults(resultFiles, fromPolling)
		}
	}(stopCh)
}

// stop stops the ingestion process. Currently only used for testing.
func (i *Ingester) stop() {
	for _, ch := range i.stopChannels {
		ch <- true
	}
	util.Close(i.statusDB)
}

func (i *Ingester) getInputChannels() (<-chan []ResultFileLocation, <-chan []ResultFileLocation) {
	pollChan := make(chan []ResultFileLocation)
	eventChan := make(chan []ResultFileLocation)
	i.stopChannels = make([]chan<- bool, 0, len(i.sources))

	for _, source := range i.sources {
		stopCh := make(chan bool)
		go func(source Source, stopCh <-chan bool) {
			util.Repeat(i.runEvery, stopCh, func() {
				var startTime, endTime int64 = 0, 0
				startTime, endTime, err := i.getCommitRangeOfInterest()
				if err != nil {
					glog.Errorf("Unable to retrieve the start and end time. Got error: %s", err)
					return
				}

				resultFiles, err := source.Poll(startTime, endTime)
				if err != nil {
					glog.Errorf("Error polling data source '%s': %s", source.ID(), err)
					return
				}
				pollChan <- resultFiles
			})
		}(source, stopCh)
		i.stopChannels = append(i.stopChannels, stopCh)

		if ch := source.EventChan(); ch != nil {
			stopCh := make(chan bool)
			go func(ch <-chan []ResultFileLocation, stopCh <-chan bool) {
			MainLoop:
				for {
					select {
					case eventChan <- (<-ch):
					case <-stopCh:
						break MainLoop
					}
				}
			}(ch, stopCh)
			i.stopChannels = append(i.stopChannels, stopCh)
		}
	}
	return pollChan, eventChan
}

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

// processFiles ingests a set of result files.
func (i *Ingester) processResults(resultFiles []ResultFileLocation, fromPolling bool) {
	glog.Infof("Start ingester: %s", i.id)

	processedMD5s := make([]string, 0, len(resultFiles))
	for _, resultLocation := range resultFiles {
		if !i.inProcessedFiles(resultLocation.MD5()) {
			if err := i.processor.Process(resultLocation); err != nil {
				glog.Errorf("Failed to ingest %s: %s", resultLocation.Name(), err)
				continue
			}

			// Gather all successfully processed MD5s
			processedMD5s = append(processedMD5s, resultLocation.MD5())
		}
		// TODO(stephana): Add a metrics to capture how often we skip files
		// because we have processed them already. Including a metric to capture
		// the percent of processed files that come from polling vs events.

	}

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

	for len(hashes) < i.nCommits {
		delta *= 2
		moreHashes := i.vcs.From(time.Now().Add(delta))
		if len(moreHashes) == len(hashes) {
			hashes = moreHashes
			break
		}
		hashes = moreHashes
	}

	if len(hashes) > i.nCommits {
		hashes = hashes[len(hashes)-i.nCommits:]
	}

	// Get the commit time of the first commit of interest.
	detail, err := i.vcs.Details(hashes[0])
	if err != nil {
		return 0, 0, err
	}

	return detail.Timestamp.Unix(), time.Now().Unix(), nil
}
