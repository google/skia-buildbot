// Package frontend houses a variety of types that represent how the frontend
// expects the format of data. The data types here are those shared by
// multiple packages.
package frontend

import (
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/code_review"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/types"
)

const urlPlaceholder = "%s"

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
func ConvertChangeList(cl code_review.ChangeList, system, urlTempl string) ChangeList {
	return ChangeList{
		System:   system,
		SystemID: cl.SystemID,
		Owner:    cl.Owner,
		Status:   cl.Status.String(),
		Subject:  cl.Subject,
		Updated:  cl.Updated,
		URL:      strings.Replace(urlTempl, urlPlaceholder, cl.SystemID, 1),
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
func ConvertTryJob(tj ci.TryJob, system, urlTempl string) TryJob {
	return TryJob{
		System:      system,
		SystemID:    tj.SystemID,
		DisplayName: tj.DisplayName,
		Updated:     tj.Updated,
		URL:         strings.Replace(urlTempl, urlPlaceholder, tj.SystemID, 1),
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

// DigestListResponse is the response for "what digests belong to..."
type DigestListResponse struct {
	Digests []types.Digest `json:"digests"`
}

// IgnoreRule represents an ignore.Rule as well as how many times the rule
// was applied. This allows for the decoupling of the rule as stored in the
// DB from how we present it to the UI.
type IgnoreRule struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	UpdatedBy   string              `json:"updatedBy"`
	Expires     time.Time           `json:"expires"`
	Query       string              `json:"query"`
	ParsedQuery map[string][]string `json:"-"`
	Note        string              `json:"note"`
	// Count represents how many traces are affected by this ignore rule.
	Count int `json:"countAll"`
	// ExclusiveCount represents how many traces are affected *exclusively* by this ignore rule,
	// that is, they are only matched by this rule.
	ExclusiveCount int `json:"exclusiveCountAll"`
	// UntriagedCount represents how many traces with an untriaged digest at HEAD are affected
	// by this ignore rule.
	UntriagedCount int `json:"count"`
	// ExclusiveUntriagedCount represents how many traces with an untriaged digest at HEAD are affected
	// *exclusively* by this ignore rule, that is, they are only matched by this rule.
	ExclusiveUntriagedCount int `json:"exclusiveCount"`
}

// ConvertIgnoreRule converts a backend ignore.Rule into its frontend
// counterpart.
func ConvertIgnoreRule(r *ignore.Rule) (*IgnoreRule, error) {
	pq, err := url.ParseQuery(r.Query)
	if err != nil {
		return nil, skerr.Wrapf(err, "invalid rule id %q; query %q", r.ID, r.Query)
	}
	return &IgnoreRule{
		ID:          r.ID,
		Name:        r.Name,
		UpdatedBy:   r.UpdatedBy,
		Expires:     r.Expires,
		Query:       r.Query,
		ParsedQuery: pq,
		Note:        r.Note,
	}, nil
}

// IgnoreRuleBody encapsulates a single ignore rule that is submitted for addition or update.
type IgnoreRuleBody struct {
	// Duration is a human readable string like "2w", "4h" to specify a duration.
	Duration string `json:"duration"`
	// Filter is a url-encoded set of key-value pairs that can be used to match traces.
	// For example: "config=angle_d3d9_es2&cpu_or_gpu_value=RadeonHD7770"
	Filter string `json:"filter"`
	// Note is a comment by a developer, typically a bug.
	Note string `json:"note"`
}
