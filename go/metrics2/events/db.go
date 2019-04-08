package events

import (
	"time"
)

// EventDB is an interface used for storing Events in a BoltDB.
type EventDB interface {
	// Append inserts an Event with the given data into the given stream at
	// the current time.
	Append(string, []byte) error
	// Close frees up resources used by the eventDB.
	Close() error
	// Insert inserts the given Event into DB.
	Insert(*Event) error
	// Range returns all Events in the given range from the given stream.
	// The beginning of the range is inclusive, while the end is exclusive.
	Range(string, time.Time, time.Time) ([]*Event, error)
}
