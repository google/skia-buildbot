package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportItem_String(t *testing.T) {
	item := &ReportItem{
		Classification: ClassificationPersistent,
		Summary:        "Test Summary",
		Culprits: []string{
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		AffectedTasks: []string{"Task 1", "Task 2"},
	}
	expected := `Test Summary
- **Classification:** Persistent
- **Potential Culprits(s):** aaaaaaa,bbbbbbb
- **Tasks Affected:**
  * Task 1
  * Task 2
`
	require.Equal(t, expected, item.String())
}

func TestReport_String(t *testing.T) {
	report := &Report{
		Summary: "General Summary",
		Items: []*ReportItem{
			{
				Classification: ClassificationPersistent,
				Summary:        "Problem 1",
				Culprits: []string{
					"12345678901234567890",
					"abcdefabcdefabcdef",
				},
				AffectedTasks: []string{"Task A"},
			},
			{
				Classification: ClassificationPersistent,
				Summary:        "Problem 2",
				Culprits: []string{
					"abcdefabcdefabcdef",
				},
				AffectedTasks: []string{"Task A", "Task D"},
			},
			{
				Classification: ClassificationFlaky,
				Summary:        "Flaky 1",
				Culprits: []string{
					"abcdefabcdefabcdef",
				},
				AffectedTasks: []string{"Task B"},
			},
			{
				Classification: ClassificationMisc,
				Summary:        "Misc 1",
				Culprits: []string{
					"fedcbafedcbafedcba",
				},
				AffectedTasks: []string{"Task C"},
			},
		},
	}
	expected := `# Skia Gardening Report

General Summary

## Persistent Failures

1. Problem 1
- **Classification:** Persistent
- **Potential Culprits(s):** 1234567,abcdefa
- **Tasks Affected:**
  * Task A

2. Problem 2
- **Classification:** Persistent
- **Potential Culprits(s):** abcdefa
- **Tasks Affected:**
  * Task A
  * Task D

## Flaky Failures

1. Flaky 1
- **Classification:** Flaky
- **Potential Culprits(s):** abcdefa
- **Tasks Affected:**
  * Task B

## Miscellaneous

1. Misc 1
- **Classification:** Misc
- **Potential Culprits(s):** fedcbaf
- **Tasks Affected:**
  * Task C

`
	require.Equal(t, expected, report.String())
}
