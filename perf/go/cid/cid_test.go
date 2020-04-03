package cid

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/mocks"
)

func TestCommitID(t *testing.T) {
	unittest.SmallTest(t)
	c := &CommitID{
		Offset: 51,
	}
	assert.Equal(t, "master-000051", c.ID())
}

func TestFromHash(t *testing.T) {
	unittest.SmallTest(t)

	const goodHash = "fe4a4029a080bc955e9588d05a6cd9eb490845d4"
	const badHash = "not-a-valid-hash"

	ctx := context.Background()
	vcs := &mocks.VCS{}
	defer vcs.AssertExpectations(t)

	vcs.On("Details", testutils.AnyContext, goodHash, true).Return(
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    goodHash,
				Subject: "Really big code change",
			},
			Timestamp: time.Now().Add(-time.Second * 10).Round(time.Second),
			Branches:  map[string]bool{"master": true},
		}, nil)
	vcs.On("IndexOf", testutils.AnyContext, goodHash).Return(0, nil)

	commitID, err := FromHash(ctx, vcs, goodHash)
	assert.NoError(t, err)

	expected := &CommitID{
		Offset: 0,
	}
	assert.Equal(t, expected, commitID)

	vcs.On("Details", testutils.AnyContext, badHash, true).Return(nil, errors.New("not found"))

	commitID, err = FromHash(ctx, vcs, badHash)
	assert.Error(t, err)
	assert.Nil(t, commitID)
}

func TestParseLogLine(t *testing.T) {
	unittest.SmallTest(t)
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
			want: "https://skia.googlesource.com/perf-buildid/android-master/+/db4eaa1d0783df0fd4b630ac897c5cbc3c387d10",
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
