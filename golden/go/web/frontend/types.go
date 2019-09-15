// package frontend houses a variety of types that represent how the frontend
// expects the format of data. The data types here are those shared by
// multiple packages.
package frontend

import (
	"time"

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
