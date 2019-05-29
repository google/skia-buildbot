package expstorage

import (
	"cloud.google.com/go/datastore"
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

	// MasterIssueID is the value used for IssueID when we dealing with the
	// master branch. Any IssueID < 0 should be ignored.
	MasterIssueID = -1
)

func init() {
	// Register the codec for EV_EXPSTORAGE_CHANGED so we can have distributed events.
	gevent.RegisterCodec(EV_EXPSTORAGE_CHANGED, util.JSONCodec(&EventExpectationChange{}))
}

// Defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests. The keys of the
	// expectations map are the test names.
	Get() (exp types.Expectations, err error)

	// AddChange writes the given classified digests to the database and records the
	// user that made the change.
	AddChange(changes types.Expectations, userId string) error

	// QueryLog allows to paginate through the changes in the expectations.
	// If details is true the result will include a list of triage operations
	// that were part a change.
	QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error)

	// UndoChange reverts a change by setting all testname/digest pairs of the
	// original change to the label they had before the change was applied.
	// A new entry is added to the log with a reference to the change that was
	// undone.
	UndoChange(changeID int64, userID string) (types.Expectations, error)

	// Clear deletes all expectations in this ExpectationsStore. This is mostly
	// used for testing, but also to delete the expectations for a Gerrit issue.
	// See the tryjobstore package.
	Clear() error
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
	// Note: The ID is a string because an int64 cannot be passed back and
	// forth to the JS frontend.
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	TS           int64           `json:"ts"`
	ChangeCount  int             `json:"changeCount"`
	Details      []*TriageDetail `json:"details"`
	UndoChangeID int64           `json:"undoChangeId"`
}

func (t *TriageLogEntry) GetChanges() types.Expectations {
	ret := types.Expectations{}
	for _, d := range t.Details {
		label := types.LabelFromString(d.Label)
		if found, ok := ret[d.TestName]; !ok {
			ret[d.TestName] = types.TestClassification{d.Digest: label}
		} else {
			found[d.Digest] = label
		}
	}
	return ret
}

// ExpChange is used to store an expectation change in the database. Each
// expectation change is an atomic change to expectations for an issue.
// The actual expectations are captured in instances of TestDigestExp.
type ExpChange struct {
	ChangeID         *datastore.Key `datastore:"__key__"`
	IssueID          int64
	UserID           string
	TimeStamp        int64 `datastore:",noindex"`
	Count            int64 `datastore:",noindex"`
	UndoChangeID     int64
	OK               bool
	ExpectationsBlob *datastore.Key `datastore:",noindex"`
}

// EventExpectationChange is the structure that is sent in expectation change events.
// When the change happened on the master branch 'IssueID' will contain a value <0
// and should be ignored.
type EventExpectationChange struct {
	IssueID     int64
	TestChanges types.Expectations

	// waitCh is used by the sender of the event to wait for the event being handled.
	// It is not serialized and therefore not handled by distributed receivers, only locally.
	waitCh chan<- bool
}

// IssueExpStoreFactory creates an ExpectationsStore instance for the given issue id.
type IssueExpStoreFactory func(issueID int64) ExpectationsStore
