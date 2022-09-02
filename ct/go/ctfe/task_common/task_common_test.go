package task_common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGerritURLRegexp(t *testing.T) {

	tests := []struct {
		cl                string
		expectedProject   string
		expectedChangeNum string
		expectedPatchNum  string
	}{
		{cl: "https://chromium-review.googlesource.com/c/1649339", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: ""},
		{cl: "https://chromium-review.googlesource.com/c/1649339/", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: ""},
		{cl: "https://chromium-review.googlesource.com/c/1649339/4", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: "4"},

		{cl: "https://chromium-review.googlesource.com/#/c/1649339", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: ""},

		{cl: "https://chromium-review.googlesource.com/c/chromium/src/+/1649339", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: ""},
		{cl: "https://chromium-review.googlesource.com/c/chromium/src/+/1649339/", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: ""},
		{cl: "https://chromium-review.googlesource.com/c/chromium/src/+/1649339/4", expectedProject: "https://chromium-review.googlesource.com", expectedChangeNum: "1649339", expectedPatchNum: "4"},
	}

	for _, test := range tests {
		matches := gerritURLRegexp.FindStringSubmatch(test.cl)
		require.Equal(t, test.expectedProject, matches[1])
		require.Equal(t, test.expectedChangeNum, matches[2])
		require.Equal(t, test.expectedPatchNum, matches[3])
	}
}

func TestGatherCLData(t *testing.T) {

	detail := clDetail{
		Project:  "chromium",
		Modified: "2022-01-02 15:04:05.12",
	}
	patch := "xyz"

	clData, err := gatherCLData(detail, patch)
	require.NoError(t, err)
	require.Equal(t, patch, clData.ChromiumPatch)
	// The rest of the project patches should be empty.
	require.Equal(t, "", clData.SkiaPatch)
	require.Equal(t, "", clData.V8Patch)
	require.Equal(t, "", clData.CatapultPatch)
}
