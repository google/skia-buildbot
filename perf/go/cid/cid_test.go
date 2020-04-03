package cid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
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
