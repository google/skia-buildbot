package regression

import (
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
)

// DSRegression is used for storing Regressions in a RegressionStore.
type DSRegression struct {
	TS      int64
	Triaged bool
	Body    string `datastore:",noindex"`
}

type DetailLookup func(c *cid.CommitID) (*cid.CommitDetail, error)

type RegressionStore interface {
	Untriaged() (int, error)
	Write(regressions map[string]*Regressions, lookup DetailLookup) error
	Range(begin, end int64) (map[string]*Regressions, error)
	SetHigh(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error)
	SetLow(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error)
	TriageLow(cid *cid.CommitDetail, alertID string, tr TriageStatus) error
	TriageHigh(cid *cid.CommitDetail, alertID string, tr TriageStatus) error
}
