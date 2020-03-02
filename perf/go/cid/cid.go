// Package cid contains CommitID and utilities for working with them.
package cid

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

// ID returns a string ID for the CommitNumber.
func ID(c types.CommitNumber) string {
	return fmt.Sprintf("%s-%06d", "master", c)
}

// FromID is the inverse operator to ID().
func FromID(s string) (types.CommitNumber, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return types.BadCommitNumber, fmt.Errorf("Invalid ID format: %s", s)
	}
	if parts[0] != "master" {
		return types.BadCommitNumber, fmt.Errorf("Invalid ID format: %s", s)
	}
	i, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return types.BadCommitNumber, fmt.Errorf("Invalid ID format: %s", s)
	}
	return types.CommitNumber(i), nil
}

// CommitDetail describes a CommitID.
type CommitDetail struct {
	CommitID  types.CommitNumber
	Author    string `json:"author"`
	Message   string `json:"message"`
	URL       string `json:"url"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"ts"`
}

// FromHash returns a CommitID for the given git hash.
func FromHash(ctx context.Context, vcs vcsinfo.VCS, hash string) (types.CommitNumber, error) {
	commit, err := vcs.Details(ctx, hash, true)
	if err != nil {
		return types.BadCommitNumber, err
	}
	if !commit.Branches["master"] {
		return types.BadCommitNumber, fmt.Errorf("Commit %s is not in master branch.", hash)
	}
	offset, err := vcs.IndexOf(ctx, hash)
	if err != nil {
		return types.BadCommitNumber, fmt.Errorf("Could not ingest, hash not found %q: %s", hash, err)
	}
	return types.CommitNumber(offset), nil
}

// cacheEntry is used in the cache of CommitIDLookup.
type cacheEntry struct {
	author  string
	subject string
	hash    string
	ts      int64
}

// CommitIDLookup allows getting CommitDetails from CommitIDs.
type CommitIDLookup struct {
	vcs vcsinfo.VCS

	// mutex protects access to cache.
	mutex sync.Mutex

	// cache information about commits to "master", by their offset from the
	// first commit.
	cache map[types.CommitNumber]*cacheEntry

	gitRepoURL string
}

// parseLogLine parses a single log line from running git log
// --format="format:%ct %H %ae %s" and converts it into a cacheEntry.
//
// index is the index of the last commit id, or -1 if we don't know which
// commit id we are on.
func parseLogLine(ctx context.Context, s string, commitNumber *types.CommitNumber, vcs vcsinfo.VCS) (*cacheEntry, error) {
	parts := strings.SplitN(s, " ", 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("Failed to parse parts of %q: %#v", s, parts)
	}
	ts := parts[0]
	hash := parts[1]
	author := parts[2]
	subject := parts[3]
	tsi, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Can't parse timestamp %q: %s", ts, err)
	}
	if *commitNumber == types.BadCommitNumber {
		index, err := vcs.IndexOf(ctx, hash)
		*commitNumber = types.CommitNumber(index)
		if err != nil {
			return nil, fmt.Errorf("Failed to get index of %q: %s", hash, err)
		}
	} else {
		*commitNumber++
	}
	return &cacheEntry{
		author:  author,
		subject: subject,
		hash:    hash,
		ts:      tsi,
	}, nil
}

// warmCache populates c.cache with all the commits to "master"
// in the past year.
func (c *CommitIDLookup) warmCache(ctx context.Context) {
	defer timer.New("cid.warmCache time").Stop()
	now := time.Now()

	// TODO(jcgregorio) Remove entire cache once we switch to a BigTable backed vcsinfo.

	// Extract ts, hash, author email, and subject from the git log.
	since := now.Add(-365 * 24 * time.Hour).Format("2006-01-02")
	g, ok := c.vcs.(*gitinfo.GitInfo)
	if !ok {
		return
	}
	log, err := g.LogArgs(ctx, "--since="+since, "--format=format:%ct %H %ae %s")
	if err != nil {
		sklog.Errorf("Could not get log for --since=%q: %s", since, err)
		return
	}

	lines := util.Reverse(strings.Split(log, "\n"))
	// Get the index of the first commit, and then increment from there.
	var index types.CommitNumber = types.BadCommitNumber
	// Parse.
	for _, s := range lines {
		entry, err := parseLogLine(ctx, s, &index, c.vcs)
		if err != nil {
			sklog.Errorf("Failed to parse git log line %q: %s", s, err)
			return
		}
		c.cache[index] = entry
	}
}

// New returns a new CommitIDLookup.
func New(ctx context.Context, vcs vcsinfo.VCS, gitRepoURL string) *CommitIDLookup {
	cidl := &CommitIDLookup{
		vcs:        vcs,
		cache:      map[types.CommitNumber]*cacheEntry{},
		gitRepoURL: gitRepoURL,
	}
	cidl.warmCache(ctx)
	return cidl
}

// urlFromParts creates the URL to link to a specific commit in a repo.
//
// debouce - See config.PerfBigTableConfig.DebouceCommitURL.
func urlFromParts(repoURL, hash, subject string, debounce bool) string {
	if debounce {
		return subject
	} else {
		return fmt.Sprintf("%s/+/%s", repoURL, hash)
	}
}

func (c *CommitIDLookup) getFromCache(commitNumber types.CommitNumber) (*cacheEntry, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry, ok := c.cache[commitNumber]
	return entry, ok
}

func (c *CommitIDLookup) addToCache(commitNumber types.CommitNumber, entry *cacheEntry) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[commitNumber] = entry
}

// Lookup returns a CommitDetail for each CommitID.
func (c *CommitIDLookup) Lookup(ctx context.Context, commitNumbers []types.CommitNumber) ([]*CommitDetail, error) {
	now := time.Now()
	ret := make([]*CommitDetail, len(commitNumbers), len(commitNumbers))
	for i, commitNumber := range commitNumbers {
		if commitNumber < 0 {
			continue
		}
		entry, ok := c.getFromCache(commitNumber)
		if ok {
			ret[i] = &CommitDetail{
				CommitID:  commitNumber,
				Author:    entry.author,
				Message:   fmt.Sprintf("%.7s - %s - %.50s", entry.hash, human.Duration(now.Sub(time.Unix(entry.ts, 0))), entry.subject),
				URL:       urlFromParts(c.gitRepoURL, entry.hash, entry.subject, config.Config.GitRepoConfig.DebouceCommitURL),
				Hash:      entry.hash,
				Timestamp: entry.ts,
			}
		} else {
			lc, err := c.vcs.ByIndex(ctx, int(commitNumber))
			if err != nil {
				return nil, fmt.Errorf("Failed to find match for cid %#v: %s", commitNumber, err)
			}
			ret[i] = &CommitDetail{
				CommitID:  commitNumber,
				Author:    lc.Author,
				Message:   fmt.Sprintf("%.7s - %s - %.50s", lc.Hash, human.Duration(now.Sub(lc.Timestamp)), lc.ShortCommit.Subject),
				URL:       urlFromParts(c.gitRepoURL, lc.Hash, lc.Subject, config.Config.GitRepoConfig.DebouceCommitURL),
				Hash:      lc.Hash,
				Timestamp: lc.Timestamp.Unix(),
			}
			c.addToCache(commitNumber, &cacheEntry{
				author:  lc.Author,
				subject: lc.ShortCommit.Subject,
				hash:    lc.Hash,
				ts:      lc.Timestamp.Unix(),
			})
		}
	}
	return ret, nil
}
