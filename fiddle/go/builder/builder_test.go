package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
)

func allAvailable(t *testing.T, testData []string) {
	tempDir, err := ioutil.TempDir("", "builder_test_")
	assert.NoError(t, err)
	defer util.RemoveAll(tempDir)
	fi, err := os.Create(filepath.Join(tempDir, GOOD_BUILDS_FILENAME))
	assert.NoError(t, err)
	fmt.Fprintf(fi, strings.Join(testData, "\n"))
	err = fi.Close()
	assert.NoError(t, err)

	b := New(tempDir, "")
	lst, err := b.AvailableBuilds()
	assert.NoError(t, err)
	assert.Equal(t, len(testData), len(lst))
	assert.Equal(t, len(testData), len(lst))

	reversed := []string{}
	for _, r := range testData {
		reversed = append(reversed, r)
	}
	assert.Equal(t, reversed, testData)
}

func TestAllAvailable(t *testing.T) {
	allAvailable(t, []string{
		"fea7de6c1459cb26c9e0a0c72033e9ccaea56530",
		"4d51f64ff18e2e15c40fec0c374d89879ba273bc",
	})
	allAvailable(t, []string{
		"fea7de6c1459cb26c9e0a0c72033e9ccaea56530",
	})
	allAvailable(t, []string{})
}
