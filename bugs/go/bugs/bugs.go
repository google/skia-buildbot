package bugs

// CALL THIS bug_framework instead.

import (
	"time"
)

const ()

type Issue struct {
	Id       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	State    string `json:"state"`
	Priority string `json:"priority"`
	Owner    string `json:"owner"`
	Link     string `json:"link"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`
	ClosedTime   time.Time `json:"closed,omitempty"`

	// Maybe:
	//   Any other extra information like labels or tags or something else
}

type BugFramework interface {

	// GetBugFrameworkName returns the name of the bug framework. Eg: Monorail, IssueTracker, Github.
	GetBugFrameworkName() string

	// Search returns issues that match the provided parameters.
	Search(username string, statuses []string) ([]Issue, error)

	// Modifying might not be possible.. because will the service account modify or the actual user?

	// AddComment adds a comment to the specified issue.
	AddComment(i Issue, comment string) error

	// SetState sets a state to the specified issue.
	SetState(i Issue, state string) error

	// SetTitle

	// SetSummary

	// Should have a way to modify the title and summary as well...

}

////////////////////////////////////////////////////////////// IssueTracker //////////////////////////////////////////////////////////////

type IssueTracker struct {
	ComponentIds []int64  `json:"component_ids"`
	UserNames    []string `json:"usernames"`
}

func InitIssueTracker() (BugFramework, error) {
	return &IssueTracker{}, nil
}

func (it *IssueTracker) GetBugFrameworkName() string {
	return "IssueTracker"
}

func (it *IssueTracker) Search(username string, statuses []string) ([]Issue, error) {
	return nil, nil
}

func (it *IssueTracker) AddComment(i Issue, comment string) error {
	return nil
}

func (it *IssueTracker) SetState(i Issue, status string) error {
	return nil
}

////////////////////////////////////////////////////////////// MONORAIL //////////////////////////////////////////////////////////////

type Monorail struct {
	Projects  []string `json:"projects"`
	UserNames []string `json:"usernames"`
}

////////////////////////////////////////////////////////////// GITHUB //////////////////////////////////////////////////////////////

type Github struct {
	Projects  []string `json:"projects"`
	UserNames []string `json:"usernames"`
}
