package ingestion

import (
	"context"
	"errors"
	"io"
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

	// TimeStamp returns the timestamp when the file was last updated.
	TimeStamp() int64

	// Content returns the content of the file if has been read or nil otherwise.
	Content() []byte
}

// Processor is the core of an Ingester. It takes instances of ResultFileLocation
// and ingests them. It is responsible for the storage of ingested data.
type Processor interface {
	// Process ingests a single result file.
	Process(ctx context.Context, resultsFile ResultFileLocation) error
}

// IngestionStore keeps track of files being ingested based on their MD5 hashes.
type IngestionStore interface {
	// SetResultFileHash indicates that we have ingested the given filename
	// with the given md5hash.
	// TODO(kjlubick) add context.Context to this interface
	SetResultFileHash(ctx context.Context, fileName, md5 string) error

	// ContainsResultFileHash returns true if the provided file and md5 hash
	// were previously set with SetResultFileHash.
	ContainsResultFileHash(ctx context.Context, fileName, md5 string) (bool, error)
}
