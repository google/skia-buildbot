package bugs

// A generic interface used by the different issue frameworks.

import (
	"context"
	"time"
)

type Issue struct {
	Id       string               `json:"id"`
	State    string               `json:"state"`
	Priority StandardizedPriority `json:"priority"`
	Owner    string               `json:"owner"`
	Link     string               `json:"link"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`

	Title   string `json:"title"`   // This is not populated in IssueTracker.
	Summary string `json:"summary"` // This is not returned in IssueTracker or Monorail.

	// Source from where this issue was found (eg: Monorail, IssueTracker, Github)
	Source IssueSource `json:"source"`
	// Client this issue should be attributed to (eg: Flutter-native, Flutter-on-web, Android, etc)
	Client RecognizedClient `json:"project"`
}

type IssueSource string
type RecognizedClient string
type StandardizedPriority string

const (
	// All recognized clients.
	AndroidClient       RecognizedClient = "Android"
	ChromiumClient      RecognizedClient = "Chromium"
	FlutterNativeClient RecognizedClient = "Flutter-native"
	FlutterOnWebClient  RecognizedClient = "Flutter-on-web"
	SkiaClient          RecognizedClient = "Skia"

	// Supported issue sources.
	GithubSource       IssueSource = "Github"
	IssueTrackerSource IssueSource = "Buganizer"
	MonorailSource     IssueSource = "Monorail"

	// All bug frameworks will be standardized to these priorities.
	PriorityP0 StandardizedPriority = "P0"
	PriorityP1 StandardizedPriority = "P1"
	PriorityP2 StandardizedPriority = "P2"
	PriorityP3 StandardizedPriority = "P3"
	PriorityP4 StandardizedPriority = "P4"
	PriorityP5 StandardizedPriority = "P5"
	PriorityP6 StandardizedPriority = "P6"
)

type BugFramework interface {

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, config interface{}) ([]*Issue, error)

	// Returns the bug framework specific link to the issue.
	GetLink(project, id string) string
}
