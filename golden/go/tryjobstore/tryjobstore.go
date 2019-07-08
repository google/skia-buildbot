package tryjobstore

const (
	// EV_TRYJOB_UPDATED is the event that is fired when a tryjob is updated (update or creation).
	EV_TRYJOB_UPDATED = "tryjobstore:tryjob-updated"
)

// NewValueFn is a callback function that allows to update the value of
// datastore entity within a transation. It receives the current value an
// entity and returns the updated value or nil, if it does not want to update
// the current value.
type NewValueFn func(data interface{}) interface{}

// TryjobStore define methods to store tryjob information and code review
// issues as a key component for transactional trybot support.
type TryjobStore interface {
	// ListIssues lists all current issues in the store. The offset and size are
	// used for pagination. 'offset' defines the starting index (zero based) of the
	// page and size defines the size of the page.
	// The function returns a a list of issues and the total number of issues.
	ListIssues(offset, size int) ([]*Issue, int, error)

	// GetIssue retrieves information about the given issue and patchsets. If needded
	// this will include tryjob information.
	GetIssue(issueID int64, loadTryjobs bool) (*Issue, error)

	// UpdateIssue updates the given issue with the provided data. If the issue does not
	// exist in the database it will be created. If updateFn is nil, issue will be
	// written to the database unconditionally, updateFn is used as described above.
	UpdateIssue(details *Issue, updateFn NewValueFn) error

	// CommitIssueExp commits the expectations of the given issue. The writeFn
	// is expected to make the changes to the master baseline. An issue is
	// marked as committed if the writeFn runs without error.
	CommitIssueExp(issueID int64, writeFn func() error) error

	// GetTryjobs returns the Tryjobs for given issues. If filterDup is true it
	// will also filter duplicate tryjobs for each patchset and only keep the newest.
	// If loadResults is true it will also load the Tryjob results. The second
	// return value will contain the results of the Tryjob with same index in the
	// first return value.
	GetTryjobs(issueID int64, patchsetIDs []int64, filterDup bool, loadResults bool) ([]*Tryjob, [][]*TryjobResult, error)

	// RunningTryjobs returns a list of tryjobs that are considered running by
	// the datastore (their status is less than TRYJOB_COMPLETE)
	RunningTryjobs() ([]*Tryjob, error)

	// GetTryjob returns the Tryjob instance defined by issueID and buildBucketID.
	GetTryjob(issueID, buildBucketID int64) (*Tryjob, error)

	// GetTryjobResults returns the results for the given Tryjobs.
	// This is intended to be used when we have a list of Tryjobs already
	// and we want to avoid another trip to the database to fetch them. The
	// return slice will match the indices of the input slice.
	GetTryjobResults(tryjobs []*Tryjob) ([][]*TryjobResult, error)

	// UpdateTryjob updates the information about a tryjob. If the tryjob does not
	// exist it will be created. If tryjob is not nil it will be written to the
	// datastore if it is newer than the current entity.
	// If tryjob is nil, then the buildBucketID and newValFn are used to load the
	// current value and update it. If the current entity does not exist an error
	// is returned.
	UpdateTryjob(buildBucketID int64, tryjob *Tryjob, newValFn NewValueFn) error

	// UpdateTryjobResult updates the results for the given tryjob. It assumes that the
	// BuildBucketID field in the given instances of TryjobResult are set correctly, thus
	// linking it to the instance of Tryjob that corresponds to that BuildBucketID.
	UpdateTryjobResult(results []*TryjobResult) error
}
