package expstorage

import (
	"context"
	"math"
	"sync"
	"time"

	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// ExpectationsStore defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests.
	Get(ctx context.Context) (expectations.ReadOnly, error)

	// GetCopy a copy of the current classifications, safe for mutating.
	GetCopy(ctx context.Context) (*expectations.Expectations, error)

	// AddChange writes the given classified digests to the database and records the
	// user that made the change. If two users are modifying changes at the same time, last one
	// in wins.
	AddChange(ctx context.Context, changes []Delta, userId string) error

	// QueryLog returns a list of n entries starting at the given offset.
	// If it is computationally cheap to do so, the second return value can be
	// a count of the total number of CLs, or CountMany otherwise.
	QueryLog(ctx context.Context, offset, n int, details bool) ([]TriageLogEntry, int, error)

	// UndoChange reverts a change by setting all testname/digest pairs of the
	// original change to the label they had before the change was applied.
	// A new entry is added to the log with a reference to the change that was
	// undone.
	UndoChange(ctx context.Context, changeID, userID string) error

	// ForChangeList returns a new ExpectationStore that will deal with the Expectations for a
	// ChangeList with the given id (aka a CLExpectations). Any Expectations added to the returned
	// ExpectationStore will be kept separate from the master branch. Any Expectations
	// returned should be treated as the delta between the MasterBranch and the given issue.
	// The parameter crs is the CodeReviewSystem (e.g. "gerrit", "github") and id is the id
	// of the CL in that CRS. (This allows us to avoid a collision between two CLs with the same
	// id in the event that we transition from one CRS to another).
	ForChangeList(id, crs string) ExpectationsStore
}

// Delta represents one changed digest and the label that was
// assigned as part of the triage operation.
type Delta struct {
	Grouping types.TestName
	Digest   types.Digest
	Label    expectations.Label
}

// AsDelta converts an Expectations object into a slice of Deltas.
func AsDelta(e expectations.ReadOnly) []Delta {
	var delta []Delta
	_ = e.ForAll(func(tn types.TestName, d types.Digest, l expectations.Label) error {
		delta = append(delta, Delta{Grouping: tn, Digest: d, Label: l})
		return nil
	})
	return delta
}

// TriageLogEntry represents a set of changes by a single person.
type TriageLogEntry struct {
	ID          string
	User        string
	TS          time.Time
	ChangeCount int
	Details     []Delta
}

// CountMany indicates it is computationally expensive to determine exactly how many
// items there are.
var CountMany = math.MaxInt32

// ChangeNotifier represents a type that will be called when the master branch expectations change.
type ChangeNotifier interface {
	NotifyChange(Delta)
}

// ChangeListener represents a callback to respond to a master branch expectations change.
type ChangeListener interface {
	ListenForChange(func(Delta))
}

// EventHandler implements the ChangeListener and ChangeNotifier interfaces. Calls to NotifyChange
// will be piped to any registered "ListenForChange" callbacks.
type EventHandler struct {
	isSync bool

	callbacks     []func(Delta)
	callbackMutex sync.Mutex
}

// NewEventHandler returns an empty event handler. Tests should set synchronous equal to true.
func NewEventHandler(synchronous bool) *EventHandler {
	return &EventHandler{
		isSync: synchronous,
	}
}

// NotifyChange implements the ChangeNotifier interface.
func (e *EventHandler) NotifyChange(d Delta) {
	e.callbackMutex.Lock()
	defer e.callbackMutex.Unlock()
	for _, fn := range e.callbacks {
		if e.isSync {
			fn(d)
		} else {
			go fn(d)
		}
	}
}

// ListenForChange implements the ChangeListener interface.
func (e *EventHandler) ListenForChange(fn func(Delta)) {
	e.callbackMutex.Lock()
	defer e.callbackMutex.Unlock()
	e.callbacks = append(e.callbacks, fn)
}
