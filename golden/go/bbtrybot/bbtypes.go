package bbtrybot

import (
	"fmt"
	"time"

	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"

	"go.skia.org/infra/go/buildbucket"
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
	TargetPatchsets []string                  `json:"-"`
}

type PatchsetDetail struct {
	ID       int64     `json:"id"`
	Tryjobs  []*Tryjob `json:"tryjobs"`
	JobTotal int64     `json:"jobTotal"`
	JobDone  int64     `json:"jobDone"`
	Digests  int64     `json:"digests"`
	InMaster int64     `json:"inMaster"`
	Url      string    `json:"url"`
}

type Tryjob struct {
	Builder     string `json:"builder"`
	Buildnumber string `json:"buildnumber"`
	Status      string `json:"status"`
}

func (i *IssueDetails) addBuild(build *bb_api.ApiCommonBuildMessage, params *Parameters) (bool, error) {
	prop := params.Properties
	issueID := prop.GerritIssue
	patchsetID := prop.GerritPatchset

	if err != nil {
		return false, fmt.Errorf("Unable to parse issue id '%s'. Got error: %s", prop.Gerri)
	}

	return false, nil
}
