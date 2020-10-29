package isolate

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/common/isolated"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCopyIsolatedFile(t *testing.T) {
	unittest.SmallTest(t)

	link := "link"
	mode := 777
	size := int64(9000)
	iso := &isolated.Isolated{
		Algo: "smrt",
		Files: map[string]isolated.File{
			"my-file": {
				Digest: "abc123",
				Link:   &link,
				Mode:   &mode,
				Size:   &size,
				Type:   isolated.Basic,
			},
		},
		Includes: []isolated.HexDigest{"blah"},
		Version:  "NEW!",
	}
	assertdeep.Copy(t, iso, CopyIsolated(iso))

	iso.Files["my-file"] = isolated.File{}
	cp := CopyIsolated(iso)
	assertdeep.Equal(t, iso, cp)
}

func TestIsolateTasks(t *testing.T) {
	unittest.LargeTest(t)

	// Setup.
	workdir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)

	c, err := NewClient(workdir, ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)

	ctx := context.Background()
	do := func(tasks []*Task, expectErr string) []string {
		hashes, files, err := c.IsolateTasks(ctx, tasks)
		if expectErr == "" {
			require.NoError(t, err)
			require.Equal(t, len(tasks), len(hashes))
			require.Equal(t, len(tasks), len(files))
			return hashes
		} else {
			require.EqualError(t, err, expectErr)
		}
		return nil
	}

	// Write some files to isolate.
	writeIsolateFile := func(filepath string, contents *isolateFile) {
		f, err := os.Create(filepath)
		require.NoError(t, err)
		defer testutils.AssertCloses(t, f)
		require.NoError(t, contents.Encode(f))
	}

	myFile1 := "myfile1"
	myFile1Path := path.Join(workdir, myFile1)
	require.NoError(t, ioutil.WriteFile(myFile1Path, []byte(myFile1), 0644))
	myFile2 := "myfile2"
	myFile2Path := path.Join(workdir, myFile2)
	require.NoError(t, ioutil.WriteFile(myFile2Path, []byte(myFile2), 0644))

	dummyIsolate1 := path.Join(workdir, "dummy1.isolate")
	writeIsolateFile(dummyIsolate1, &isolateFile{
		Includes: []string{},
		Files:    []string{myFile1},
	})
	dummyIsolate2 := path.Join(workdir, "dummy2.isolate")
	writeIsolateFile(dummyIsolate2, &isolateFile{
		Includes: []string{},
		Files:    []string{myFile2},
	})

	// Empty tasks list.
	do([]*Task{}, "")

	// Invalid task.
	t1 := &Task{}
	_ = do([]*Task{t1}, "BaseDir is required.")
	t1.BaseDir = workdir
	_ = do([]*Task{t1}, "IsolateFile is required.")
	t1.IsolateFile = dummyIsolate1
	// Task with no OsType is ok.
	_ = do([]*Task{t1}, "")
	// Add OsType for below tests.
	t1.OsType = "linux"

	// Minimum valid task.
	hashes := do([]*Task{t1}, "")
	h1 := hashes[0]

	// Add a duplicate task.
	t2 := &Task{
		BaseDir:     workdir,
		IsolateFile: dummyIsolate1,
		OsType:      "linux",
	}
	hashes = do([]*Task{t1, t2}, "")
	assertdeep.Equal(t, hashes, []string{h1, h1})

	// Tweak the second task.
	t2.IsolateFile = dummyIsolate2
	hashes = do([]*Task{t1, t2}, "")
	h2 := hashes[1]
	require.NotEqual(t, h1, h2)
	assertdeep.Equal(t, hashes, []string{h1, h2})

	// Add a dependency of t2 on t1. Ensure that we get a different hash,
	// which implies that the dependency was added successfully.
	t2.Deps = []string{h1}
	hashes = do([]*Task{t2}, "")
	require.NotEqual(t, h2, hashes[0])

	// Isolate a bunch of tasks individually and then all at once, ensuring
	// that we get the same hashes in the correct order.
	tasks := []*Task{}
	expectHashes := []string{}
	for i := 0; i < 11; i++ {
		f := fmt.Sprintf("myfile%d", i)
		fp := path.Join(workdir, f)
		require.NoError(t, ioutil.WriteFile(fp, []byte(f), 0644))
		dummyIsolate := path.Join(workdir, fmt.Sprintf("dummy%d.isolate", i))
		writeIsolateFile(dummyIsolate, &isolateFile{
			Includes: []string{},
			Files:    []string{f},
		})
		t := &Task{
			BaseDir:     workdir,
			IsolateFile: dummyIsolate,
			OsType:      "linux",
		}
		h := do([]*Task{t}, "")
		tasks = append(tasks, t)
		expectHashes = append(expectHashes, h[0])
	}
	gotHashes := do(tasks, "")
	assertdeep.Equal(t, expectHashes, gotHashes)
}

func TestDownloadIsolateHash(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	workdir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)
	ctx := context.Background()
	testHash := "dummy-hash"
	downloadedFileList := "dummy-file"

	// Mock isolated download.
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		require.Equal(t, "isolated", cmd.Name)
		require.Equal(t, "download", cmd.Args[0])
		downloadArgs := strings.Join(cmd.Args[1:], " ")
		require.Equal(t, true, strings.Contains(downloadArgs, fmt.Sprintf("--isolated %s", testHash)))
		require.Equal(t, true, strings.Contains(downloadArgs, fmt.Sprintf("--output-dir %s", workdir)))
		require.Equal(t, true, strings.Contains(downloadArgs, fmt.Sprintf("--output-files %s", filepath.Join(workdir, downloadedFileList))))
		return nil
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	c, err := NewClient(workdir, ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)

	err = c.DownloadIsolateHash(ctx, testHash, workdir, downloadedFileList)
	require.NoError(t, err)
}

func TestReUploadIsolatedFiles(t *testing.T) {
	unittest.LargeTest(t)

	// Setup.
	workdir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)

	c, err := NewClient(workdir, ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)

	ctx := context.Background()

	link := "link"
	mode := 777
	size := int64(9000)
	i1 := &isolated.Isolated{
		Algo: "smrt",
		Files: map[string]isolated.File{
			"my-file": {
				Digest: "abc123",
				Link:   &link,
				Mode:   &mode,
				Size:   &size,
				Type:   isolated.Basic,
			},
		},
		Includes: []isolated.HexDigest{"blah"},
		Version:  "NEW!",
	}
	// Initial upload.
	hashes, err := c.ReUploadIsolatedFiles(ctx, []*isolated.Isolated{i1})
	require.NoError(t, err)
	require.Equal(t, 1, len(hashes))
	hash1 := hashes[0]
	require.Equal(t, 40, len(hash1)) // Sanity check.

	// Re-upload the same Isolated. We should have the same hash.
	hashes, err = c.ReUploadIsolatedFiles(ctx, []*isolated.Isolated{i1})
	require.NoError(t, err)
	require.Equal(t, 1, len(hashes))
	hash2 := hashes[0]
	require.Equal(t, hash1, hash2)

	// Now, change the Isolated. We should get a different hash.
	i1.Includes = append(i1.Includes, "anotherhash")
	hashes, err = c.ReUploadIsolatedFiles(ctx, []*isolated.Isolated{i1})
	require.NoError(t, err)
	require.Equal(t, 1, len(hashes))
	hash3 := hashes[0]
	require.Equal(t, 40, len(hash3)) // Sanity check.
	require.NotEqual(t, hash1, hash3)
}
