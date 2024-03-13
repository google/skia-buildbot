package du

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	subdir1_subdir_file1_contents = []byte{0, 0, 0, 0, 0, 0, 0, 0}
	subdir1_file_contents         = []byte{0, 0, 0, 0, 0, 0}
	toplevel_file_contents        = []byte{0, 0, 0, 0}
)

// getExampleDir returns an example directory structure used for testing.
func getExampleDir(name string) *Dir {
	return &Dir{
		Name: name,
		Dirs: []*Dir{
			{
				Name:       "subdir2",
				Dirs:       []*Dir{},
				Files:      []*File{},
				TotalSize:  4096,
				TotalFiles: 0,
			},
			{
				Name: "subdir1",
				Dirs: []*Dir{
					{
						Name: "subdir1_subdir",
						Dirs: []*Dir{},
						Files: []*File{
							{
								Name: "subdir1_subdir_file1",
								Size: 8,
							},
						},
						TotalSize:  4096 + 8,
						TotalFiles: 1,
					},
				},
				Files: []*File{
					{
						Name: "subdir1_file",
						Size: 6,
					},
				},
				TotalSize:  4096 + 4096 + 6 + 8,
				TotalFiles: 2,
			},
		},
		Files: []*File{
			{
				Name: "toplevel_file",
				Size: 4,
			},
		},
		TotalSize:  4096*4 + 8 + 6 + 4,
		TotalFiles: 3,
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
	expect := getExampleDir(tmp)
	require.Equal(t, expect, actual)
}

func TestGenerateReport(t *testing.T) {
	ctx := context.Background()

	test := func(name string, maxDepth int, human bool, expect string) {
		t.Run(name, func(t *testing.T) {
			actual, err := GenerateReport(ctx, getExampleDir("."), ".", maxDepth, human)
			require.NoError(t, err)
			require.Equal(t, expect, actual)
		})
	}

	test("no-max-depth", 0, false, `4096	subdir2
4104	subdir1/subdir1_subdir
8206	subdir1
16402	.`)

	test("max-depth-1", 1, false, `4096	subdir2
8206	subdir1
16402	.`)

	test("max-depth-2", 2, false, `4096	subdir2
4104	subdir1/subdir1_subdir
8206	subdir1
16402	.`)

	test("max-depth-3", 3, false, `4096	subdir2
4104	subdir1/subdir1_subdir
8206	subdir1
16402	.`)

	test("human", 0, true, `4.1 kB	subdir2
4.1 kB	subdir1/subdir1_subdir
8.2 kB	subdir1
16 kB	.`)
}

func TestGenerateJSONReport(t *testing.T) {
	ctx := context.Background()

	test := func(name string, maxDepth int, human bool, expect string) {
		t.Run(name, func(t *testing.T) {
			actual, err := GenerateJSONReport(ctx, getExampleDir("."), ".", maxDepth, human)
			require.NoError(t, err)
			require.Equal(t, expect, actual)
		})
	}

	test("no-max-depth", 0, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"4104"}],"size":"8206"}],"size":"16402"}`)

	test("max-depth-1", 1, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","size":"8206"}],"size":"16402"}`)

	test("max-depth-2", 2, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"4104"}],"size":"8206"}],"size":"16402"}`)

	test("max-depth-3", 3, false, `{"name":".","dirs":[{"name":"subdir2","size":"4096"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"4104"}],"size":"8206"}],"size":"16402"}`)

	test("human", 0, true, `{"name":".","dirs":[{"name":"subdir2","size":"4.1 kB"},{"name":"subdir1","dirs":[{"name":"subdir1/subdir1_subdir","size":"4.1 kB"}],"size":"8.2 kB"}],"size":"16 kB"}`)
}
