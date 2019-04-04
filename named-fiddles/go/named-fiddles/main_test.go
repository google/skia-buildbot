package main

import (
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"
)

const (
	TEST_DATA_DIR = "./testdata"
)

func TestGetIncludes(t *testing.T) {
	values, err := getIncludes(filepath.Join(TEST_DATA_DIR, "example.cpp"))
	assert.Nil(t, err)
	assert.Equal(t, 4, len(values))
	assert.Equal(t, values[0], filepath.Join(TEST_DATA_DIR, "foo.cpp"))
	assert.Equal(t, values[1], filepath.Join(TEST_DATA_DIR, "foobar.cpp"))
	assert.Equal(t, values[2], filepath.Join(TEST_DATA_DIR, "../bar.cpp"))
	assert.Equal(t, values[3], filepath.Join(TEST_DATA_DIR, "../foobaz.cpp"))

	_, err = getIncludes(filepath.Join(TEST_DATA_DIR, "nonexist.cpp"))
	assert.NotNil(t, err)
}
