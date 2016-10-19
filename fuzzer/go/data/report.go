package data

import "sort"

type FuzzReport struct {
	// These are set by fuzzpool on ingestion.
	FileName     string `json:"fileName"`
	FunctionName string `json:"functionName"`
	LineNumber   int    `json:"lineNumber"`

	DebugStackTrace   StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace StackTrace `json:"releaseStackTrace"`
	DebugFlags        []string   `json:"debugFlags"`
	ReleaseFlags      []string   `json:"releaseFlags"`

	FuzzName         string `json:"fuzzName"`
	FuzzCategory     string `json:"category"`
	FuzzArchitecture string `json:"architecture"`
	IsGrey           bool   `json:"isGrey"`
}

// ParseReport creates a report given the raw materials passed in.
func ParseReport(g GCSPackage) FuzzReport {
	result := ParseGCSPackage(g)
	return FuzzReport{
		DebugStackTrace:   result.Debug.StackTrace,
		ReleaseStackTrace: result.Release.StackTrace,
		DebugFlags:        result.Debug.Flags.ToHumanReadableFlags(),
		ReleaseFlags:      result.Release.Flags.ToHumanReadableFlags(),
		FuzzName:          g.Name,
		FuzzCategory:      g.FuzzCategory,
		FuzzArchitecture:  g.FuzzArchitecture,
		IsGrey:            result.IsGrey(),
	}
}

// SortedFuzzReports keeps the fuzzes sorted by FuzzName, i.e. the hash of the contents.
type SortedFuzzReports []FuzzReport

func (p SortedFuzzReports) Len() int           { return len(p) }
func (p SortedFuzzReports) Less(i, j int) bool { return p[i].FuzzName < p[j].FuzzName }
func (p SortedFuzzReports) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Append adds b to the already sorted caller, and returns the sorted result.
// Precondition: Caller must be nil or sorted
func (p SortedFuzzReports) Append(b FuzzReport) SortedFuzzReports {
	s := append(p, b)

	// Google Storage gives us the fuzzes in alphabetical order.  Thus, we can short circuit
	// if the fuzz goes on the end (which is usually does).
	// However, we can't always do this because when we load a second batch of fuzzes,
	// those are in alphabetical order, but starting over from 0.
	// We want to avoid [a,c,x,z,b,d] where b,d were added from the second batch.
	if len(s) <= 1 || s.Less(len(s)-2, len(s)-1) {
		return s
	}
	sort.Sort(s)
	return s
}

// containsName returns the FuzzReport and true if a fuzz with the given name is in the list.
func (p SortedFuzzReports) containsName(fuzzName string) (FuzzReport, bool) {
	i := sort.Search(len(p), func(i int) bool { return p[i].FuzzName >= fuzzName })
	if i < len(p) && p[i].FuzzName == fuzzName {
		return p[i], true
	}
	return FuzzReport{}, false
}
