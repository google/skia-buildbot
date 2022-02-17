package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/codesize/go/common"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestStore_Index_InvalidJSONMetadataFile_Error(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()
	err := store.Index(context.Background(), "kaboom.tsv")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character '{'")
}

func TestStore_Index_CalledTwiceWithSameFile_FileIndexedOnce(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()

	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/dm.tsv"))
	assert.Len(t, store.GetMostRecentBinaries(10), 1)
	assert.Len(t, store.GetMostRecentBinaries(10)[0].Binaries, 1)

	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/fm.tsv"))
	assert.Len(t, store.GetMostRecentBinaries(10), 1)
	assert.Len(t, store.GetMostRecentBinaries(10)[0].Binaries, 2)

	// Same file as previous call.
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/fm.tsv"))
	assert.Len(t, store.GetMostRecentBinaries(10), 1)
	assert.Len(t, store.GetMostRecentBinaries(10)[0].Binaries, 2)

	require.NoError(t, store.Index(context.Background(), "commit2/Build-Foo/dm.tsv"))
	assert.Len(t, store.GetMostRecentBinaries(10), 2)
	assert.Len(t, store.GetMostRecentBinaries(10)[0].Binaries, 1)
	assert.Len(t, store.GetMostRecentBinaries(10)[1].Binaries, 2)

	// Same file as previous call.
	require.NoError(t, store.Index(context.Background(), "commit2/Build-Foo/dm.tsv"))
	assert.Len(t, store.GetMostRecentBinaries(10), 2)
	assert.Len(t, store.GetMostRecentBinaries(10)[0].Binaries, 1)
	assert.Len(t, store.GetMostRecentBinaries(10)[1].Binaries, 2)
}

func TestStore_IndexThenGetMostRecentBinaries_Success(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()

	// Store is initially empty.
	assert.Empty(t, store.GetMostRecentBinaries(10))

	// Index a bunch of binaries.
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/fm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Bar/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit2/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "issue9876/patchset3/job1/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "issue9876/patchset3/job2/Build-Bar/fm.tsv"))

	// Retrieve all binaries from the store.
	assert.Equal(t, []BinariesFromCommitOrPatchset{
		{
			CommitOrPatchset: CommitOrPatchset{
				Commit: "commit2",
			},
			Binaries: []Binary{
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T16:34:48Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "dm",
						Revision:        "commit2",
					},
					BloatyOutputFileGCSPath: "commit2/Build-Foo/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 16, 34, 48, 0, time.UTC),
				},
			},
		},
		{
			CommitOrPatchset: CommitOrPatchset{
				Commit: "commit1",
			},
			Binaries: []Binary{
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T15:23:19Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "dm",
						Revision:        "commit1",
					},
					BloatyOutputFileGCSPath: "commit1/Build-Foo/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 15, 23, 19, 0, time.UTC),
				},
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T15:22:50Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "fm",
						Revision:        "commit1",
					},
					BloatyOutputFileGCSPath: "commit1/Build-Foo/fm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 15, 22, 50, 0, time.UTC),
				},
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T15:21:53Z",
						CompileTaskName: "Build-Bar",
						BinaryName:      "dm",
						Revision:        "commit1",
					},
					BloatyOutputFileGCSPath: "commit1/Build-Bar/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 15, 21, 53, 0, time.UTC),
				},
			},
		},
		{
			CommitOrPatchset: CommitOrPatchset{
				PatchIssue: "509137",
				PatchSet:   "11",
			},
			Binaries: []Binary{
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T14:34:25Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "dm",
						PatchIssue:      "509137",
						PatchSet:        "11",
						Revision:        "commit3",
					},
					BloatyOutputFileGCSPath: "issue9876/patchset3/job1/Build-Foo/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 14, 34, 25, 0, time.UTC),
				},
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T14:26:52Z",
						CompileTaskName: "Build-Bar",
						BinaryName:      "fm",
						PatchIssue:      "509137",
						PatchSet:        "11",
						Revision:        "commit3",
					},
					BloatyOutputFileGCSPath: "issue9876/patchset3/job2/Build-Bar/fm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 14, 26, 52, 0, time.UTC),
				},
			},
		},
	},
		store.GetMostRecentBinaries(10))

	// Retrieve only the two most recent results.
	assert.Equal(t, []BinariesFromCommitOrPatchset{
		{
			CommitOrPatchset: CommitOrPatchset{
				Commit: "commit2",
			},
			Binaries: []Binary{
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T16:34:48Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "dm",
						Revision:        "commit2",
					},
					BloatyOutputFileGCSPath: "commit2/Build-Foo/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 16, 34, 48, 0, time.UTC),
				},
			},
		},
		{
			CommitOrPatchset: CommitOrPatchset{
				Commit: "commit1",
			},
			Binaries: []Binary{
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T15:23:19Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "dm",
						Revision:        "commit1",
					},
					BloatyOutputFileGCSPath: "commit1/Build-Foo/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 15, 23, 19, 0, time.UTC),
				},
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T15:22:50Z",
						CompileTaskName: "Build-Foo",
						BinaryName:      "fm",
						Revision:        "commit1",
					},
					BloatyOutputFileGCSPath: "commit1/Build-Foo/fm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 15, 22, 50, 0, time.UTC),
				},
				{
					Metadata: common.BloatyOutputMetadata{
						Version:         1,
						Timestamp:       "2022-02-16T15:21:53Z",
						CompileTaskName: "Build-Bar",
						BinaryName:      "dm",
						Revision:        "commit1",
					},
					BloatyOutputFileGCSPath: "commit1/Build-Bar/dm.tsv",
					Timestamp:               time.Date(2022, time.February, 16, 15, 21, 53, 0, time.UTC),
				},
			},
		},
	},
		store.GetMostRecentBinaries(2))
}

func TestStore_GetBinary_NonExistentBinary_Error(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()

	assert.Empty(t, store.GetMostRecentBinaries(10))
	_, ok := store.GetBinary(CommitOrPatchset{Commit: "no such commit"}, "dm", "Build-Foo")
	assert.False(t, ok)
}

func TestStore_IndexThenGetBinary_Success(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()

	// Store is initially empty.
	assert.Empty(t, store.GetMostRecentBinaries(10))

	// Index a bunch of binaries.
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/fm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Bar/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit2/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "issue9876/patchset3/job1/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "issue9876/patchset3/job2/Build-Bar/fm.tsv"))

	// Get a binary from a commit.
	binary, ok := store.GetBinary(
		CommitOrPatchset{Commit: "commit1"},
		"dm",
		"Build-Foo")
	require.True(t, ok)
	assert.Equal(t, Binary{
		Metadata: common.BloatyOutputMetadata{
			Version:         1,
			Timestamp:       "2022-02-16T15:23:19Z",
			CompileTaskName: "Build-Foo",
			BinaryName:      "dm",
			Revision:        "commit1",
		},
		BloatyOutputFileGCSPath: "commit1/Build-Foo/dm.tsv",
		Timestamp:               time.Date(2022, time.February, 16, 15, 23, 19, 0, time.UTC),
	},
		binary)

	// Get a binary from a tryjob.
	binary, ok = store.GetBinary(
		CommitOrPatchset{PatchIssue: "509137", PatchSet: "11"},
		"fm",
		"Build-Bar")
	require.True(t, ok)
	assert.Equal(t, Binary{
		Metadata: common.BloatyOutputMetadata{
			Version:         1,
			Timestamp:       "2022-02-16T14:26:52Z",
			CompileTaskName: "Build-Bar",
			BinaryName:      "fm",
			PatchIssue:      "509137",
			PatchSet:        "11",
			Revision:        "commit3",
		},
		BloatyOutputFileGCSPath: "issue9876/patchset3/job2/Build-Bar/fm.tsv",
		Timestamp:               time.Date(2022, time.February, 16, 14, 26, 52, 0, time.UTC),
	},
		binary)
}

func TestStore_GetBloatyOutputFileContents_NonExistentBinary_Error(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()

	_, err := store.GetBloatyOutputFileContents(context.Background(), Binary{
		BloatyOutputFileGCSPath: "no-such-file.tsv",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found: no-such-file.tsv")
}

func TestStore_IndexThenGetBloatyOutputFileContents_Success(t *testing.T) {
	unittest.SmallTest(t)
	store := newStoreForTesting()

	// Store is initially empty.
	assert.Empty(t, store.GetMostRecentBinaries(10))

	// Index a bunch of binaries.
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Foo/fm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit1/Build-Bar/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "commit2/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "issue9876/patchset3/job1/Build-Foo/dm.tsv"))
	require.NoError(t, store.Index(context.Background(), "issue9876/patchset3/job2/Build-Bar/fm.tsv"))

	// Binary from a commit.
	bytes, err := store.GetBloatyOutputFileContents(context.Background(), Binary{
		Metadata: common.BloatyOutputMetadata{
			Version:         1,
			Timestamp:       "2022-02-16T15:23:19Z",
			CompileTaskName: "Build-Foo",
			BinaryName:      "dm",
			Revision:        "commit1",
		},
		BloatyOutputFileGCSPath: "commit1/Build-Foo/dm.tsv",
		Timestamp:               time.Date(2022, time.February, 16, 15, 23, 19, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Equal(t, "Fake Bloaty output 1", string(bytes))

	// Binary from a tryjob.
	bytes, err = store.GetBloatyOutputFileContents(context.Background(), Binary{
		Metadata: common.BloatyOutputMetadata{
			Version:         1,
			Timestamp:       "2022-02-16T14:26:52Z",
			CompileTaskName: "Build-Bar",
			BinaryName:      "fm",
			PatchIssue:      "509137",
			PatchSet:        "11",
			Revision:        "commit3",
		},
		BloatyOutputFileGCSPath: "issue9876/patchset3/job2/Build-Bar/fm.tsv",
		Timestamp:               time.Date(2022, time.February, 16, 14, 26, 52, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Equal(t, "Fake Bloaty output 6", string(bytes))
}

// newStoreForTesting returns a Store that "downloads" files from the fake GCS bucket defined by
// the fakeGCSBucket map below.
func newStoreForTesting() Store {
	return New(func(ctx context.Context, path string) ([]byte, error) {
		contents, ok := fakeGCSBucket[path]
		if !ok {
			return nil, fmt.Errorf("not found: %s", path)
		}
		return []byte(contents), nil
	})
}

// The JSON metadata files only contain those fields relevant to indexing. The particular filenames
// do not matter.
var fakeGCSBucket = map[string]string{
	// A binary from a commit (i.e. not a tryjob).
	"commit1/Build-Foo/dm.json": `
		{
			"version": 1,
			"timestamp": "2022-02-16T15:23:19Z",
			"compile_task_name": "Build-Foo",
			"binary_name": "dm",
			"patch_issue": "",
			"patch_set": "",
			"revision": "commit1"
		}
	`,
	"commit1/Build-Foo/dm.tsv": "Fake Bloaty output 1",

	// Same commit and compile task as previous file, different binary.
	"commit1/Build-Foo/fm.json": `
		{
			"version": 1,
			"timestamp": "2022-02-16T15:22:50Z",
			"compile_task_name": "Build-Foo",
			"binary_name": "fm",
			"patch_issue": "",
			"patch_set": "",
			"revision": "commit1"
		}
	`,
	"commit1/Build-Foo/fm.tsv": "Fake Bloaty output 2",

	// Same commit as previous file, different compile task.
	"commit1/Build-Bar/dm.json": `
		{
			"version": 1,
			"timestamp": "2022-02-16T15:21:53Z",
			"compile_task_name": "Build-Bar",
			"binary_name": "dm",
			"patch_issue": "",
			"patch_set": "",
			"revision": "commit1"
		}
	`,
	"commit1/Build-Bar/dm.tsv": "Fake Bloaty output 3",

	// New commit.
	"commit2/Build-Foo/dm.json": `
		{
			"version": 1,
			"timestamp": "2022-02-16T16:34:48Z",
			"compile_task_name": "Build-Foo",
			"binary_name": "dm",
			"patch_issue": "",
			"patch_set": "",
			"revision": "commit2"
		}
	`,
	"commit2/Build-Foo/dm.tsv": "Fake Bloaty output 4",

	// A binary from a tryjob.
	"issue9876/patchset3/job1/Build-Foo/dm.json": `
		{
			"version": 1,
			"timestamp": "2022-02-16T14:34:25Z",
			"compile_task_name": "Build-Foo",
			"binary_name": "dm",
			"patch_issue": "509137",
			"patch_set": "11",
			"revision": "commit3"
		}
	`,
	"issue9876/patchset3/job1/Build-Foo/dm.tsv": "Fake Bloaty output 5",

	// Same tryjob as previous file, different compile task and binary.
	"issue9876/patchset3/job2/Build-Bar/fm.json": `
		{
			"version": 1,
			"timestamp": "2022-02-16T14:26:52Z",
			"compile_task_name": "Build-Bar",
			"binary_name": "fm",
			"patch_issue": "509137",
			"patch_set": "11",
			"revision": "commit3"
		}
	`,
	"issue9876/patchset3/job2/Build-Bar/fm.tsv": "Fake Bloaty output 6",

	"kaboom.json": `{{{{{"malformed JSON file": "kaboom"}`,
	"kaboom.tsv":  "Invalid file.",
}
