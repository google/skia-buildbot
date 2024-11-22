// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package try

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs"
	vfs_mocks "go.skia.org/infra/go/vfs/mocks"
)

func mockGitilesVFS(fs vfs.FS) func(ctx context.Context, repo gitiles.GitilesRepo, ref string) (vfs.FS, error) {
	return func(ctx context.Context, repo gitiles.GitilesRepo, ref string) (vfs.FS, error) {
		return fs, nil
	}
}

func TestMergeReleaseNotes_WithValidNotes_MergesNotes(t *testing.T) {

	const (
		currentMilestoneReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 113
-------------
  * First item
  * Second item
  * Third item
`
		newMilestoneReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 114
-------------
  * The first change.
  * The second change.

* * *

Milestone 113
-------------
  * First item
  * Second item
  * Third item
`
		commitMessage = "Merge 2 release notes into RELEASE_NOTES.md"
	)

	firstNote := []byte("The first change.")
	secondNote := []byte("The second change.")
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "README.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "first.md",
			Size:    int64(len(firstNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "second.md",
			Size:    int64(len(secondNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
	}

	mockCmd := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockCmd.Run)

	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Project:  "skia",
		Branch:   "chrome/m114",
		Id:       "I1234567890123456789012345678901234567890",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}

	fs := vfs_mocks.NewFS(t)
	dir := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/first.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, firstNote)
	}).Return(len(firstNote), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	f2 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/second.md").Once().Return(f2, nil)
	f2.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, secondNote)
	}).Return(len(secondNote), io.EOF)
	f2.On("Close", testutils.AnyContext).Return(nil)

	f3 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f3, nil)
	f3.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, currentMilestoneReleaseNotes)
	}).Return(len(currentMilestoneReleaseNotes), io.EOF)
	f3.On("Close", testutils.AnyContext).Return(nil)

	repo := mocks.NewGitilesRepo(t)
	repo.On("ResolveRef", testutils.AnyContext, "chrome/m114").
		Once().Return("7ecb228be2abc108caf2096b518fa36ef418be11", nil)
	newGitilesVFS = mockGitilesVFS(fs)

	g := gerrit_mocks.NewGerritInterface(t)
	g.On("CreateChange", ctx, "skia", "refs/heads/chrome/m114", commitMessage,
		"", "Icc898ef6bb4eeb8e93fa8c5d1195364d55ca2a4c").Once().Return(&ci, nil)
	g.On("EditFile", ctx, mock.Anything, "RELEASE_NOTES.md",
		newMilestoneReleaseNotes).Once().Return(nil)
	g.On("DeleteFile", ctx, mock.Anything, "relnotes/first.md").Once().Return(nil)
	g.On("DeleteFile", ctx, mock.Anything, "relnotes/second.md").Once().Return(nil)
	g.On("PublishChangeEdit", ctx, mock.Anything).Once().Return(nil)
	g.On("GetIssueProperties", ctx, int64(123)).Once().Return(&ci, nil)
	g.On("Url", int64(123)).Once().Return("https://skia-review.googlesource.com/c/skia/+/123")

	mci, err := mergeReleaseNotes(ctx, g, repo, "Icc898ef6bb4eeb8e93fa8c5d1195364d55ca2a4c", 114, "chrome/m114", nil)
	require.NoError(t, err)
	require.NotNil(t, mci)
}

func TestMergeReleaseNotes_WithOneNote_SimgularMessageLanguage(t *testing.T) {

	const (
		currentMilestoneReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 113
-------------
  * First item
  * Second item
  * Third item
`
		newMilestoneReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 114
-------------
  * The first change.

* * *

Milestone 113
-------------
  * First item
  * Second item
  * Third item
`
		commitMessage = "Merge 1 release note into RELEASE_NOTES.md"
	)

	firstNote := []byte("The first change.")
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "README.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "first.md",
			Size:    int64(len(firstNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
	}

	mockCmd := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockCmd.Run)

	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Project:  "skia",
		Branch:   "chrome/m114",
		Id:       "I1234567890123456789012345678901234567890",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}

	fs := vfs_mocks.NewFS(t)
	dir := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/first.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, firstNote)
	}).Return(len(firstNote), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	f2 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f2, nil)
	f2.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, currentMilestoneReleaseNotes)
	}).Return(len(currentMilestoneReleaseNotes), io.EOF)
	f2.On("Close", testutils.AnyContext).Return(nil)

	repo := mocks.NewGitilesRepo(t)
	repo.On("ResolveRef", testutils.AnyContext, "chrome/m114").
		Once().Return("7ecb228be2abc108caf2096b518fa36ef418be11", nil)
	newGitilesVFS = mockGitilesVFS(fs)

	g := gerrit_mocks.NewGerritInterface(t)
	g.On("CreateChange", ctx, "skia", "refs/heads/chrome/m114", commitMessage,
		"", "Icc898ef6bb4eeb8e93fa8c5d1195364d55ca2a4c").Once().Return(&ci, nil)
	g.On("EditFile", ctx, mock.Anything, "RELEASE_NOTES.md",
		newMilestoneReleaseNotes).Once().Return(nil)
	g.On("DeleteFile", ctx, mock.Anything, "relnotes/first.md").Once().Return(nil)
	g.On("PublishChangeEdit", ctx, mock.Anything).Once().Return(nil)
	g.On("GetIssueProperties", ctx, int64(123)).Once().Return(&ci, nil)
	g.On("Url", int64(123)).Once().Return("https://skia-review.googlesource.com/c/skia/+/123")

	mci, err := mergeReleaseNotes(ctx, g, repo, "Icc898ef6bb4eeb8e93fa8c5d1195364d55ca2a4c", 114, "chrome/m114", nil)
	require.NoError(t, err)
	require.NotNil(t, mci)
}

func TestCreateCherryPickMessage(t *testing.T) {
	ci := gerrit.ChangeInfo{
		ChangeId: "I1234567890123456789012345678901234567890",
		Project:  "skia",
		Branch:   "chrome/m114",
		Subject: `Merge 2 release notes into RELEASE_NOTES.md

Change-Id: I1234567890123456789012345678901234567890`,
		Id:    "myProject~chrome/m114~I1234567890123456789012345678901234567890",
		Issue: 123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}

	const expectedMsg = `Merge 2 release notes into RELEASE_NOTES.md

Change-Id: I1234567890123456789012345678901234567890

Cherry pick change I1234567890123456789012345678901234567890 from branch chrome/m114
to main.
`

	msg := createCherryPickMessage(&ci, git_common.MainBranch)
	require.Equal(t, expectedMsg, msg)
}

func TestCherryPickChangeToBranch(t *testing.T) {

	mockCmd := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockCmd.Run)

	mergeCI := gerrit.ChangeInfo{
		ChangeId: "I1234567890123456789012345678901234567890",
		Project:  "skia",
		Branch:   "chrome/m114",
		Id:       "myProject~chrome/m114~I1234567890123456789012345678901234567890",
		Issue:    123,
		Subject:  "Relnotes merge subject.",
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}

	cherryPickCI := gerrit.ChangeInfo{
		ChangeId: "I0987654321098765432109876543210987654321",
		Project:  "skia",
		Branch:   "test-branch",
		Id:       "myProject~test-branch~I0987654321098765432109876543210987654321",
		Issue:    124,
		Subject: `Relnotes merge subject.

Cherry pick change I1234567890123456789012345678901234567890 from branch chrome/m114
to test-branch.
`,
		WorkInProgress: true,
	}

	cherryPickGetCI := gerrit.ChangeInfo{
		ChangeId: "I0987654321098765432109876543210987654321",
		Project:  "skia",
		Branch:   "test-branch",
		Id:       "myProject~test-branch~I0987654321098765432109876543210987654321",
		Issue:    124,
		Subject: `Relnotes merge subject.

Cherry pick change I1234567890123456789012345678901234567890 from branch chrome/m114
to test-branch.
`,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}

	g := gerrit_mocks.NewGerritInterface(t)
	g.On("CreateCherryPickChange",
		testutils.AnyContext,
		"myProject~chrome/m114~I1234567890123456789012345678901234567890",
		"current",
		cherryPickCI.Subject,
		"test-branch").Once().Return(&cherryPickCI, nil)
	g.On("Url", int64(124)).Once().Return("https://skia-review.googlesource.com/c/skia/+/124")
	g.On("SetReview", testutils.AnyContext, &cherryPickGetCI, "", map[string]int(nil),
		[]string{"reviewer@"}, gerrit.NotifyOption(""), gerrit.NotifyDetails(nil), "", 0,
		[]*gerrit.AttentionSetInput(nil)).Once().Return(nil)
	g.On("GetChange", testutils.AnyContext, "myProject~test-branch~I0987654321098765432109876543210987654321").
		Once().Return(&cherryPickGetCI, nil)

	err := cherryPickChangeToBranch(ctx, g, &mergeCI, "test-branch", []string{"reviewer@"})
	require.NoError(t, err)
}

func TestUpdateJobsJSON(t *testing.T) {
	oldContents := []byte(`[
  {"name":  "Build-Debian10-EMCC-asmjs-Release-PathKit"},
  {"name":    "Build-Debian10-EMCC-wasm-Debug-CanvasKit" },
    {"name": "Build-Debian10-EMCC-wasm-Debug-CanvasKit_WebGPU",

    "cq_config":   {  }
	},
  {"name": "Build-Debian10-EMCC-wasm-Debug-CanvasKit_CPU"},
  {"name": "Build-Debian10-EMCC-wasm-Debug-PathKit",
  "cq_config": {
	"location_regexes": [
	  "modules/canvaskit/.*"
	]
  }}
]`)
	expectNewContents := []byte(`[
  {"name": "Build-Debian10-EMCC-asmjs-Release-PathKit"},
  {"name": "Build-Debian10-EMCC-wasm-Debug-CanvasKit"},
  {"name": "Build-Debian10-EMCC-wasm-Debug-CanvasKit_WebGPU"},
  {"name": "Build-Debian10-EMCC-wasm-Debug-CanvasKit_CPU"},
  {"name": "Build-Debian10-EMCC-wasm-Debug-PathKit"}
]`)
	newContents, err := updateJobsJSON(oldContents)
	require.NoError(t, err)
	require.Equal(t, string(expectNewContents), string(newContents))
}
