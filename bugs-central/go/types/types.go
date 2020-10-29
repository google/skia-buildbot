package types

import (
	"fmt"
	"time"

	"github.com/hako/durafmt"
)

const (
	// All bug frameworks will be standardized to these priorities.
	PriorityP0 StandardizedPriority = "P0"
	PriorityP1 StandardizedPriority = "P1"
	PriorityP2 StandardizedPriority = "P2"
	PriorityP3 StandardizedPriority = "P3"
	PriorityP4 StandardizedPriority = "P4"
	PriorityP5 StandardizedPriority = "P5"
	PriorityP6 StandardizedPriority = "P6"

	// Convenient constants to use when calculating SLO violations.
	Daily     = 24 * time.Hour
	Weekly    = 7 * Daily
	Monthly   = 30 * Daily
	Biannualy = 6 * Monthly
	Yearly    = 2 * Biannualy
	Biennialy = 2 * Yearly
)

type sloViolationData struct {
	// If an open issue's last modified time is beyond this duration then it is an SLO violation.
	modifiedDuration time.Duration
	// If an open issue's creation time is beyond this duration then it is an SLO violation.
	createdDuration time.Duration
}

var (
	// Uses data from https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit
	priorityToSLOViolationData = map[StandardizedPriority]sloViolationData{
		PriorityP0: {
			modifiedDuration: Daily,
			createdDuration:  Weekly,
		},
		PriorityP1: {
			modifiedDuration: Weekly,
			createdDuration:  Monthly,
		},
		PriorityP2: {
			modifiedDuration: Biannualy,
			createdDuration:  Yearly,
		},
		PriorityP3: {
			modifiedDuration: Yearly,
			createdDuration:  Biennialy,
		},
	}
)

// IssueSource types will be all the recognized issue frameworks (eg: Github, IssueTracker, Monorail).
type IssueSource string

// RecognizedClient types will be all the recognized Skia clients (eg: Android, Chromium, Flutter).
type RecognizedClient string

// StandardizedPriority types will be the priorities used across issue frameworks.
type StandardizedPriority string

// All issues from the different issue frameworks will be standardized to this struct.
type Issue struct {
	Id       string               `json:"id"`
	State    string               `json:"state"`
	Priority StandardizedPriority `json:"priority"`
	Owner    string               `json:"owner"`
	Link     string               `json:"link"`

	SLOViolation         bool          `json:"slo_violation"`
	SLOViolationReason   string        `json:"slo_violation_reason"`
	SLOViolationDuration time.Duration `json:"slo_violation_duration"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`

	Title   string `json:"title"`   // This is not populated in IssueTracker.
	Summary string `json:"summary"` // This is not returned in IssueTracker or Monorail.
}

// IssueCountsData will be used to keep track of the counts of different issue types that
// are returned by the different bug frameworks. It will also be used to keep track of
// SLO violations and the queries that were used.
type IssueCountsData struct {
	OpenCount       int `json:"open_count"`
	UnassignedCount int `json:"unassigned_count"`
	UntriagedCount  int `json:"untriaged_count"`

	// Priority counts.
	P0Count int `json:"p0_count"`
	P1Count int `json:"p1_count"`
	P2Count int `json:"p2_count"`
	P3Count int `json:"p3_count"`
	P4Count int `json:"p4_count"`
	P5Count int `json:"p5_count"`
	P6Count int `json:"p6_count"`

	// SLO violations per priority.
	// We only do SLOs for P0-P3. Listed here: https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit
	P0SLOViolationCount int `json:"p0_slo_count"`
	P1SLOViolationCount int `json:"p1_slo_count"`
	P2SLOViolationCount int `json:"p2_slo_count"`
	P3SLOViolationCount int `json:"p3_slo_count"`

	// Links to the issue framework.
	QueryLink          string `json:"query_link"`
	UntriagedQueryLink string `json:"untriaged_query_link"`
	P0Link             string `json:"p0_link"`
	P1Link             string `json:"p1_link"`
	P2Link             string `json:"p2_link"`
	P3AndRestLink      string `json:"p3_and_rest_link"`
}

// IncSLOViolations will increment the priority's corresponding slo count.
func (icd *IssueCountsData) IncSLOViolation(violation bool, priority StandardizedPriority) {
	if !violation {
		// Nothing to do here.
		return
	}
	switch priority {
	case PriorityP0:
		if violation {
			icd.P0SLOViolationCount++
		}
	case PriorityP1:
		if violation {
			icd.P1SLOViolationCount++
		}
	case PriorityP2:
		if violation {
			icd.P2SLOViolationCount++
		}
	case PriorityP3:
		if violation {
			icd.P3SLOViolationCount++
		}
	}
}

// Merge is used to combine an instance of IssueCountsData into this one.
func (icd *IssueCountsData) Merge(from ...*IssueCountsData) {
	for _, f := range from {
		icd.OpenCount += f.OpenCount
		icd.UnassignedCount += f.UnassignedCount
		icd.UntriagedCount += f.UntriagedCount

		icd.P0Count += f.P0Count
		icd.P1Count += f.P1Count
		icd.P2Count += f.P2Count
		icd.P3Count += f.P3Count
		icd.P4Count += f.P4Count
		icd.P5Count += f.P5Count
		icd.P6Count += f.P6Count

		icd.P0SLOViolationCount += f.P0SLOViolationCount
		icd.P1SLOViolationCount += f.P1SLOViolationCount
		icd.P2SLOViolationCount += f.P2SLOViolationCount
		icd.P3SLOViolationCount += f.P3SLOViolationCount
	}
}

// IncPriority will increment the corresponding priority count of the
// specified StandardizedPriority.
func (icd *IssueCountsData) IncPriority(priority StandardizedPriority) {
	switch priority {
	case PriorityP0:
		icd.P0Count++
	case PriorityP1:
		icd.P1Count++
	case PriorityP2:
		icd.P2Count++
	case PriorityP3:
		icd.P3Count++
	case PriorityP4:
		icd.P4Count++
	case PriorityP5:
		icd.P5Count++
	case PriorityP6:
		icd.P6Count++
	}
}

// IsPrioritySLOViolation returns whether the priority is outside the SLO.
// If issue has violated SLO then returns description and a duration that shows by how much
// it was surpassed.
func IsPrioritySLOViolation(now, created, modified time.Time, priority StandardizedPriority) (bool, string, time.Duration) {
	if sloViolationData, ok := priorityToSLOViolationData[priority]; ok {
		if now.After(modified.Add(sloViolationData.modifiedDuration)) {
			duration := now.Sub(modified.Add(sloViolationData.modifiedDuration))
			return true, fmt.Sprintf("exceeded modified time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
		} else if now.After(created.Add(sloViolationData.createdDuration)) {
			duration := now.Sub(created.Add(sloViolationData.createdDuration))
			return true, fmt.Sprintf("exceeded creation time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
		}
	}
	return false, "", 0
}
