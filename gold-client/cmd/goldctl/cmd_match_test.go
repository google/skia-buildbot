package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/imgmatching"
)

func TestMatch_Fuzzy_ImagesAreWithinTolerance_ExitCodeZero(t *testing.T) {
	unittest.MediumTest(t)

	td := testutils.TestDataDir(t)

	// Call imgtest match using the fuzzy match algorithm
	ctx, output, exit := testContext(nil, nil, nil, nil)
	env := matchEnv{
		algorithmName: "fuzzy",
		parameters: []string{
			string(imgmatching.MaxDifferentPixels + ":2"),
			string(imgmatching.PixelDeltaThreshold + ":10"),
		},
	}
	runUntilExit(t, func() {
		env.Match(ctx, filepath.Join(td, a01Digest+".png"), filepath.Join(td, a05Digest+".png"))
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, output.String())

	// Output should have numbers be right aligned.
	assert.Equal(t, `Images match.
          Number of different pixels: 2
       Maximum per-channel delta sum: 7
`, logs)
}

func TestMatch_Sobel_ImagesAreVeryDifferent_ExitCodeZero(t *testing.T) {
	unittest.MediumTest(t)

	td := testutils.TestDataDir(t)

	// Call imgtest match using the fuzzy match algorithm
	ctx, output, exit := testContext(nil, nil, nil, nil)
	env := matchEnv{
		algorithmName: "sobel",
		parameters: []string{
			string(imgmatching.MaxDifferentPixels + ":2"),
			string(imgmatching.PixelDeltaThreshold + ":10"),
			string(imgmatching.EdgeThreshold + ":2"),
		},
	}
	runUntilExit(t, func() {
		env.Match(ctx, filepath.Join(td, a01Digest+".png"), filepath.Join(td, a09Digest+".png"))
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, output.String())

	assert.Contains(t, logs, `Images do not match.`, logs)
	assert.Contains(t, logs, `Number of different pixels: 34`, logs)
	assert.Contains(t, logs, `Maximum per-channel delta sum: 1020`, logs)
}
