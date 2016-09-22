// Package cid contains CommitID and utilities for working with them.
package cid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/constants"
)

const (
	CODE_REVIEW_URL = "https://codereview.chromium.org"
)

var (
	// safeRe is used in CommitID.Filename() to replace unsafe chars in a filename.
	safeRe = regexp.MustCompile("[^a-zA-Z0-9]")
)

// CommitID represents the time of a particular commit, where a commit could either be
// a real commit into the repo, or an event like running a trybot.
type CommitID struct {
	Offset int    `json:"offset"` // The index number of the commit from beginning of time, or the index of the patch number in Reitveld.
	Source string `json:"source"` // The branch name, e.g. "master", or the Reitveld issue id.
}

// Filename returns a safe filename to be used as part of the underlying BoltDB tile name.
func (c CommitID) Filename() string {
	return fmt.Sprintf("%s-%06d.bdb", safeRe.ReplaceAllLiteralString(c.Source, "_"), c.Offset/constants.COMMITS_PER_TILE)
}

// CommitDetail describes a CommitID.
type CommitDetail struct {
	Author    string `json:"author"`
	Message   string `json:"message"`
	URL       string `json:"url"`
	Timestamp int64  `json:"ts"`
}

// FromIssue returns a CommitID for the given Rietveld issue and patchset.
func FromIssue(review *rietveld.Rietveld, issueStr, patchsetStr string) (*CommitID, error) {
	patchset, err := strconv.ParseInt(patchsetStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse trybot patch id: %s", err)
	}
	issueID, err := strconv.ParseInt(issueStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse trybot issue id: %s", err)
	}

	issue, err := review.GetIssueProperties(issueID, false)
	if err != nil {
		return nil, fmt.Errorf("Failed to get issue details %d: %s", issueID, err)
	}
	// Look through the Patchsets and find a matching one.
	var offset int = -1
	for i, pid := range issue.Patchsets {
		if pid == patchset {
			offset = i
			break
		}
	}
	if offset == -1 {
		return nil, fmt.Errorf("Failed to find patchset %d in review %d", patchset, issueID)
	}

	return &CommitID{
		Offset: offset,
		Source: fmt.Sprintf("%s/%s", CODE_REVIEW_URL, issueStr),
	}, nil
}

// FromHash returns a CommitID for the given git hash.
func FromHash(vcs vcsinfo.VCS, hash string) (*CommitID, error) {
	commit, err := vcs.Details(hash, true)
	if err != nil {
		return nil, err
	}
	if !commit.Branches["master"] {
		glog.Warningf("Commit %s is not in master branch.", hash)
		return nil, ingestion.IgnoreResultsFileErr
	}
	offset, err := vcs.IndexOf(hash)
	if err != nil {
		return nil, fmt.Errorf("Could not ingest, hash not found %q: %s", hash, err)
	}
	return &CommitID{
		Offset: offset,
		Source: "master",
	}, nil
}

// CommitIDLookup allows getting CommitDetails from CommitIDs.
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

// Lookup returns a CommitDetail for each CommitID.
func (c *CommitIDLookup) Lookup(cids []*CommitID) ([]*CommitDetail, error) {
	ret := make([]*CommitDetail, len(cids), len(cids))
	for i, cid := range cids {
		if strings.HasPrefix(cid.Source, CODE_REVIEW_URL) {
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
				Message:   fmt.Sprintf("Iss: %d Patch: %d - %s", issueID, patchsetID, issue.Description),
				URL:       cid.Source,
				Timestamp: patchset.Created.Unix(),
			}
		} else {
			// Presume that cid.Source is a branch name.
			lc, err := c.git.ByIndex(cid.Offset)
			if err != nil {
				return nil, fmt.Errorf("Failed to find match for cid %#v: %s", *cid, err)
			}
			ret[i] = &CommitDetail{
				Author:    lc.Author,
				Message:   fmt.Sprintf("%.7s - %s", lc.Hash, lc.ShortCommit.Subject),
				URL:       fmt.Sprintf("https://skia.googlesource.com/skia/+/%s", lc.Hash),
				Timestamp: lc.Timestamp.Unix(),
			}
		}
	}
	return ret, nil
}
