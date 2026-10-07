package types

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"go.skia.org/infra/task_scheduler/go/types"
)

type TaskSummary struct {
	Analysis       string `json:"analysis"`
	ErrorMessage   string `json:"errorMessage"`
	FailureClassId string `json:"failureClassId"`
}

func (s TaskSummary) String() string {
	return fmt.Sprintf("**Error Message:**\n```\n%s\n```\n\n**Analysis:** %s\n", s.ErrorMessage, s.Analysis)
}

type TaskAndSummary struct {
	Task    *types.Task
	Summary *TaskSummary
}

type Classification string

const (
	ClassificationUnknown      Classification = ""
	ClassificationPersistent   Classification = "Persistent"
	ClassificationFlaky        Classification = "Flaky"
	ClassificationResolved     Classification = "Resolved"
	ClassificationInfraFailure Classification = "InfraFailure"
	ClassificationMisc         Classification = "Misc"
)

type Report struct {
	Summary string
	Items   []*ReportItem
}

func (r *Report) String() string {
	var sb strings.Builder

	_, _ = fmt.Fprintf(&sb, "# Skia Gardening Report\n\n%s\n\n", r.Summary)

	writeSection := func(cls Classification, title string) {
		found := false
		for _, item := range r.Items {
			if item.Classification == cls {
				found = true
				break
			}
		}

		if found {
			_, _ = fmt.Fprintf(&sb, "## %s\n\n", title)
			idx := 1
			for _, item := range r.Items {
				if item.Classification == cls {
					_, _ = fmt.Fprintf(&sb, "%d. %s\n", idx, item.String())
					idx++
				}
			}
		}
	}
	writeSection(ClassificationPersistent, "Persistent Failures")
	writeSection(ClassificationFlaky, "Flaky Failures")
	writeSection(ClassificationInfraFailure, "Infra Failures")
	writeSection(ClassificationResolved, "Resolved Failures")
	writeSection(ClassificationMisc, "Miscellaneous")

	return sb.String()
}

type ReportItem struct {
	Classification Classification `jsonschema:"enum=Persistent,enum=Flaky,enum=Resolved,enum=InfraFailure,enum=Misc"`
	Culprits       []string
	AffectedTasks  []string
	Summary        string
}

func (i *ReportItem) String() string {
	var culpritsSB strings.Builder
	for idx, culprit := range i.Culprits {
		comma := ","
		if idx == len(i.Culprits)-1 {
			comma = ""
		}
		_, _ = fmt.Fprintf(&culpritsSB, "%s%s", culprit[:7], comma)
	}

	var tasksSB strings.Builder
	for _, task := range i.AffectedTasks {
		_, _ = fmt.Fprintf(&tasksSB, "  * %s\n", task)
	}

	return fmt.Sprintf(`%s
- **Classification:** %s
- **Potential Culprits(s):** %s
- **Tasks Affected:**
%s`, i.Summary, i.Classification, culpritsSB.String(), tasksSB.String())
}

type SummaryForTasks struct {
	TaskSummary
	TaskNames []string
	TaskIDs   []string
}

func (r SummaryForTasks) String() string {
	return fmt.Sprintf(`**Task Names:** %s
**Task IDs:** %s
**Error:** %s
**Analysis:** %s
`, strings.Join(r.TaskNames, ", "), strings.Join(r.TaskIDs, ", "), r.ErrorMessage, r.Analysis)
}

// FailureClass represents a set of task failures with the same root cause.
type FailureClass struct {
	Id       string                `json:"id"`
	LastSeen time.Time             `json:"lastSeen"`
	Repo     string                `json:"repo"`
	Updates  []*FailureClassUpdate `json:"updates"`

	// Legacy fields used only to recover top-level ErrorMessage and Analysis
	// from old database entries that have no Updates.
	LegacyErrorMessage *string `firestore:"ErrorMessage,omitempty" json:"-"`
	LegacyAnalysis     *string `firestore:"Analysis,omitempty" json:"-"`
}

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

// Title returns the latest non-nil Title across Updates.
func (fc *FailureClass) Title() string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.Title != nil {
			return *u.Title
		}
	}
	return ""
}

// ErrorMessage returns the latest non-nil ErrorMessage across Updates.
func (fc *FailureClass) ErrorMessage() string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.ErrorMessage != nil {
			return *u.ErrorMessage
		}
	}
	return ""
}

// Analysis returns the latest non-nil Analysis across Updates.
func (fc *FailureClass) Analysis() string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.Analysis != nil {
			return *u.Analysis
		}
	}
	return ""
}

// Flaky returns the latest non-nil Flaky value across Updates.
func (fc *FailureClass) Flaky() bool {
	for _, u := range slices.Backward(fc.Updates) {
		if u.Flaky != nil {
			return *u.Flaky
		}
	}
	return false
}

// Culprits returns the latest non-nil Culprits slice across Updates.
func (fc *FailureClass) Culprits() []string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.Culprits != nil {
			return *u.Culprits
		}
	}
	return nil
}

// ResolvedBy returns the latest non-nil ResolvedBy slice across Updates.
func (fc *FailureClass) ResolvedBy() []string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.ResolvedBy != nil {
			return *u.ResolvedBy
		}
	}
	return nil
}

// Bugs returns the latest non-nil Bugs slice across Updates.
func (fc *FailureClass) Bugs() []string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.Bugs != nil {
			return *u.Bugs
		}
	}
	return nil
}

// DuplicateOf returns the latest non-nil DuplicateOf value across Updates.
func (fc *FailureClass) DuplicateOf() string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.DuplicateOf != nil {
			return *u.DuplicateOf
		}
	}
	return ""
}

// Duplicates returns the latest non-nil Duplicates slice across Updates.
func (fc *FailureClass) Duplicates() []string {
	for _, u := range slices.Backward(fc.Updates) {
		if u.Duplicates != nil {
			return *u.Duplicates
		}
	}
	return nil
}

// FailureClassUpdate represents a single timestamped comment/edit on a
// FailureClass, authored by either autogardener or a user.
type FailureClassUpdate struct {
	Timestamp    time.Time `json:"timestamp"`
	User         string    `json:"user"`
	Message      string    `json:"message,omitempty"`
	Title        *string   `json:"title,omitempty"`
	ErrorMessage *string   `json:"errorMessage,omitempty"`
	Analysis     *string   `json:"analysis,omitempty"`
	Flaky        *bool     `json:"flaky,omitempty"`
	Culprits     *[]string `json:"culprits,omitempty"`
	ResolvedBy   *[]string `json:"resolvedBy,omitempty"`
	Bugs         *[]string `json:"bugs,omitempty"`
	DuplicateOf  *string   `json:"duplicateOf,omitempty"`
	Duplicates   *[]string `json:"duplicates,omitempty"`
}
