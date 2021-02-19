package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMain_DifferentFlags_ChangeOutput(t *testing.T) {
	unittest.SmallTest(t)

	testdataDir := testutils.TestDataDir(t)
	oldFile := filepath.Join(testdataDir, "nanobench_old.json")
	newFile := filepath.Join(testdataDir, "nanobench_new.json")

	// See the regenerate-testdata target in the Makefile to update these tests.
	check(t, "all", "--all", oldFile, newFile)
	check(t, "iqrr", "--iqrr", oldFile, newFile)
	check(t, "sort", "--sort=name", oldFile, newFile)
	check(t, "test", "--test=ttest", oldFile, newFile)
	check(t, "nochange", oldFile, oldFile)
}

// check a single run of nanotest with the given 'args'.
//
// name - the root name of the golden file and also the name of the sub-test.
func check(t *testing.T, name string, args ...string) {
	t.Run(name, func(t *testing.T) {
		os.Args = append([]string{"nanostat"}, args...)

		var stdout bytes.Buffer
		actualMain(&stdout)

		golden := testutils.ReadFile(t, name+".golden")
		assert.Equal(t, golden, stdout.String())
	})
}
