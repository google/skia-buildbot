// Package cid contains CommitID and utilities for working with them.
package cid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
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

// ID returns a unique ID for the CommitID.
func (c CommitID) ID() string {
	return fmt.Sprintf("%s-%06d", safeRe.ReplaceAllLiteralString(c.Source, "_"), c.Offset)
}

// FromID is the inverse operator to ID().
func FromID(s string) (*CommitID, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid ID format: %s", s)
	}
	if strings.Contains(parts[0], "_") {
		return nil, fmt.Errorf("Invalid ID format: %s", s)
	}
	i, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Invalid ID format: %s", s)
	}
	return &CommitID{
		Offset: int(i),
		Source: parts[0],
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
		sklog.Warningf("Commit %s is not in master branch.", hash)
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

// cacheEntry is used in the cache of CommitIDLookup.
type cacheEntry struct {
	author  string
	subject string
	hash    string
	ts      int64
}

// CommitIDLookup allows getting CommitDetails from CommitIDs.
type CommitIDLookup struct {
	git *gitinfo.GitInfo
	rv  *rietveld.Rietveld

	// mutex protects access to cache.
	mutex sync.Mutex

	// cache information about commits to "master", by their offset from the
	// first commit.
	cache map[int]*cacheEntry

	gitRepoURL string
}

// parseLogLine parses a single log line from running git log
// --format="format:%ct %H %ae %s" and converts it into a cacheEntry.
//
// index is the index of the last commit id, or -1 if we don't know which
// commit id we are on.
func parseLogLine(s string, index *int, git *gitinfo.GitInfo) (*cacheEntry, error) {
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
	if *index == -1 {
		*index, err = git.IndexOf(hash)
		if err != nil {
			return nil, fmt.Errorf("Failed to get index of %q: %s", hash, err)
		}
	} else {
		*index++
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
func (c *CommitIDLookup) warmCache() {
	defer timer.New("cid.warmCache time").Stop()
	now := time.Now()

	// Extract ts, hash, author email, and subject from the git log.
	since := now.Add(-365 * 24 * time.Hour).Format("2006-01-02")
	log, err := c.git.LogArgs("--since="+since, "--format=format:%ct %H %ae %s")
	if err != nil {
		sklog.Errorf("Could not get log for --since=%q: %s", since, err)
		return
	}

	lines := util.Reverse(strings.Split(log, "\n"))
	// Get the index of the first commit, and then increment from there.
	var index int = -1
	// Parse.
	for _, s := range lines {
		entry, err := parseLogLine(s, &index, c.git)
		if err != nil {
			sklog.Errorf("Failed to parse git log line %q: %s", s, err)
			return
		}
		c.cache[index] = entry
	}
}

func New(git *gitinfo.GitInfo, rv *rietveld.Rietveld, gitRepoURL string) *CommitIDLookup {
	cidl := &CommitIDLookup{
		git:        git,
		rv:         rv,
		cache:      map[int]*cacheEntry{},
		gitRepoURL: gitRepoURL,
	}
	cidl.warmCache()
	return cidl
}

// Lookup returns a CommitDetail for each CommitID.
func (c *CommitIDLookup) Lookup(cids []*CommitID) ([]*CommitDetail, error) {
	now := time.Now()
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
				CommitID:  *cid,
				Author:    issue.Owner,
				Message:   fmt.Sprintf("Iss: %d Patch: %d - %s", issueID, patchsetID, issue.Description),
				URL:       cid.Source,
				Timestamp: patchset.Created.Unix(),
			}
		} else if cid.Source == "master" {
			c.mutex.Lock()
			entry, ok := c.cache[cid.Offset]
			c.mutex.Unlock()
			if ok {
				ret[i] = &CommitDetail{
					CommitID:  *cid,
					Author:    entry.author,
					Message:   fmt.Sprintf("%.7s - %s - %.50s", entry.hash, human.Duration(now.Sub(time.Unix(entry.ts, 0))), entry.subject),
					URL:       fmt.Sprintf("%s/+/%s", c.gitRepoURL, entry.hash),
					Hash:      entry.hash,
					Timestamp: entry.ts,
				}
			} else {
				lc, err := c.git.ByIndex(cid.Offset)
				if err != nil {
					return nil, fmt.Errorf("Failed to find match for cid %#v: %s", *cid, err)
				}
				ret[i] = &CommitDetail{
					CommitID:  *cid,
					Author:    lc.Author,
					Message:   fmt.Sprintf("%.7s - %s - %.50s", lc.Hash, human.Duration(now.Sub(lc.Timestamp)), lc.ShortCommit.Subject),
					URL:       fmt.Sprintf("%s/+/%s", c.gitRepoURL, lc.Hash),
					Hash:      lc.Hash,
					Timestamp: lc.Timestamp.Unix(),
				}
				c.mutex.Lock()
				c.cache[cid.Offset] = &cacheEntry{
					author:  lc.Author,
					subject: lc.ShortCommit.Subject,
					hash:    lc.Hash,
					ts:      lc.Timestamp.Unix(),
				}
				c.mutex.Unlock()
			}
		} else {
			return nil, fmt.Errorf("Using branches other than 'master' is currently unimplemented.")
		}
	}
	return ret, nil
}
