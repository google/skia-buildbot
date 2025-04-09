package du

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	// All files are smaller than 4096 bytes, or 8 512-byte blocks.
	fileBlockCount = 8
	// defaultDirBlockCount is 4096 bytes, or 8 512-byte blocks, which is a
	// reasonable default for most systems.
	defaultDirBlockCount = 8
)

var (
	subdir1_subdir_file1_contents = []byte{0, 0, 0, 0, 0, 0, 0, 0}
	subdir1_file_contents         = []byte{0, 0, 0, 0, 0, 0}
	toplevel_file_contents        = []byte{0, 0, 0, 0}
)

// getExampleDir returns an example directory structure used for testing.
func getExampleDir(name string, dirBlockCount uint64) *Dir {
	return &Dir{
		Name: name,
		Dirs: []*Dir{
			{
				Name:        "subdir2",
				Dirs:        []*Dir{},
				Files:       []*File{},
				Blocks:      dirBlockCount,
				TotalBlocks: dirBlockCount,
				TotalFiles:  0,
				TotalSize:   dirBlockCount * blockSize,
			},
			{
				Name: "subdir1",
				Dirs: []*Dir{
					{
						Name: "subdir1_subdir",
						Dirs: []*Dir{},
						Files: []*File{
							{
								Name:   "subdir1_subdir_file1",
								Size:   8,
								Blocks: fileBlockCount,
							},
						},
						Blocks:      dirBlockCount,
						TotalBlocks: dirBlockCount + fileBlockCount,
						TotalFiles:  1,
						TotalSize:   (dirBlockCount + fileBlockCount) * blockSize,
					},
				},
				Files: []*File{
					{
						Name:   "subdir1_file",
						Size:   6,
						Blocks: fileBlockCount,
					},
				},
				Blocks:      dirBlockCount,
				TotalBlocks: 2*dirBlockCount + 2*fileBlockCount,
				TotalFiles:  2,
				TotalSize:   (2*dirBlockCount + 2*fileBlockCount) * blockSize,
			},
		},
		Files: []*File{
			{
				Name:   "toplevel_file",
				Size:   4,
				Blocks: fileBlockCount,
			},
		},
		Blocks:      dirBlockCount,
		TotalBlocks: 4*dirBlockCount + 3*fileBlockCount,
		TotalFiles:  3,
		TotalSize:   (4*dirBlockCount + 3*fileBlockCount) * blockSize,
	}
}

func TestUsage(t *testing.T) {
	tmp := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir1", "subdir1_subdir"), os.ModePerm))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir2"), os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "subdir1", "subdir1_subdir", "subdir1_subdir_file1"), subdir1_subdir_file1_contents, os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "subdir1", "subdir1_file"), subdir1_file_contents, os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "toplevel_file"), toplevel_file_contents, os.ModePerm))

	ctx := context.Background()
	actual, err := Usage(ctx, tmp)
	require.NoError(t, err)

	// Find the number of 512-byte blocks occupied by directories on this
	// system, to ensure that our expectations match reality.
	var stat syscall.Stat_t
	require.NoError(t, syscall.Stat(tmp, &stat))
	expect := getExampleDir(tmp, uint64(stat.Blocks))
	require.Equal(t, expect, actual)
}

func TestGenerateReport(t *testing.T) {
	ctx := context.Background()

	test := func(name string, maxDepth int, human bool, expect string) {
		t.Run(name, func(t *testing.T) {
			actual, err := GenerateReport(ctx, getExampleDir(".", defaultDirBlockCount), maxDepth, human)
			require.NoError(t, err)
			require.Equal(t, expect, actual)
		})
	}

	test("no-max-depth", 0, false, `4096	subdir2
8192	subdir1/subdir1_subdir
16384	subdir1
28672	.`)

	test("max-depth-1", 1, false, `4096	subdir2
16384	subdir1
28672	.`)

	test("max-depth-2", 2, false, `4096	subdir2
8192	subdir1/subdir1_subdir
16384	subdir1
28672	.`)

	test("max-depth-3", 3, false, `4096	subdir2
8192	subdir1/subdir1_subdir
16384	subdir1
28672	.`)

	test("human", 0, true, `4.0 KiB	subdir2
8.0 KiB	subdir1/subdir1_subdir
16 KiB	subdir1
28 KiB	.`)
}

func TestGenerateJSONReport(t *testing.T) {
	ctx := context.Background()

	test := func(name string, maxDepth int, human bool, expect string) {
		t.Run(name, func(t *testing.T) {
			actual, err := GenerateJSONReport(ctx, getExampleDir(".", defaultDirBlockCount), maxDepth, human)
			require.NoError(t, err)
			require.Equal(t, expect, actual)
		})
	}

	test("no-max-depth", 0, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"8192"}],"size":"16384"}],"size":"28672"}`)

	test("max-depth-1", 1, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","size":"16384"}],"size":"28672"}`)

	test("max-depth-2", 2, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"8192"}],"size":"16384"}],"size":"28672"}`)

	test("max-depth-3", 3, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"8192"}],"size":"16384"}],"size":"28672"}`)

	test("human", 0, true, `{"name":".","dirs":[{"name":"subdir2","size":"4.0 KiB"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"8.0 KiB"}],"size":"16 KiB"}],"size":"28 KiB"}`)
}
