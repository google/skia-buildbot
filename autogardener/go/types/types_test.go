package types

import (
	"encoding/json"
	"testing"
	"time"

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

func TestFailureClass_Getters(t *testing.T) {
	fc := &FailureClass{
		Id: "fc-1",
		Updates: []*FailureClassUpdate{
			{
				User:         "autogardener",
				Title:        Ptr("initial title"),
				ErrorMessage: Ptr("initial error"),
				Analysis:     Ptr("initial analysis"),
				Flaky:        Ptr(true),
				Culprits:     Ptr([]string{"commit-a", "commit-b"}),
			},
			{
				User:        "dev@google.com",
				Title:       Ptr("updated title"),
				Analysis:    Ptr("updated analysis"),
				Flaky:       Ptr(false),
				Culprits:    Ptr([]string{"commit-b"}),
				Bugs:        Ptr([]string{"b/12345"}),
				ResolvedBy:  Ptr([]string{"commit-c"}),
				DuplicateOf: Ptr("fc-0"),
				Duplicates:  Ptr([]string{"fc-2", "fc-3"}),
			},
			{
				User:         "dev2@google.com",
				Title:        Ptr(""),
				ErrorMessage: Ptr("trimmed error"),
				ResolvedBy:   Ptr([]string{}),
				DuplicateOf:  Ptr("fc-canonical"),
			},
		},
	}

	// Verify JSON round-trip preserves empty strings and empty slices vs nil.
	b, err := json.Marshal(fc)
	require.NoError(t, err)
	var roundTripped FailureClass
	require.NoError(t, json.Unmarshal(b, &roundTripped))

	require.Equal(t, "", roundTripped.Title())
	require.Equal(t, "trimmed error", roundTripped.ErrorMessage())
	require.Equal(t, "updated analysis", roundTripped.Analysis())
	require.False(t, roundTripped.Flaky())
	require.Equal(t, []string{"commit-b"}, roundTripped.Culprits())
	require.Equal(t, []string{}, roundTripped.ResolvedBy())
	require.Equal(t, []string{"b/12345"}, roundTripped.Bugs())
	require.Equal(t, "fc-canonical", roundTripped.DuplicateOf())
	require.Equal(t, []string{"fc-2", "fc-3"}, roundTripped.Duplicates())
}

func TestFailureClassUpdate_JSON(t *testing.T) {
	ts := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	t.Run("nil fields are omitted", func(t *testing.T) {
		u := &FailureClassUpdate{
			Timestamp: ts,
			User:      "autogardener",
		}
		b, err := json.Marshal(u)
		require.NoError(t, err)
		require.JSONEq(t, `{"timestamp":"2026-09-30T12:00:00Z","user":"autogardener"}`, string(b))

		var got FailureClassUpdate
		require.NoError(t, json.Unmarshal(b, &got))
		require.Equal(t, u, &got)
	})

	t.Run("empty values are serialized and preserved", func(t *testing.T) {
		u := &FailureClassUpdate{
			Timestamp:    ts,
			User:         "dev@google.com",
			Title:        Ptr(""),
			ErrorMessage: Ptr(""),
			Analysis:     Ptr(""),
			Flaky:        Ptr(false),
			Culprits:     Ptr([]string{}),
			ResolvedBy:   Ptr([]string{}),
			Bugs:         Ptr([]string{}),
			DuplicateOf:  Ptr(""),
			Duplicates:   Ptr([]string{}),
		}
		b, err := json.Marshal(u)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"timestamp": "2026-09-30T12:00:00Z",
			"user": "dev@google.com",
			"title": "",
			"errorMessage": "",
			"analysis": "",
			"flaky": false,
			"culprits": [],
			"resolvedBy": [],
			"bugs": [],
			"duplicateOf": "",
			"duplicates": []
		}`, string(b))

		var got FailureClassUpdate
		require.NoError(t, json.Unmarshal(b, &got))
		require.Equal(t, u, &got)
	})

	t.Run("non-empty values are serialized and preserved", func(t *testing.T) {
		u := &FailureClassUpdate{
			Timestamp:    ts,
			User:         "dev@google.com",
			Message:      "Investigated failure",
			Title:        Ptr("Shader compile failure"),
			ErrorMessage: Ptr("error: undeclared identifier"),
			Analysis:     Ptr("Introduced by shader refactor"),
			Flaky:        Ptr(true),
			Culprits:     Ptr([]string{"abc1234"}),
			ResolvedBy:   Ptr([]string{"def5678"}),
			Bugs:         Ptr([]string{"b/12345"}),
			DuplicateOf:  Ptr("fc-parent"),
			Duplicates:   Ptr([]string{"fc-child-1", "fc-child-2"}),
		}
		b, err := json.Marshal(u)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"timestamp": "2026-09-30T12:00:00Z",
			"user": "dev@google.com",
			"message": "Investigated failure",
			"title": "Shader compile failure",
			"errorMessage": "error: undeclared identifier",
			"analysis": "Introduced by shader refactor",
			"flaky": true,
			"culprits": ["abc1234"],
			"resolvedBy": ["def5678"],
			"bugs": ["b/12345"],
			"duplicateOf": "fc-parent",
			"duplicates": ["fc-child-1", "fc-child-2"]
		}`, string(b))

		var got FailureClassUpdate
		require.NoError(t, json.Unmarshal(b, &got))
		require.Equal(t, u, &got)
	})
}
