package bugs

// A generic interface used by the different issue frameworks.

import (
	"context"
	"time"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
)

type Issue struct {
	Id       string                     `json:"id"`
	State    string                     `json:"state"`
	Priority types.StandardizedPriority `json:"priority"`
	Owner    string                     `json:"owner"`
	Link     string                     `json:"link"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`

	Title   string `json:"title"`   // This is not populated in IssueTracker.
	Summary string `json:"summary"` // This is not returned in IssueTracker or Monorail.
}

const (
	// All recognized clients.
	AndroidClient       types.RecognizedClient = "Android"
	ChromiumClient      types.RecognizedClient = "Chromium"
	FlutterNativeClient types.RecognizedClient = "Flutter-native"
	FlutterOnWebClient  types.RecognizedClient = "Flutter-on-web"
	SkiaClient          types.RecognizedClient = "Skia"

	// Supported issue sources.
	GithubSource       types.IssueSource = "Github"
	IssueTrackerSource types.IssueSource = "Buganizer"
	MonorailSource     types.IssueSource = "Monorail"

	// All bug frameworks will be standardized to these priorities.
	PriorityP0 types.StandardizedPriority = "P0"
	PriorityP1 types.StandardizedPriority = "P1"
	PriorityP2 types.StandardizedPriority = "P2"
	PriorityP3 types.StandardizedPriority = "P3"
	PriorityP4 types.StandardizedPriority = "P4"
	PriorityP5 types.StandardizedPriority = "P5"
	PriorityP6 types.StandardizedPriority = "P6"
)

type BugFramework interface {

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, config interface{}) ([]*Issue, error)

	// PutInDB puts the count of issues using the provided config into the DB.
	PutInDB(ctx context.Context, config interface{}, count int, dbClient *db.FirestoreDB) error
}
