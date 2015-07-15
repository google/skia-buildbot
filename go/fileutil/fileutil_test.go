package fileutil

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestTwoLevelRadixPath(t *testing.T) {
	assert.Equal(t, "", TwoLevelRadixPath(""))
	assert.Equal(t, "ab/cd/abcdefgh.txt", TwoLevelRadixPath("abcdefgh.txt"))
	assert.Equal(t, "/etc/xyz/ab.txt", TwoLevelRadixPath("/etc", "xyz/ab.txt"))
	assert.Equal(t, "/etc/xyz/ab/cd/abcdefg.txt", TwoLevelRadixPath("/etc", "xyz/abcdefg.txt"))
	assert.Equal(t, "so/me/somefile_no_ext", TwoLevelRadixPath("somefile_no_ext"))
}
