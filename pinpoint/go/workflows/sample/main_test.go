package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestParseGerritURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		expected  *pb.GerritChange
		expectErr bool
	}{
		{
			name: "valid Skia Gerrit URL with patchset",
			url:  "https://skia-review.googlesource.com/c/buildbot/+/1235860/3",
			expected: &pb.GerritChange{
				Host:     "skia-review.googlesource.com",
				Project:  "buildbot",
				Change:   1235860,
				Patchset: 3,
			},
			expectErr: false,
		},
		{
			name: "valid Chromium Gerrit URL without patchset",
			url:  "https://chromium-review.googlesource.com/c/chromium/src/+/987654",
			expected: &pb.GerritChange{
				Host:     "chromium-review.googlesource.com",
				Project:  "chromium/src",
				Change:   987654,
				Patchset: 1, // default to 1
			},
			expectErr: false,
		},
		{
			name: "valid http Gerrit URL",
			url:  "http://chrome-internal-review.googlesource.com/c/chrome/src/+/12345/2",
			expected: &pb.GerritChange{
				Host:     "chrome-internal-review.googlesource.com",
				Project:  "chrome/src",
				Change:   12345,
				Patchset: 2,
			},
			expectErr: false,
		},
		{
			name:      "invalid URL missing scheme",
			url:       "skia-review.googlesource.com/c/buildbot/+/1235860",
			expected:  nil,
			expectErr: true,
		},
		{
			name:      "invalid URL missing /c/",
			url:       "https://skia-review.googlesource.com/buildbot/+/1235860",
			expected:  nil,
			expectErr: true,
		},
		{
			name:      "invalid URL missing /+/",
			url:       "https://skia-review.googlesource.com/c/buildbot/1235860",
			expected:  nil,
			expectErr: true,
		},
		{
			name:      "invalid URL missing change ID",
			url:       "https://skia-review.googlesource.com/c/buildbot/+/",
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseGerritURL(tc.url)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, actual)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			}
		})
	}
}
