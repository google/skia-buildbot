package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/samplestats"
)

func TestMain_DifferentFlags_ChangeOutput(t *testing.T) {
	unittest.SmallTest(t)

	testdata, err := testutils.TestDataDir()
	require.NoError(t, err)
	oldFile := filepath.Join(testdata, "nanobench_old.json")
	newFile := filepath.Join(testdata, "nanobench_new.json")

	// See the regenerate-testdata target in the Makefile to update these tests.
	check(t, "all", "--all", oldFile, newFile)
	check(t, "iqrr", "--iqrr", oldFile, newFile)
	check(t, "sort", "--sort=name", oldFile, newFile)
	check(t, "test", "--test=ttest", oldFile, newFile)
}

// check a single run of nanotest with the given 'args'.
//
// name - the root name of the golden file and also the name of the sub-test.
func check(t *testing.T, name string, args ...string) {
	t.Run(name, func(t *testing.T) {
		os.Args = append([]string{"nanostat"}, args...)

		// Create a pipe to capture stdout.
		r, w, err := os.Pipe()
		require.NoError(t, err)
		c := make(chan []byte)
		go func() {
			data, err := ioutil.ReadAll(r)
			require.NoError(t, err)
			c <- data
		}()

		stdout := os.Stdout
		os.Stdout = w
		osExit = func(code int) {
			require.FailNow(t, "exit %d during main", code)
		}

		// Reset flags to defaults before each test run.
		*flagAlpha = 0.05
		*flagSort = "delta"
		*flagIQRR = false
		*flagAll = false
		*flagTest = string(samplestats.UTest)
		main()

		w.Close()
		os.Stdout = stdout
		osExit = os.Exit

		data := <-c
		testdata, err := testutils.TestDataDir()
		require.NoError(t, err)
		golden, err := ioutil.ReadFile(filepath.Join(testdata, name+".golden"))
		require.NoError(t, err)
		assert.Equal(t, string(golden), string(data))
	})
}
