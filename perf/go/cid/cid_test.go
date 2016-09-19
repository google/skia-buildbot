package cid

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/vcsinfo"

	"github.com/stretchr/testify/assert"
)

var (
	// Fix the current point as reference. We remove the nano seconds from
	// now (below) because commits are only precise down to seconds.
	now = time.Now()

	// TEST_COMMITS are the commits we are considering. It needs to contain at
	// least all the commits referenced in the test file.
	TEST_COMMITS = []*vcsinfo.LongCommit{
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Round(time.Second),
			Branches:  map[string]bool{"master": true},
		},
	}
)

func TestCommitID(t *testing.T) {
	c := &CommitID{
		Offset: 51,
		Source: "master",
	}
	assert.Equal(t, "master-000001.bdb", c.Filename())

	c = &CommitID{
		Offset: 0,
		Source: "https://codereview.chromium.org/2251213006",
	}
	assert.Equal(t, "https___codereview_chromium_org_2251213006-000000.bdb", c.Filename())
}

// Tests FromIssue.
func TestFromIssue(t *testing.T) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "rietveld_response.txt"))
	assert.NoError(t, err)
	m := mockhttpclient.NewURLMock()
	m.Mock("https://codereview.chromium.org/api/1467533002", mockhttpclient.MockGetDialogue(b))
	m.Mock("https://chromium-cq-status.appspot.com/v2/patch-summary/codereview.chromium.org/2320153002/840001", mockhttpclient.MockGetDialogue([]byte("{}")))

	review := rietveld.New("https://codereview.chromium.org", m.Client())
	commitID, err := FromIssue(review, "1467533002", "40001")
	assert.NoError(t, err)

	expected := &CommitID{
		Source: "https://codereview.chromium.org/1467533002",
		Offset: 2,
	}
	assert.Equal(t, expected, commitID)

	commitID, err = FromIssue(review, "999999999", "40001")
	assert.Error(t, err)
	assert.Nil(t, commitID)
}

func TestFromHash(t *testing.T) {
	vcs := ingestion.MockVCS(TEST_COMMITS)
	commitID, err := FromHash(vcs, "fe4a4029a080bc955e9588d05a6cd9eb490845d4")
	assert.NoError(t, err)

	expected := &CommitID{
		Source: "master",
		Offset: 0,
	}
	assert.Equal(t, expected, commitID)

	commitID, err = FromHash(vcs, "not-a-valid-hash")
	assert.Error(t, err)
	assert.Nil(t, commitID)
}
