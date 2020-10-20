package types

import "time"

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

type IssueSource string
type RecognizedClient string
type StandardizedPriority string

// All issues from the different issue frameworks will be standardized to this struct.
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
}

type IssueCountsData struct {
	OpenCount       int `json:"open_count"`
	UnassignedCount int `json:"unassigned_count"`

	// Priority counts.
	P0Count int `json:"p0_count"`
	P1Count int `json:"p1_count"`
	P2Count int `json:"p2_count"`
	P3Count int `json:"p3_count"`
	P4Count int `json:"p4_count"`
	P5Count int `json:"p5_count"`
	P6Count int `json:"p6_count"`

	// SLO violations per priority.
	// We only for SLOs for P0-P3. Listed here: https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit
	P0SLOViolationCount int `json:"p0_slo_count"`
	P1SLOViolationCount int `json:"p1_slo_count"`
	P2SLOViolationCount int `json:"p2_slo_count"`
	P3SLOViolationCount int `json:"p3_slo_count"`

	QueryLink string `json:"query_link"`
}

// CalculateSLOViolations uses data from https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit
func (qcd *IssueCountsData) CalculateSLOViolations(now, created, modified time.Time, priority StandardizedPriority) {
	switch priority {
	case PriorityP0:
		if now.After(modified.Add(Daily)) || now.After(created.Add(Weekly)) {
			qcd.P0SLOViolationCount++
		}
	case PriorityP1:
		if now.After(modified.Add(Weekly)) || now.After(created.Add(Monthly)) {
			qcd.P1SLOViolationCount++
		}
	case PriorityP2:
		if now.After(modified.Add(Biannualy)) || now.After(created.Add(Yearly)) {
			qcd.P2SLOViolationCount++
		}
	case PriorityP3:
		if now.After(modified.Add(Yearly)) || now.After(created.Add(Biennialy)) {
			qcd.P3SLOViolationCount++
		}
	}
}

func (qcd *IssueCountsData) Merge(from ...*IssueCountsData) {
	for _, f := range from {
		qcd.OpenCount += f.OpenCount
		qcd.UnassignedCount += f.UnassignedCount

		qcd.P0Count += f.P0Count
		qcd.P1Count += f.P1Count
		qcd.P2Count += f.P2Count
		qcd.P3Count += f.P3Count
		qcd.P4Count += f.P4Count
		qcd.P5Count += f.P5Count
		qcd.P6Count += f.P6Count

		qcd.P0SLOViolationCount += f.P0SLOViolationCount
		qcd.P1SLOViolationCount += f.P1SLOViolationCount
		qcd.P2SLOViolationCount += f.P2SLOViolationCount
		qcd.P3SLOViolationCount += f.P3SLOViolationCount
	}
}

func (qcd *IssueCountsData) IncPriority(priority StandardizedPriority) {
	switch priority {
	case PriorityP0:
		qcd.P0Count++
	case PriorityP1:
		qcd.P1Count++
	case PriorityP2:
		qcd.P2Count++
	case PriorityP3:
		qcd.P3Count++
	case PriorityP4:
		qcd.P4Count++
	case PriorityP5:
		qcd.P5Count++
	case PriorityP6:
		qcd.P6Count++
	}
}
