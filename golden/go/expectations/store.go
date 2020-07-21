package expectations

import (
	"context"
	"math"
	"sync"
	"time"

	"go.skia.org/infra/golden/go/types"
)

// Store defines the interface for persisting expectations.
type Store interface {
	// Get a snapshot of the current classifications for image digests.
	Get(ctx context.Context) (ReadOnly, error)

	// GetCopy a copy of the current classifications, safe for mutating.
	GetCopy(ctx context.Context) (*Expectations, error)

	// AddChange writes the given classified digests to the database and records the user that
	// made the change. If two users are modifying changes at the same time, last one in wins.
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

	// ForChangeList returns a new Store that will deal with the Expectations for a ChangeList
	// with the given id (aka a CLExpectations). Any Expectations added to the returned Store
	// will be kept separate from the master branch. Any Expectations returned should be
	// treated as the delta between the MasterBranch and the given issue. The parameter crs is
	// the CodeReviewSystem (e.g. "gerrit", "github") and id is the id of the CL in that CRS.p
	// (This allows us to avoid a collision between two CLs with the same id in the event that
	// we transition from one CRS to another).
	ForChangeList(id, crs string) Store

	// GetTriageHistory returns a slice of TriageHistory structs that indicate the last edits
	// for a given grouping/digest. If not nil, the return value should be sorted such that
	// the most recent entry is first.
	GetTriageHistory(ctx context.Context, grouping types.TestName, digest types.Digest) ([]TriageHistory, error)
}

// GarbageCollector encapsulates methods that can be used to clean up expectations not used in a
// configurable amount of time.
type GarbageCollector interface {
	// UpdateLastUsed will bulk update the given IDs as last used on the given time. This will
	// not impact the "modified" timestamp for an entry. This bulk operation will not show up in
	// the triage log.
	UpdateLastUsed(context.Context, []ID, time.Time) error

	// MarkUnusedEntriesForGC marks entries matching the given label for garbage collecting, provided
	// they have a modified ts and a last used ts before the given ts. It returns the number of
	// affected entries or an error if there were issues. This bulk operation need not appear in the
	// triage log. It does not affect CL expectations.
	MarkUnusedEntriesForGC(context.Context, LabelInt, time.Time) (int, error)

	// GarbageCollect removes all entries that have previously been marked for GC. It returns the
	// number of affected entries or an error if there were issues. This bulk operation will not
	// appear in the triage log.
	GarbageCollect(context.Context) (int, error)
}

// Delta represents one changed digest and the label that was
// assigned as part of the triage operation.
type Delta struct {
	Grouping types.TestName
	Digest   types.Digest
	Label    Label
}

// ID returns the ID for the Delta, as a method of convenience.
func (d Delta) ID() ID {
	return ID{
		Grouping: d.Grouping,
		Digest:   d.Digest,
	}
}

// DeltaWithRange is unused. It represents the likely change that would be required to add support
// for expectation ranges.
type DeltaWithRange struct {
	Delta
	FirstIndex int
	LastIndex  int
}

// AsDelta converts an Expectations object into a slice of Deltas.
func AsDelta(e ReadOnly) []Delta {
	var delta []Delta
	_ = e.ForAll(func(tn types.TestName, d types.Digest, l Label) error {
		delta = append(delta, Delta{Grouping: tn, Digest: d, Label: l})
		return nil
	})
	return delta
}

// TriageHistory represents the changes that happened to a given expectation entry.
type TriageHistory struct {
	User string
	TS   time.Time
}

// ID represents a unique identifier for an entry in the Store.
type ID struct {
	Grouping types.TestName
	Digest   types.Digest
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
	NotifyChange(ID)
}

// ChangeEventRegisterer allows for the registration of callbacks that are called when the
// expectations on the master branch change.
type ChangeEventRegisterer interface {
	ListenForChange(func(ID))
}

// ChangeEventDispatcher implements the ChangeEventRegisterer and ChangeNotifier interfaces. Calls to NotifyChange
// will be piped to any registered "ListenForChange" callbacks.
type ChangeEventDispatcher struct {
	isSyncForTesting bool

	callbacks     []func(ID)
	callbackMutex sync.Mutex
}

// NewEventDispatcher returns an empty event dispatcher to be used by production/
func NewEventDispatcher() *ChangeEventDispatcher {
	return &ChangeEventDispatcher{
		isSyncForTesting: false,
	}
}

// NewEventDispatcherForTesting returns an empty event dispatcher to be used for testing. Calls
// to NotifyChange will synchronously call all registered callbacks and then return.
func NewEventDispatcherForTesting() *ChangeEventDispatcher {
	return &ChangeEventDispatcher{
		isSyncForTesting: true,
	}
}

// NotifyChange implements the ChangeNotifier interface.
func (e *ChangeEventDispatcher) NotifyChange(d ID) {
	e.callbackMutex.Lock()
	defer e.callbackMutex.Unlock()
	for _, fn := range e.callbacks {
		if e.isSyncForTesting {
			fn(d)
		} else {
			go fn(d)
		}
	}
}

// ListenForChange implements the ChangeEventRegisterer interface.
func (e *ChangeEventDispatcher) ListenForChange(fn func(ID)) {
	e.callbackMutex.Lock()
	defer e.callbackMutex.Unlock()
	e.callbacks = append(e.callbacks, fn)
}

// Make sure ChangeEventDispatcher implements the ChangeEventRegisterer interface.
var _ ChangeEventRegisterer = (*ChangeEventDispatcher)(nil)

// Make sure ChangeEventDispatcher implements the ChangeNotifier interface.
var _ ChangeNotifier = (*ChangeEventDispatcher)(nil)
