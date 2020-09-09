package bugs

import (
	"time"
)

const ()

type Bug struct {
	Id       string `json:"id"`
	Status   string `json:"status"`
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

	// Search returns bugs that match the provided parameters.
	Search(username string, statuses []string) ([]Bug, error)

	// AddComment adds a comment to the issue with the given id.
	AddComment(b Bug, comment string) error

	SetStatus(b Bug, bugstatus string) error
}

type BuganizerConfig struct {
	ComponentIds []int64 `json:"component_ids"`
	UserName     string  `json:"username"`
}

type MonorailConfig struct {
	Projects []string `json:"projects"`
}

type GithubConfig struct {
	Projects []string `json:"projects"`
}
