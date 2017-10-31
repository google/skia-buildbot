package bbtrybot

import (
	"time"

	"go.skia.org/infra/go/buildbucket"
)

type TryjobStatus string

const (
	TRYJOB_SCHEDULED TryjobStatus = "scheduled"
	TRYJOB_RUNNING   TryjobStatus = "running"
	TRYJOB_COMPLETE  TryjobStatus = "complete"
	TRYJOB_INGESTED  TryjobStatus = "ingested"
	TRYJOB_FAILED    TryjobStatus = "failed"
	TRYJOB_UNKNOWN   TryjobStatus = "unknown"
)

type Parameters = buildbucket.Parameters
type Properties = buildbucket.Properties

// Issue captures information about a single Rietveld issued.
type Issue struct {
	ID        int64     `json:"id"`
	Subject   string    `json:"subject"`
	Owner     string    `json:"owner"`
	Updated   time.Time `json:"updated"`
	URL       string    `json:"url"`
	Patchsets []int64   `json:"patchsets"`
}

// IssueDetails extends issue with the commit ideas for the issue.
type IssueDetails struct {
	*Issue
	PatchsetDetails map[int64]*PatchsetDetail `json:"-"`
	dirty           bool
}

type PatchsetDetail struct {
	ID       int64     `json:"id"`
	Tryjobs  []*Tryjob `json:"tryjobs"`
	JobTotal int64     `json:"jobTotal"`
	JobDone  int64     `json:"jobDone"`
}

type Tryjob struct {
	Builder     string       `json:"builder"`
	Buildnumber int64        `json:"buildnumber"`
	Status      TryjobStatus `json:"status"`
}

func (i *IssueDetails) addBuild(patchsetID int64, tryjob *Tryjob) (bool, error) {

	// // Find the patchset
	// params.Properties.GerritPatchset

	// // Add the build to the patchset information

	// //
	// trybot := i.findTryjob(params.BuilderName, build.Id)
	// prop := params.Properties
	// issueID := prop.GerritIssue
	// patchsetID := prop.GerritPatchset

	// if err != nil {
	// 	return false, fmt.Errorf("Unable to parse issue id '%s'. Got error: %s", prop.Gerri)
	// }

	// // buildbucket.RESULT_FAILURE

	return false, nil
}
