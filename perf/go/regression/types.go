package regression

import (
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
)

// DetailLookup is used by RegressionStore to look up commit details.
type DetailLookup func(c *cid.CommitID) (*cid.CommitDetail, error)

// RegressionStore persists Regressions.
type RegressionStore interface {
	// Untriaged returns the number of untriaged regressions.
	Untriaged() (int, error)

	// Write the Regressions to the store.
	Write(regressions map[string]*Regressions, lookup DetailLookup) error

	// Range returns a map from cid.ID()'s to *Regressions that exist in the given time range.
	Range(begin, end int64) (map[string]*Regressions, error)

	// SetHigh sets the ClusterSummary for a high regression at the given commit and alertID.
	SetHigh(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error)

	// SetLow sets the ClusterSummary for a low regression at the given commit and alertID.
	SetLow(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error)

	// TriageLow sets the triage status for the low cluster at the given commit and alertID.
	TriageLow(cid *cid.CommitDetail, alertID string, tr TriageStatus) error

	// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
	TriageHigh(cid *cid.CommitDetail, alertID string, tr TriageStatus) error
}
