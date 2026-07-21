package types

import (
	"fmt"
	"strings"

	"go.skia.org/infra/task_scheduler/go/types"
)

type TaskSummary struct {
	Analysis     string `json:"analysis"`
	ErrorMessage string `json:"errorMessage"`
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
