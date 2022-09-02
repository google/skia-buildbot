package fileutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

const (
	TEST_DATA_DIR = "./testdata"
)

func TestTwoLevelRadixPath(t *testing.T) {
	require.Equal(t, "", TwoLevelRadixPath(""))
	require.Equal(t, filepath.Join("ab", "cd", "abcdefgh.txt"), TwoLevelRadixPath("abcdefgh.txt"))
	require.Equal(t, filepath.Join("/etc", "xyz", "ab.txt"), TwoLevelRadixPath("/etc", "xyz/ab.txt"))
	require.Equal(t, filepath.Join("/etc", "xyz", "ab", "cd", "abcdefg.txt"), TwoLevelRadixPath("/etc", "xyz/abcdefg.txt"))
	require.Equal(t, filepath.Join("so", "me", "somefile_no_ext"), TwoLevelRadixPath("somefile_no_ext"))
}

func TestCountLines(t *testing.T) {

	lines, err := CountLines(filepath.Join(TEST_DATA_DIR, "no_lines_file.txt"))
	require.Nil(t, err)
	require.Equal(t, 0, lines)

	lines, err = CountLines(filepath.Join(TEST_DATA_DIR, "one_line_file.txt"))
	require.Nil(t, err)
	require.Equal(t, 1, lines)

	lines, err = CountLines(filepath.Join(TEST_DATA_DIR, "ten_lines_file.txt"))
	require.Nil(t, err)
	require.Equal(t, 10, lines)

	lines, err = CountLines(filepath.Join(TEST_DATA_DIR, "non_existant.txt"))
	require.NotNil(t, err)
	require.Equal(t, -1, lines)
}

func TestReadAllFilesRecursive(t *testing.T) {

	test := func(write, expect map[string]string, excludeDirs []string) {
		wd, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		for k, v := range write {
			dir := filepath.Dir(k)
			if dir != "" {
				require.NoError(t, os.MkdirAll(filepath.Join(wd, dir), os.ModePerm))
			}
			require.NoError(t, ioutil.WriteFile(filepath.Join(wd, k), []byte(v), os.ModePerm))
		}
		actual, err := ReadAllFilesRecursive(wd, excludeDirs)
		require.NoError(t, err)
		expectBytes := make(map[string][]byte, len(expect))
		for k, v := range expect {
			expectBytes[k] = []byte(v)
		}
		assertdeep.Equal(t, expectBytes, actual)
	}
	test(nil, map[string]string{}, nil)
	test(map[string]string{
		"somefile": "contents",
	}, map[string]string{
		"somefile": "contents",
	}, nil)
	test(map[string]string{
		filepath.Join("a", "b", "c"): "contents",
	}, map[string]string{
		filepath.Join("a", "b", "c"): "contents",
	}, nil)
	test(map[string]string{
		filepath.Join("a", "file"): "contents",
		filepath.Join("b", "file"): "contents",
	}, map[string]string{
		filepath.Join("b", "file"): "contents",
	}, []string{"a"})
}
