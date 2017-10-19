package db

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	MAX_ISSUE_CACHE_SIZE = 1000

	MAX_TILE_CACHE_SIZE = 10
)

// CommitIDLong contains more detailed information about each commit,
// regardless of whether it came from an actual commit or a trybot.
type CommitIDLong struct {
	*CommitID
	Author string `json:"author"`
	Desc   string `json:"desc"`

	// Details contains the related information either from git or a codereview system..
	// A nil value means the codereview system failed to respond or the issue was not available.
	Details interface{} `json:"-"`
}

// BranchTileBuilder is a high level interface to build tiles base on a datasource that
// originated via a version control system or from a code review system via trybot
// runs.
type BranchTileBuilder interface {
	// ListLong returns a slice of CommitIDLongs that appear in the given time
	// range from begin to end, and may be filtered by the 'source' parameter. If
	// 'source' is the empty string then no filtering is done.
	ListLong(begin, end time.Time, source string) ([]*CommitIDLong, error)

	// CachedTileFromCommits creates a tile from the given commits. The tile is cached.
	CachedTileFromCommits(commits []*CommitID) (*tiling.Tile, error)
}

// cachedTile is used in the caching of tiles. It holds the tile and the md5
// hashes of the data that make up the tile.
type cachedTile struct {
	tile *tiling.Tile
	md5  string
}

// tileBuilder implements BranchTileBuilder.
type tileBuilder struct {
	db                 DB
	vcs                vcsinfo.VCS
	rietveldReview     *rietveld.Rietveld
	rietveldReviewURL  string
	rietveldIssueCache *rietveld.CodeReviewCache

	gerritReview          *gerrit.Gerrit
	gerritReviewURL       string
	gerritChangeInfoCache *gerrit.CodeReviewCache

	// tcache is a cache for tiles built from CachedTileFromCommits, it stores 'cachedTile's.
	tcache *lru.Cache

	// mutex protects access to the caches.
	mutex sync.Mutex
}

// NewBranchTileBuilder returns an instance of BranchTileBuilder that allows
// creating tiles based on the given VCS or code review system based on
// querying db.
//
// TODO(stephana): The EventBus is used to update the internal cache as commits are updated.
func NewBranchTileBuilder(db DB, vcs vcsinfo.VCS, rietveldReview *rietveld.Rietveld, gerritReview *gerrit.Gerrit, evt eventbus.EventBus) BranchTileBuilder {
	return &tileBuilder{
		db:                 db,
		vcs:                vcs,
		rietveldReview:     rietveldReview,
		rietveldReviewURL:  rietveldReview.Url(0),
		rietveldIssueCache: rietveld.NewCodeReviewCache(rietveldReview, time.Minute, MAX_ISSUE_CACHE_SIZE),

		gerritReview:          gerritReview,
		gerritReviewURL:       gerritReview.Url(0),
		gerritChangeInfoCache: gerrit.NewCodeReviewCache(gerritReview, time.Minute, MAX_ISSUE_CACHE_SIZE),
		tcache:                lru.New(MAX_TILE_CACHE_SIZE),
	}
}

// CachedTileFromCommits returns a tile built from the given commits. The tiles are
// cached to speed up subsequent requests.
func (b *tileBuilder) CachedTileFromCommits(commits []*CommitID) (*tiling.Tile, error) {
	key := ""
	for _, cid := range commits {
		key += cid.String()
	}
	md5 := ""
	if hashes, err := b.db.ListMD5(commits); err == nil {
		md5 = strings.Join(hashes, "")
		sklog.Infof("Got md5: %s", md5)
	} else {
		sklog.Errorf("Failed to load the md5 hashes for a slice of commits: %s", err)
	}

	// Determine if we need to fetch a fresh tile from tracedb.
	getFreshTile := false
	b.mutex.Lock()
	interfaceCacheEntry, ok := b.tcache.Get(key)
	b.mutex.Unlock()
	var tileCacheEntry *cachedTile = nil
	if !ok {
		getFreshTile = true
	} else {
		tileCacheEntry, ok = interfaceCacheEntry.(*cachedTile)
		if !ok {
			getFreshTile = true
		} else if md5 != tileCacheEntry.md5 {
			getFreshTile = true
		}
	}
	if getFreshTile {
		sklog.Info("Tile is missing or expired.")
		tile, hashes, err := b.db.TileFromCommits(commits)
		if err != nil {
			return nil, fmt.Errorf("Unable to create fresh tile: %s", err)
		}
		md5 := strings.Join(hashes, "")
		if md5 == "" {
			sklog.Errorf("Not caching, didn't get a valid set of hashes, is traceserverd out of date? : %s", key)
		} else {
			b.mutex.Lock()
			b.tcache.Add(key, &cachedTile{
				tile: tile,
				md5:  strings.Join(hashes, ""),
			})
			b.mutex.Unlock()
		}
		return tile, nil
	} else {
		return tileCacheEntry.tile, nil
	}
}

// See the TileBuilder interface.
func (b *tileBuilder) ListLong(begin, end time.Time, source string) ([]*CommitIDLong, error) {
	commitIDs, err := b.db.List(begin, end)
	if err != nil {
		return nil, fmt.Errorf("Error while looking up commits: %s", err)
	}
	return b.convertToLongCommits(commitIDs, source), nil
}

// ShortFromLong converts a slice of CommitIDLong to a slice of CommitID.
func ShortFromLong(commitIDs []*CommitIDLong) []*CommitID {
	shortCids := make([]*CommitID, len(commitIDs))
	for idx, cid := range commitIDs {
		shortCids[idx] = cid.CommitID
	}
	return shortCids
}

// convertToLongCommits converts the CommitIDs into CommitIDLong's, after
// potentially filtering the slice based on the provided source.
func (b *tileBuilder) convertToLongCommits(commitIDs []*CommitID, source string) []*CommitIDLong {
	// Filter
	if source != "" {
		dst := make([]*CommitID, 0, len(commitIDs))
		for _, cid := range commitIDs {
			if strings.HasPrefix(cid.Source, source) {
				dst = append(dst, cid)
			}
		}
		commitIDs = dst
	}

	// Convert to CommitIDLong.
	results := []*CommitIDLong{}
	for _, cid := range commitIDs {
		results = append(results, &CommitIDLong{
			CommitID: cid,
			Details:  nil,
		})
	}

	// Populate Author and Desc from gitinfo or code review systems as appropriate.
	// Caching Rietveld info as needed.
	for _, c := range results {
		if strings.HasPrefix(c.Source, b.rietveldReviewURL) {
			// Rietveld
			issueInfo, err := b.getRietveldIssue(c.Source)
			if err != nil {
				// Only a warning since users can delete Rietveld issues.
				sklog.Warningf("Failed to get details for commit from Rietveld %s: %s", c.ID, err)
				continue
			}
			c.Author = issueInfo.Owner
			c.Desc = issueInfo.Subject
			c.Details = issueInfo
		} else if strings.HasPrefix(c.Source, b.gerritReviewURL) {
			changeInfo, err := b.getGerritIssue(c.Source)
			if err != nil {
				// Only a warning since users can delete Gerrit issues.
				sklog.Warningf("Failed to get details for commit from Gerrit %s: %s", c.ID, err)
				continue
			}

			c.Author = changeInfo.Owner.Email
			c.Desc = changeInfo.Subject
			c.Details = changeInfo
		} else {
			// vcsinfo
			details, err := b.vcs.Details(c.ID, true)
			if err != nil {
				sklog.Errorf("Failed to get details for commit from Git %s: %s", c.ID, err)
				continue
			}
			c.Author = details.Author
			c.Desc = details.Subject
			c.Details = details
		}
	}

	return results
}

// getRietveldIssue parses the source, which looks like
// "https://chromium.codereview.org/1232143243" and returns information about
// the issue from Rietveld.
func (b *tileBuilder) getRietveldIssue(source string) (*rietveld.Issue, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse trybot source: %s", err)
	}

	issueStr := u.Path[1:]
	issueInt, err := strconv.ParseInt(issueStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert Rietveld issue id: %s", err)
	}

	var issue *rietveld.Issue
	var ok bool
	if issue, ok = b.rietveldIssueCache.Get(issueInt); !ok {
		issue, err = b.rietveldReview.GetIssueProperties(int64(issueInt), false)
		if err != nil {
			return nil, fmt.Errorf("Failed to get details for review %s: %s", source, err)
		}
		b.rietveldIssueCache.Add(issueInt, issue)
	}
	return issue, nil
}

// getGerritIssue parses the source, which should look like
// https://skia-review.googlesource.com/c/2781/ and returns information about
// the issue from Gerrit.
func (b *tileBuilder) getGerritIssue(source string) (*gerrit.ChangeInfo, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse trybot source: %s", err)
	}

	splitPath := strings.Split(strings.Trim(u.Path, "/"), "/")
	if (len(splitPath) != 2) || (splitPath[0] != "c") {
		return nil, fmt.Errorf("Error parsing Gerrit URL. Path format should be /c/<id>. Got %s", u.Path)
	}
	changeInfoID, err := strconv.ParseInt(splitPath[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert Gerrit issue id: %s. Got error: %s", splitPath[1], err)
	}

	var changeInfo *gerrit.ChangeInfo
	var ok bool
	if changeInfo, ok = b.gerritChangeInfoCache.Get(changeInfoID); !ok {
		changeInfo, err = b.gerritReview.GetIssueProperties(changeInfoID)
		if err != nil {
			return nil, fmt.Errorf("Failed to get details for Gerrit CL %s: %s", source, err)
		}
		b.gerritChangeInfoCache.Add(changeInfoID, changeInfo)
	}
	return changeInfo, nil
}
