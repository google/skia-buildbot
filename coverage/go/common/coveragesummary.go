package common

// CoverageSummary represents the parsed coverage data for a coverage job.
type CoverageSummary struct {
	Name        string `json:"name"`
	TotalLines  int    `json:"total_lines"`
	MissedLines int    `json:"missed_lines"`
}

type CoverageSummarySlice []CoverageSummary

// The following 3 lines implement sort.Interface
func (s CoverageSummarySlice) Len() int           { return len(s) }
func (s CoverageSummarySlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s CoverageSummarySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
