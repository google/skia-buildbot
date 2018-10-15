package cid

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	// TEST_COMMITS are the commits we are considering. It needs to contain at
	// least all the commits referenced in the test file.
	TEST_COMMITS = []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
				Subject: "Really big code change",
			},
			Timestamp: time.Now().Add(-time.Second * 10).Round(time.Second),
			Branches:  map[string]bool{"master": true},
		},
	}
)

func TestCommitID(t *testing.T) {
	testutils.SmallTest(t)
	c := &CommitID{
		Offset: 51,
		Source: "master",
	}
	assert.Equal(t, "master-000001.bdb", c.Filename())
	assert.Equal(t, "master-000051", c.ID())
}

func TestFromHash(t *testing.T) {
	testutils.SmallTest(t)
	ctx := context.Background()
	vcs := ingestion.MockVCS(TEST_COMMITS, nil, nil)
	commitID, err := FromHash(ctx, vcs, "fe4a4029a080bc955e9588d05a6cd9eb490845d4")
	assert.NoError(t, err)

	expected := &CommitID{
		Source: "master",
		Offset: 0,
	}
	assert.Equal(t, expected, commitID)

	commitID, err = FromHash(ctx, vcs, "not-a-valid-hash")
	assert.Error(t, err)
	assert.Nil(t, commitID)
}

func TestParseLogLine(t *testing.T) {
	testutils.SmallTest(t)
	ctx := context.Background()
	s := "1476870603 e8f0a7b986f1e5583c9bc162efcdd92fd6430549 joel.liang@arm.com Generate Signed Distance Field directly from vector path"
	var index int = 3
	entry, err := parseLogLine(ctx, s, &index, nil)
	assert.NoError(t, err)
	expected := &cacheEntry{
		author:  "joel.liang@arm.com",
		subject: "Generate Signed Distance Field directly from vector path",
		hash:    "e8f0a7b986f1e5583c9bc162efcdd92fd6430549",
		ts:      1476870603,
	}
	assert.Equal(t, expected, entry)
	assert.Equal(t, 4, index)

	// No subject.
	s = "1476870603 e8f0a7b986f1e5583c9bc162efcdd92fd6430549 joel.liang@arm.com"
	entry, err = parseLogLine(ctx, s, &index, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to parse parts")
	assert.Equal(t, 4, index)

	// Invalid timestamp.
	s = "1476870ZZZ e8f0a7b986f1e5583c9bc162efcdd92fd6430549 joel.liang@arm.com Generate Signed Distance Field directly from vector path"
	entry, err = parseLogLine(ctx, s, &index, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Can't parse timestamp")
	assert.Equal(t, 4, index)
}

func TestFromID(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		value    string
		expected *CommitID
		err      bool
		message  string
	}{
		{
			value: "master-000051",
			expected: &CommitID{
				Offset: 51,
				Source: "master",
			},
			err:     false,
			message: "Simple",
		},
		{
			value:    "some_trybot-000051",
			expected: nil,
			err:      true,
			message:  "TryBot should fail",
		},
		{
			value:    "master-notanint",
			expected: nil,
			err:      true,
			message:  "Fail parse int",
		},
		{
			value:    "invalid",
			expected: nil,
			err:      true,
			message:  "no dashes",
		},
		{
			value:    "in-val-id",
			expected: nil,
			err:      true,
			message:  "too many dashes",
		},
	}

	for _, tc := range testCases {
		got, err := FromID(tc.value)
		assert.Equal(t, tc.err, err != nil, tc.message)
		assert.Equal(t, tc.expected, got, tc.message)
	}

}
