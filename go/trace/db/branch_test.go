package db

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/groupcache/lru"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/types"
)

const (
	GERRIT_TEST_FILE = "testdata/gerrit_response.txt"
	GERRIT_URL       = "https://skia-review.googlesource.com"
	GERRIT_ISSUE_URL = GERRIT_URL + "/c/2781/"
)

func TestPerfTrace(t *testing.T) {
	testutils.LargeTest(t)
	b, err := ioutil.ReadFile(filepath.Join("testdata", "rietveld_response.txt"))
	assert.NoError(t, err)

	gerritResp, err := ioutil.ReadFile(GERRIT_TEST_FILE)
	assert.NoError(t, err)

	m := mockhttpclient.NewURLMock()
	// Mock this only once to confirm that caching works.
	m.MockOnce("https://codereview.chromium.org/api/1490543002", mockhttpclient.MockGetDialogue(b))

	issueAPIUrl := fmt.Sprintf("%s/changes/%d/detail?o=ALL_REVISIONS", GERRIT_URL, 2781)
	m.MockOnce(issueAPIUrl, mockhttpclient.MockGetDialogue(gerritResp))

	rietveldReview := rietveld.New(rietveld.RIETVELD_SKIA_URL, httputils.NewTimeoutClient())
	gerritReview, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", m.Client())
	assert.NoError(t, err)

	vcsCommits := []*vcsinfo.LongCommit{
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "foofoofoo",
				Author:  "bar@example.com",
				Subject: "some commit",
			},
		},
	}
	vcs := ingestion.MockVCS(vcsCommits)

	builder := NewBranchTileBuilder(nil, vcs, rietveldReview, gerritReview, nil).(*tileBuilder)

	now := time.Unix(100, 0)
	commits := []*CommitID{
		&CommitID{
			Timestamp: time.Now().Unix(),
			ID:        "1",
			Source:    "https://codereview.chromium.org/1490543002",
		},
		&CommitID{
			Timestamp: time.Now().Unix(),
			ID:        "2",
			Source:    GERRIT_ISSUE_URL,
		},
		&CommitID{
			Timestamp: time.Now().Unix(),
			ID:        "foofoofoo",
			Source:    "master",
		},
	}

	long := builder.convertToLongCommits(commits, "master")
	assert.Equal(t, 1, len(long), "Only one commit should match master.")
	assert.Equal(t, "foofoofoo", long[0].ID)
	assert.Equal(t, "some commit", long[0].Desc)
	assert.Equal(t, "bar@example.com", long[0].Author)

	long = builder.convertToLongCommits(commits, "https://codereview.chromium.org/1490543002")
	assert.Equal(t, 1, len(long), "Only one commit should match the trybot.")
	assert.Equal(t, "1", long[0].ID)
	assert.Equal(t, "no merge conflicts here.", long[0].Desc)
	assert.Equal(t, "jcgregorio", long[0].Author)

	long = builder.convertToLongCommits(commits, GERRIT_URL)
	assert.Equal(t, 1, len(long), "Only one commit should match the Gerrit trybot.")
	assert.Equal(t, "2", long[0].ID)
	assert.Equal(t, "Some Test subject", long[0].Desc)
	assert.Equal(t, "jdoe@example.com", long[0].Author)

	long = builder.convertToLongCommits(commits, "")
	assert.Equal(t, 3, len(long), "All commits should now appear.")
	assert.Equal(t, "1", long[0].ID)
	assert.Equal(t, "no merge conflicts here.", long[0].Desc)
	assert.Equal(t, "jcgregorio", long[0].Author)

	assert.Equal(t, "2", long[1].ID)
	assert.Equal(t, "Some Test subject", long[1].Desc)
	assert.Equal(t, "jdoe@example.com", long[1].Author)

	assert.Equal(t, "foofoofoo", long[2].ID)
	assert.Equal(t, "some commit", long[2].Desc)
	assert.Equal(t, "bar@example.com", long[2].Author)

	badCommits := []*CommitID{
		&CommitID{
			Timestamp: now.Add(2 * time.Minute).Unix(),
			ID:        "2",
			Source:    "https://codereview.chromium.org/99999999",
		},
		&CommitID{
			Timestamp: now.Add(3 * time.Minute).Unix(),
			ID:        "barbarbar",
			Source:    "master",
		},
	}
	long = builder.convertToLongCommits(badCommits, "")
	assert.Equal(t, 2, len(long), "Both commits should now appear.")
	assert.Equal(t, "2", long[0].ID)
	assert.Equal(t, "", long[0].Desc)
	assert.Equal(t, "", long[0].Author)
	assert.Equal(t, "barbarbar", long[1].ID)
	assert.Equal(t, "", long[1].Desc)
	assert.Equal(t, "", long[1].Author)
}

func TestTileFromCommits(t *testing.T) {
	testutils.SmallTest(t)
	ts, cleanup := setupClientServerForTesting(t.Fatalf)
	defer cleanup()

	now := time.Unix(100, 0)

	commitIDs := []*CommitID{
		&CommitID{
			Timestamp: now.Unix(),
			ID:        "foofoofoo",
			Source:    "master",
		},
	}

	vcsCommits := []*vcsinfo.LongCommit{
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "foofoofoo",
				Author:  "bar@example.com",
				Subject: "some commit",
			},
		},
	}
	vcs := ingestion.MockVCS(vcsCommits)

	entries := map[string]*Entry{
		"key:8888:android": &Entry{
			Params: map[string]string{
				"config":   "8888",
				"platform": "android",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(0.01),
		},
		"key:gpu:win8": &Entry{
			Params: map[string]string{
				"config":   "gpu",
				"platform": "win8",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(1.234),
		},
	}

	// Populate the tile with some data.
	err := ts.Add(commitIDs[0], entries)
	assert.NoError(t, err)

	// Now test tileBuilder.
	review := rietveld.New(rietveld.RIETVELD_SKIA_URL, httputils.NewTimeoutClient())
	builder := &tileBuilder{
		db:                 ts,
		vcs:                vcs,
		tcache:             lru.New(2),
		rietveldIssueCache: rietveld.NewCodeReviewCache(review, time.Minute, 2),
	}
	tile, err := builder.CachedTileFromCommits(commitIDs)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tile.Commits))
	assert.Equal(t, 2, len(tile.Traces))
	assert.Equal(t, 1, builder.tcache.Len(), "The tile should have been added to the cache.")

	entries = map[string]*Entry{
		"key:565:linux": &Entry{
			Params: map[string]string{
				"config":   "565",
				"platform": "linux",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(0.05),
		},
	}

	// Add more data and be sure that the new data is returned when we
	// call CachedTileFromCommits again.
	err = ts.Add(commitIDs[0], entries)
	assert.NoError(t, err)
	tile, err = builder.CachedTileFromCommits(commitIDs)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tile.Commits))
	assert.Equal(t, 3, len(tile.Traces), "The new data should appear in the tile.")
	assert.Equal(t, 1, builder.tcache.Len(), "The new tile should have replaced the old tile in the cache.")
}
