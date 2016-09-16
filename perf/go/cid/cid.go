// Package main provides ...
package cid

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/perf/go/ptraceingest"
	"go.skia.org/infra/perf/go/ptracestore"
)

type CommitDetail struct {
	Author    string `json:"author"`
	Message   string `json:"message"`
	URL       string `json:"url"`
	Timestamp int64  `json:"ts"`
}

type CommitIDLookup struct {
	git *gitinfo.GitInfo
	rv  *rietveld.Rietveld
}

func New(git *gitinfo.GitInfo, rv *rietveld.Rietveld) *CommitIDLookup {
	return &CommitIDLookup{
		git: git,
		rv:  rv,
	}
}

func (c *CommitIDLookup) fromIssue(issue, patchset string) (*ptracestore.CommitID, error) {
	return nil, nil
}

func (c *CommitIDLookup) fromHash(hash string) (*ptracestore.CommitID, error) {
	return nil, nil
}

func (c *CommitIDLookup) Lookup(cids []*ptracestore.CommitID) ([]*CommitDetail, error) {
	glog.Infoln("Starting lookup.")
	ret := make([]*CommitDetail, len(cids), len(cids))
	for i, cid := range cids {
		if strings.HasPrefix(cid.Source, ptraceingest.CODE_REVIEW_URL) {
			parts := strings.Split(cid.Source, "/")
			if len(parts) != 4 {
				return nil, fmt.Errorf("Not a valid source id: %q", cid.Source)
			}
			issueStr := parts[len(parts)-1]
			issueID, err := strconv.ParseInt(issueStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Not a valid issue id %q: %s", issueStr, err)
			}
			issue, err := c.rv.GetIssueProperties(issueID, false)
			if err != nil {
				return nil, fmt.Errorf("Failed to load issue %d: %s", issueID, err)
			}
			if cid.Offset < 0 || cid.Offset > len(issue.Patchsets) {
				return nil, fmt.Errorf("Failed to find patch with offset %d", cid.Offset)
			}
			patchsetID := issue.Patchsets[cid.Offset]
			patchset, err := c.rv.GetPatchset(issueID, patchsetID)
			if err != nil {
				return nil, fmt.Errorf("Failed to load patchset with id %d: %s", patchsetID, err)
			}

			ret[i] = &CommitDetail{
				Author:    issue.Owner,
				Message:   issue.Description,
				URL:       cid.Source,
				Timestamp: patchset.Created.Unix(),
			}
		} else {
			// Presume that it is a branch name.
			lc, err := c.git.ByIndex(cid.Offset)
			if err != nil {
				return nil, fmt.Errorf("Failed to find match for cid %#v: %s", *cid, err)
			}
			ret[i] = &CommitDetail{
				Author:    lc.Author,
				Message:   lc.ShortCommit.Subject,
				URL:       fmt.Sprintf("https://skia.googlesource.com/skia/+/%s", lc.Hash),
				Timestamp: lc.Timestamp.Unix(),
			}
		}
	}
	return ret, nil
}
