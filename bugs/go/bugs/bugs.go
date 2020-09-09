package bugs

// CALL THIS bug_framework instead.

import (
	"time"
)

const ()

type Issue struct {
	Id       string `json:"id"`
	Title    string `json:"title"`
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

	// GetBugFrameworkName returns the name of the bug framework. Eg: Monorail, Buganizer, Github.
	GetBugFrameworkName() string

	// Search returns issues that match the provided parameters.
	Search(username string, statuses []string) ([]Issue, error)

	// AddComment adds a comment to the specified issue.
	AddComment(i Issue, comment string) error

	// SetStatus adds a comment to the specified issue.
	SetStatus(i Issue, status string) error
}

////////////////////////////////////////////////////////////// BUGANIZER //////////////////////////////////////////////////////////////

type Buganizer struct {
	ComponentIds []int64  `json:"component_ids"`
	UserNames    []string `json:"usernames"`
}

func InitBuganizer() (BugFramework, error) {
	return &Buganizer{}, nil
}

func (b *Buganizer) GetBugFrameworkName() string {
	return "Buganizer"
}

func (b *Buganizer) Search(username string, statuses []string) ([]Issue, error) {
	return nil, nil
}

func (b *Buganizer) AddComment(i Issue, comment string) error {
	return nil
}

func (b *Buganizer) SetStatus(i Issue, status string) error {
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
