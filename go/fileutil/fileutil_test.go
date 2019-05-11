package fileutil

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	TEST_DATA_DIR = "./testdata"
)

func TestTwoLevelRadixPath(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "", TwoLevelRadixPath(""))
	assert.Equal(t, "ab/cd/abcdefgh.txt", TwoLevelRadixPath("abcdefgh.txt"))
	assert.Equal(t, "/etc/xyz/ab.txt", TwoLevelRadixPath("/etc", "xyz/ab.txt"))
	assert.Equal(t, "/etc/xyz/ab/cd/abcdefg.txt", TwoLevelRadixPath("/etc", "xyz/abcdefg.txt"))
	assert.Equal(t, "so/me/somefile_no_ext", TwoLevelRadixPath("somefile_no_ext"))
}

func TestCountLines(t *testing.T) {
	unittest.MediumTest(t)

	lines, err := CountLines(filepath.Join(TEST_DATA_DIR, "no_lines_file.txt"))
	assert.Nil(t, err)
	assert.Equal(t, 0, lines)

	lines, err = CountLines(filepath.Join(TEST_DATA_DIR, "one_line_file.txt"))
	assert.Nil(t, err)
	assert.Equal(t, 1, lines)

	lines, err = CountLines(filepath.Join(TEST_DATA_DIR, "ten_lines_file.txt"))
	assert.Nil(t, err)
	assert.Equal(t, 10, lines)

	lines, err = CountLines(filepath.Join(TEST_DATA_DIR, "non_existant.txt"))
	assert.NotNil(t, err)
	assert.Equal(t, -1, lines)
}

func TestReadAllFilesRecursive(t *testing.T) {
	unittest.LargeTest(t)

	test := func(write, expect map[string]string, excludeDirs []string) {
		wd, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		for k, v := range write {
			dir := path.Dir(k)
			if dir != "" {
				assert.NoError(t, os.MkdirAll(path.Join(wd, dir), os.ModePerm))
			}
			assert.NoError(t, ioutil.WriteFile(path.Join(wd, k), []byte(v), os.ModePerm))
		}
		actual, err := ReadAllFilesRecursive(wd, excludeDirs)
		assert.NoError(t, err)
		expectBytes := make(map[string][]byte, len(expect))
		for k, v := range expect {
			expectBytes[k] = []byte(v)
		}
		deepequal.AssertDeepEqual(t, expectBytes, actual)
	}
	test(nil, map[string]string{}, nil)
	test(map[string]string{
		"somefile": "contents",
	}, map[string]string{
		"somefile": "contents",
	}, nil)
	test(map[string]string{
		"a/b/c": "contents",
	}, map[string]string{
		"a/b/c": "contents",
	}, nil)
	test(map[string]string{
		"a/file": "contents",
		"b/file": "contents",
	}, map[string]string{
		"b/file": "contents",
	}, []string{"a"})
}
