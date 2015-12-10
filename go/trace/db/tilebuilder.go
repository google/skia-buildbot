package db

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	MAX_CACHE_SIZE = 1000
)

type tileBuilder struct {
	*Builder

	vcs    vcsinfo.VCS
	review *rietveld.Rietveld
	// cache is a cache for rietveld.Issue's. Note that gitinfo has its own cache
	// for Details(), so we don't need to cache the results.
	cache map[string]*rietveld.Issue
}

func NewTileBuilder(git *gitinfo.GitInfo, address string, tileSize int, traceBuilder tiling.TraceBuilder, reviewURL string) (tiling.TileBuilder, error) {
	review := rietveld.New(reviewURL, util.NewTimeoutClient())
	builder, err := NewBuilder(git, address, tileSize, traceBuilder)
	if err != nil {
		return nil, fmt.Errorf("Failed to construct Builder: %s", err)
	}

	return &tileBuilder{
		Builder: builder,
		vcs:     git,
		review:  review,
		cache:   map[string]*rietveld.Issue{},
	}, nil
}

// See the tiling.TileBuilder interface.
func (b *tileBuilder) ListLong(begin, end time.Time, source string) ([]*tiling.CommitIDLong, error) {
	commitIDs, err := b.DB.List(begin, end)
	if err != nil {
		return nil, fmt.Errorf("Error while looking up commits: %s", err)
	}
	return b.convertToLongCommits(commitIDs, source), nil
}

// convertToLongCommits converts the CommitIDs into CommitIDLong's, after
// potentially filtering the slice based on the provided source.
func (b *tileBuilder) convertToLongCommits(commitIDs []*CommitID, source string) []*tiling.CommitIDLong {
	// Filter
	if source != "" {
		dst := []*CommitID{}
		for _, cid := range commitIDs {
			if cid.Source == source {
				dst = append(dst, cid)
			}
		}
		commitIDs = dst
	}

	// Convert to CommitIDLong.
	results := []*tiling.CommitIDLong{}
	for _, cid := range commitIDs {
		results = append(results, &tiling.CommitIDLong{
			Timestamp: cid.Timestamp.Unix(),
			ID:        cid.ID,
			Source:    cid.Source,
		})
	}

	// Populate Author and Desc from gitinfo or rietveld as appropriate.
	// Caching Rietveld info as needed.
	for _, c := range results {
		if strings.HasPrefix(c.Source, "https://codereview.chromium.org") {
			// Rietveld
			issueInfo, err := b.getIssue(c.Source)
			if err != nil {
				glog.Errorf("Failed to get details for commit from Rietveld %s: %s", c.ID, err)
				continue
			}
			c.Author = issueInfo.Owner
			c.Desc = issueInfo.Subject
		} else {
			// vcsinfo
			details, err := b.vcs.Details(c.ID)
			if err != nil {
				glog.Errorf("Failed to get details for commit from Git %s: %s", c.ID, err)
				continue
			}
			c.Author = details.Author
			c.Desc = details.Subject
		}
	}

	return results
}

// getIssue parses the source, which looks like
// "https://chromium.codereview.org/1232143243" and returns information about
// the issue from Rietveld.
func (b *tileBuilder) getIssue(source string) (*rietveld.Issue, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse trybot source: %s", err)
	}
	// The issue id as a string is the URL path w/o the leading slash.
	issueStr := u.Path[1:]
	if issue, ok := b.cache[issueStr]; !ok {
		issueInt, err := strconv.Atoi(issueStr)
		if err != nil {
			return nil, fmt.Errorf("Unable to convert Rietveld issue id: %s", err)
		}
		issue, err = b.review.GetIssueProperties(int64(issueInt), false)
		if err != nil {
			return nil, fmt.Errorf("Failed to get details for review %s: %s", source, err)
		}
		if len(b.cache) > MAX_CACHE_SIZE {
			b.cache = map[string]*rietveld.Issue{}
		}
		b.cache[issueStr] = issue
		return issue, nil
	} else {
		return issue, nil
	}
}
