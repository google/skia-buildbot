package fileutil

import (
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

const (
	TEST_DATA_DIR = "./testdata"
)

func TestTwoLevelRadixPath(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "", TwoLevelRadixPath(""))
	assert.Equal(t, "ab/cd/abcdefgh.txt", TwoLevelRadixPath("abcdefgh.txt"))
	assert.Equal(t, "/etc/xyz/ab.txt", TwoLevelRadixPath("/etc", "xyz/ab.txt"))
	assert.Equal(t, "/etc/xyz/ab/cd/abcdefg.txt", TwoLevelRadixPath("/etc", "xyz/abcdefg.txt"))
	assert.Equal(t, "so/me/somefile_no_ext", TwoLevelRadixPath("somefile_no_ext"))
}

func TestCountLines(t *testing.T) {
	testutils.MediumTest(t)

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
