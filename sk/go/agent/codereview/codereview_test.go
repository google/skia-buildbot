// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package codereview

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gerrit"
)

func TestParseIssueArg(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		wantURL   string
		wantIssue int64
		wantErr   bool
	}{
		{
			name:      "numeric issue ID",
			arg:       "1394436",
			wantURL:   "https://skia-review.googlesource.com",
			wantIssue: 1394436,
		},
		{
			name:      "standard Gerrit URL",
			arg:       "https://skia-review.googlesource.com/c/skia/+/1394436",
			wantURL:   "https://skia-review.googlesource.com",
			wantIssue: 1394436,
		},
		{
			name:      "Gerrit URL with patchset",
			arg:       "https://skia-review.googlesource.com/c/skia/+/1394436/3",
			wantURL:   "https://skia-review.googlesource.com",
			wantIssue: 1394436,
		},
		{
			name:      "short Gerrit URL",
			arg:       "https://skia-review.googlesource.com/1394436",
			wantURL:   "https://skia-review.googlesource.com",
			wantIssue: 1394436,
		},
		{
			name:      "corp redirected Gerrit URL",
			arg:       "https://skia-review.git.corp.google.com/c/skia/+/1394436",
			wantURL:   "https://skia-review.googlesource.com",
			wantIssue: 1394436,
		},
		{
			name:    "invalid argument",
			arg:     "not-an-issue",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotIssue, err := parseIssueArg(tc.arg, gerrit.GerritSkiaURL)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantURL, gotURL)
			require.Equal(t, tc.wantIssue, gotIssue)
		})
	}
}

func TestResolveComments_DropsMidLineCutoff(t *testing.T) {
	content := "#include \"include/core/SkCubicMap.h\"\n#include <cmath>\n#include <algorithm>\n#include \"src/base/SkCubics.h\"\n"
	sf := newSourceFile("src/core/SkCubicMap.cpp", content, map[int]bool{
		1: true,
		2: true,
		3: true,
		4: true,
	})
	files := map[string]*sourceFile{
		"src/core/SkCubicMap.cpp": sf,
	}

	raw := []*modelComment{
		{
			Path:      "src/core/SkCubicMap.cpp",
			StartLine: 2,
			EndLine:   4,
			Message:   "Sort includes alphabetically.",
			Replacements: []modelReplacement{
				{
					StartLine: 2,
					OldText:   "#include <cmath>\n#include <algorithm>\n#include ",
					NewText:   "#include \"src/base/SkCubics.h\"\n\n#include <algorithm>\n#include <cmath>\n",
				},
			},
		},
	}

	got := resolveComments(raw, files)
	require.Len(t, got, 1)
	require.Empty(t, got[0].SuggestedFix)
	require.Equal(t, gerrit.CommentRange{
		StartLine:      2,
		StartCharacter: 0,
		EndLine:        4,
		EndCharacter:   len("#include \"src/base/SkCubics.h\""),
	}, got[0].Range)
}
