package main

import (
	"path/filepath"
	"testing"
	"strings"

	assert "github.com/stretchr/testify/require"
)

const testGetIncludesTestString = `// some comments

#include "foo.cpp"
#include "foobar.cpp"    
    #include "../bar.cpp"
  #include   "../foobaz.cpp"   
`
func TestGetIncludes(t *testing.T) {
	const testDir = "./testdata"
	reader := strings.NewReader(testGetIncludesTestString)

	values, err := getIncludes(testDir, reader)
	assert.Nil(t, err)
	assert.Equal(t, 4, len(values))
	assert.Equal(t, values[0], filepath.Join(testDir, "foo.cpp"))
	assert.Equal(t, values[1], filepath.Join(testDir, "foobar.cpp"))
	assert.Equal(t, values[2], filepath.Join(testDir, "../bar.cpp"))
	assert.Equal(t, values[3], filepath.Join(testDir, "../foobaz.cpp"))
}
