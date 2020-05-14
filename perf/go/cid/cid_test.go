package cid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	perfsql "go.skia.org/infra/perf/go/sql"
)

func TestCommitID(t *testing.T) {
	unittest.SmallTest(t)
	c := &CommitID{
		Offset: 51,
	}
	assert.Equal(t, "master-000051", c.ID())
}

func TestFromID(t *testing.T) {
	unittest.SmallTest(t)
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

func Test_urlFromParts(t *testing.T) {
	unittest.SmallTest(t)
	type args struct {
		repoURL  string
		hash     string
		subject  string
		debounce bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no bounce",
			args: args{
				repoURL:  "https://skia.googlesource.com/perf-buildid/android-master",
				hash:     "db4eaa1d0783df0fd4b630ac897c5cbc3c387d10",
				subject:  "https://android-master-ingest.skia.org/r/6146906?branch=aosp-androidx-master-dev",
				debounce: false,
			},
			want: "https://skia.googlesource.com/perf-buildid/android-master/+show/db4eaa1d0783df0fd4b630ac897c5cbc3c387d10",
		},
		{
			name: "bounce",
			args: args{
				repoURL:  "https://skia.googlesource.com/perf-buildid/android-master",
				hash:     "db4eaa1d0783df0fd4b630ac897c5cbc3c387d10",
				subject:  "https://android-master-ingest.skia.org/r/6146906?branch=aosp-androidx-master-dev",
				debounce: true,
			},
			want: "https://android-master-ingest.skia.org/r/6146906?branch=aosp-androidx-master-dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := urlFromParts(tt.args.repoURL, tt.args.hash, tt.args.subject, tt.args.debounce); got != tt.want {
				t.Errorf("urlFromParts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommitIDLookup_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, hashes, dialect, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.SQLiteDialect)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, dialect, instanceConfig)
	require.NoError(t, err)
	commitIdLookup := New(ctx, g, instanceConfig)

	details, err := commitIdLookup.Lookup(ctx, []*CommitID{
		{Offset: 0},
		{Offset: 2},
		{Offset: 5},
	})
	require.NoError(t, err)
	assert.Equal(t, hashes[0], details[0].Hash)
	assert.Equal(t, hashes[2], details[1].Hash)
	assert.Equal(t, hashes[5], details[2].Hash)

	// Message contains info relative to the current time, so we'll test it
	// individually.
	assert.Contains(t, details[0].Message, hashes[0][:7])

	// URL contains info relative to the tmp dir used to create the test git
	// repo, so we'll test it individuall.
	assert.Contains(t, details[0].URL, hashes[0][:7])

	// Then clear it to test the rest of the struct.
	details[0].Message = ""
	details[0].URL = ""
	assert.Equal(t, &CommitDetail{
		CommitID:  CommitID{Offset: 0},
		Author:    "test <test@google.com>",
		Message:   "",
		URL:       "",
		Hash:      hashes[0],
		Timestamp: gittest.StartTime.Unix(),
	}, details[0])
}

func TestCommitIDLookup_ErrOnBadCommit(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, _, dialect, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.SQLiteDialect)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, dialect, instanceConfig)
	require.NoError(t, err)
	commitIdLookup := New(ctx, g, instanceConfig)

	_, err = commitIdLookup.Lookup(ctx, []*CommitID{
		{Offset: -1},
	})
	require.Error(t, err)
}
