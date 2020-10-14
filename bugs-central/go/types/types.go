package types

const (
	// All bug frameworks will be standardized to these priorities.
	PriorityP0 StandardizedPriority = "P0"
	PriorityP1 StandardizedPriority = "P1"
	PriorityP2 StandardizedPriority = "P2"
	PriorityP3 StandardizedPriority = "P3"
	PriorityP4 StandardizedPriority = "P4"
	PriorityP5 StandardizedPriority = "P5"
	PriorityP6 StandardizedPriority = "P6"
)

type IssueSource string
type RecognizedClient string
type StandardizedPriority string

type IssueCountsData struct {
	OpenCount       int `json:"open_count"`
	UnassignedCount int `json:"unassigned_count"`
	P0Count         int `json:"p0_count"`
	P1Count         int `json:"p1_count"`
	P2Count         int `json:"p2_count"`
	P3Count         int `json:"p3_count"`
}

func (qcd *IssueCountsData) MergeInto(mergeFrom *IssueCountsData) {
	qcd.OpenCount += mergeFrom.OpenCount
	qcd.UnassignedCount += mergeFrom.UnassignedCount
	qcd.P0Count += mergeFrom.P0Count
	qcd.P1Count += mergeFrom.P1Count
	qcd.P2Count += mergeFrom.P2Count
	qcd.P3Count += mergeFrom.P3Count
}

func (qcd *IssueCountsData) IsEqual(compareWith *IssueCountsData) bool {
	return qcd.OpenCount == compareWith.OpenCount &&
		qcd.UnassignedCount == compareWith.UnassignedCount &&
		qcd.P0Count == compareWith.P0Count &&
		qcd.P1Count == compareWith.P1Count &&
		qcd.P2Count == compareWith.P2Count &&
		qcd.P3Count == compareWith.P3Count
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
	}
}
