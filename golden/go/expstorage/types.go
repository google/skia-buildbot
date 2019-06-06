package expstorage

import (
	"context"

	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// Events emitted by this package.
const (
	// Event emitted when expectations change.
	// Callback argument: []string with the names of changed tests.
	EV_EXPSTORAGE_CHANGED = "expstorage:changed"

	// EV_TRYJOB_EXP_CHANGED is the event type that is fired when the expectations
	// for an issue change. It sends an instance of *TryjobExpChange.
	EV_TRYJOB_EXP_CHANGED = "expstorage:tryjob-exp-change"

	// MasterBranch is the value used for IssueID when we dealing with the
	// master branch. Any other IssueID < 0 should be ignored.
	MasterBranch = int64(-1)
)

func init() {
	// Register the codec for EV_EXPSTORAGE_CHANGED so we can have distributed events.
	gevent.RegisterCodec(EV_EXPSTORAGE_CHANGED, util.JSONCodec(&EventExpectationChange{}))
}

// Defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests. The keys of the
	// expectations map are the test names.
	Get() (types.Expectations, error)

	// AddChange writes the given classified digests to the database and records the
	// user that made the change.
	// TODO(kjlubick): This interface leads to a potential race condition if two
	// users on the front-end click Positive and Negative for the same testname/digest.
	//  A less racy interface would take an "old value"/"new value" so that if the
	// old value didn't match, we could reject the change.
	AddChange(ctx context.Context, changes types.Expectations, userId string) error

	// QueryLog allows to paginate through the changes in the expectations.
	// If details is true the result will include a list of triage operations
	// that were part a change.
	QueryLog(ctx context.Context, offset, size int, details bool) ([]TriageLogEntry, int, error)

	// UndoChange reverts a change by setting all testname/digest pairs of the
	// original change to the label they had before the change was applied.
	// A new entry is added to the log with a reference to the change that was
	// undone. The expectations returned are the expectations that were changed,
	// with the newly reverted values.
	UndoChange(ctx context.Context, changeID, userID string) (types.Expectations, error)

	// Returns a new ExpectationStore that will deal with the Expectations for an issue
	// with the given id (aka an IssueExpectations). Any Expectations added to the returned
	// ExpectationStore will be kept separate from the master branch. Any Expectations
	// returned should be treated as the delta between the MasterBranch and the given issue.
	// This issue id is the Gerrit id or GitHub id.
	ForIssue(id int64) ExpectationsStore
}

// TriageDetails represents one changed digest and the label that was
// assigned as part of the triage operation.
type TriageDetail struct {
	TestName types.TestName `json:"test_name"`
	Digest   types.Digest   `json:"digest"`
	Label    string         `json:"label"`
}

// TriageLogEntry represents one change in the expectation store.
type TriageLogEntry struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	TS          int64          `json:"ts"` // is milliseconds since the epoch
	ChangeCount int            `json:"changeCount"`
	Details     []TriageDetail `json:"details"`
}

// EventExpectationChange is the structure that is sent in expectation change events.
// When the change happened on the master branch 'IssueID' will contain a value <0
// and should be ignored.
type EventExpectationChange struct {
	IssueID     int64
	TestChanges types.Expectations
}
