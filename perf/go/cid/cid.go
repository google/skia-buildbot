// Package cid contains CommitID and utilities for working with them.
package cid

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/types"
)

// CommitID represents the time of a particular commit, where a commit could either be
// a real commit into the repo, or an event like running a trybot.
//
// TODO(jcgregorio) Collapse this into just types.CommitNumber.
type CommitID struct {
	Offset int `json:"offset"` // The index number of the commit from beginning of time, or the index of the patch number in Reitveld.
}

// ID returns a unique ID for the CommitID.
func (c CommitID) ID() string {
	return fmt.Sprintf("%s-%06d", "master", c.Offset)
}

// CommitIDFromCommitNumber converts a types.CommitNumber into a *CommitID.
//
// This is a transitional step on the way to completely replacing all CommitID
// usage into types.CommitNumber.
func CommitIDFromCommitNumber(n types.CommitNumber) *CommitID {
	return &CommitID{
		Offset: int(n),
	}
}

// FromID is the inverse operator to ID().
func FromID(s string) (*CommitID, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid ID format: %s", s)
	}
	if parts[0] != "master" {
		return nil, fmt.Errorf("Invalid ID format: %s", s)
	}
	i, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Invalid ID format: %s", s)
	}
	return &CommitID{
		Offset: int(i),
	}, nil
}

// CommitDetail describes a CommitID.
type CommitDetail struct {
	CommitID
	Author    string `json:"author"`
	Message   string `json:"message"`
	URL       string `json:"url"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"ts"`
}

// CommitIDLookup allows getting CommitDetails from CommitIDs.
type CommitIDLookup struct {
	git *perfgit.Git

	instanceConfig *config.InstanceConfig
}

// New returns a new CommitIDLookup.
//
// TODO(jcgregorio) Fold this functionality into perf/go/git once CommitID has
// been simplified to just a types.CommitNumber.
func New(ctx context.Context, git *perfgit.Git, instanceConfig *config.InstanceConfig) *CommitIDLookup {
	cidl := &CommitIDLookup{
		git:            git,
		instanceConfig: instanceConfig,
	}
	return cidl
}

// urlFromParts creates the URL to link to a specific commit in a repo.
//
// debouce - See config.PerfBigTableConfig.DebouceCommitURL.
func urlFromParts(repoURL, hash, subject string, debounce bool) string {
	format := config.Config.GitRepoConfig.CommitURL
	if format == "" {
		format = gitiles.CommitURL
	}
	if debounce {
		return subject
	} else {
		return fmt.Sprintf(format, repoURL, hash)
	}
}

// Lookup returns a CommitDetail for each CommitID.
//
// TODO(jcgregorio) Once CommitID is types.CommitNumber then move this functionality into perfgit.Git.
func (c *CommitIDLookup) Lookup(ctx context.Context, cids []*CommitID) ([]*CommitDetail, error) {
	now := time.Now()
	ret := make([]*CommitDetail, len(cids), len(cids))
	for i, cid := range cids {
		commit, err := c.git.CommitFromCommitNumber(ctx, types.CommitNumber(cid.Offset))
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed cid.Lookup.")
		}

		ret[i] = &CommitDetail{
			CommitID:  *cid,
			Author:    commit.Author,
			Message:   fmt.Sprintf("%.7s - %s - %.50s", commit.GitHash, human.Duration(now.Sub(time.Unix(commit.Timestamp, 0))), commit.Subject),
			URL:       urlFromParts(c.instanceConfig.GitRepoConfig.URL, commit.GitHash, commit.Subject, c.instanceConfig.GitRepoConfig.DebouceCommitURL),
			Hash:      commit.GitHash,
			Timestamp: commit.Timestamp,
		}
	}
	return ret, nil
}
