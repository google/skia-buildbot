// package frontend houses a variety of types that represent how the frontend
// expects the format of data. The data types here are those shared by
// multiple packages.
package frontend

import (
	"time"

	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"

	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
)

// ChangeList encapsulates how the frontend expects to get information
// about a code_review.ChangeList that has Gold results associated with it.
// We have a separate struct so we can decouple the JSON representation
// and the backend representation (if it needs changing or use by another project
// with its own JSON requirements).
type ChangeList struct {
	System   string    `json:"system"`
	SystemID string    `json:"id"`
	Owner    string    `json:"owner"`
	Status   string    `json:"status"`
	Subject  string    `json:"subject"`
	Updated  time.Time `json:"updated"`
	URL      string    `json:"url"`
}

// ConvertChangeList turns a code_review.ChangeList into a ChangeList for the frontend.
func ConvertChangeList(cl code_review.ChangeList, system, urlPrefix string) ChangeList {
	return ChangeList{
		System:   system,
		SystemID: cl.SystemID,
		Owner:    cl.Owner,
		Status:   cl.Status.String(),
		Subject:  cl.Subject,
		Updated:  cl.Updated,
		URL:      urlPrefix + "/" + cl.SystemID,
	}
}

// ChangeListSummary encapsulates how the frontend expects to get a summary of
// the TryJob information we have associated with a given ChangeList. These
// TryJobs are those we've noticed that uploaded results to Gold.
type ChangeListSummary struct {
	CL ChangeList `json:"cl"`
	// these are only those patchsets with data.
	PatchSets         []PatchSet `json:"patch_sets"`
	NumTotalPatchSets int        `json:"num_total_patch_sets"`
}

// PatchSet represents the data the frontend needs for PatchSets.
type PatchSet struct {
	SystemID string   `json:"id"`
	Order    int      `json:"order"`
	TryJobs  []TryJob `json:"try_jobs"`
}

// TryJob represents the data the frontend needs for TryJobs.
type TryJob struct {
	SystemID    string    `json:"id"`
	DisplayName string    `json:"name"`
	Updated     time.Time `json:"updated"`
	System      string    `json:"system"`
	URL         string    `json:"url"`
}

// ConvertTryJob turns a ci.TryJob into a TryJob for the frontend.
func ConvertTryJob(tj ci.TryJob, system, urlPrefix string) TryJob {
	return TryJob{
		System:      system,
		SystemID:    tj.SystemID,
		DisplayName: tj.DisplayName,
		Updated:     tj.Updated,
		URL:         urlPrefix + "/" + tj.SystemID,
	}
}

// TriageRequest is the form of the JSON posted by the frontend when triaging
// (both single and bulk).
type TriageRequest struct {
	// TestDigestStatus maps status to test name and digests. The strings are
	// types.Label.String() values
	TestDigestStatus map[types.TestName]map[types.Digest]string `json:"testDigestStatus"`

	// ChangeListID is the id of the ChangeList for which we want to change the expectations.
	// "issue" is the JSON field for backwards compatibility.
	ChangeListID string `json:"issue"`
}

// TriageDelta represents one changed digest and the label that was
// assigned as part of the triage operation.
type TriageDelta struct {
	TestName types.TestName `json:"test_name"`
	Digest   types.Digest   `json:"digest"`
	Label    string         `json:"label"`
}

// TriageLogEntry represents a set of changes by a single person.
type TriageLogEntry struct {
	ID          string        `json:"id"`
	User        string        `json:"name"`
	TS          int64         `json:"ts"` // is milliseconds since the epoch
	ChangeCount int           `json:"changeCount"`
	Details     []TriageDelta `json:"details"`
}

// ConvertLogEntry turns an expstorage.TriageLogEntry into its frontend representation.
func ConvertLogEntry(entry expstorage.TriageLogEntry) TriageLogEntry {
	tle := TriageLogEntry{
		ID:          entry.ID,
		User:        entry.User,
		TS:          entry.TS.Unix() * 1000,
		ChangeCount: entry.ChangeCount,
	}
	for _, d := range entry.Details {
		tle.Details = append(tle.Details, TriageDelta{
			TestName: d.Grouping,
			Digest:   d.Digest,
			Label:    d.Label.String(),
		})
	}
	return tle
}
