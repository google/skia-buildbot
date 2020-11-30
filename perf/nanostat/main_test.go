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
)

func TestMain(t *testing.T) {
	unittest.SmallTest(t)

	testdata, err := testutils.TestDataDir()
	require.NoError(t, err)
	oldFile := filepath.Join(testdata, "nanobench_old.json")
	newFile := filepath.Join(testdata, "nanobench_new.json")

	check(t, "all", "--all", oldFile, newFile)
	check(t, "iqrr", "--iqrr", oldFile, newFile)
}

func check(t *testing.T, name string, args ...string) {
	t.Run(name, func(t *testing.T) {
		os.Args = append([]string{"nanostat"}, args...)
		r, w, err := os.Pipe()
		require.NoError(t, err)
		c := make(chan []byte)
		go func() {
			data, err := ioutil.ReadAll(r)
			require.NoError(t, err)
			c <- data
		}()

		stdout := os.Stdout
		stderr := os.Stderr
		os.Stdout = w
		os.Stderr = w
		osExit = func(code int) {
			require.FailNow(t, "exit %d during main", code)
		}

		main()

		w.Close()
		os.Stdout = stdout
		os.Stderr = stderr
		osExit = os.Exit

		data := <-c
		testdata, err := testutils.TestDataDir()
		require.NoError(t, err)
		golden, err := ioutil.ReadFile(filepath.Join(testdata, name+".golden"))
		require.NoError(t, err)
		assert.Equal(t, string(golden), string(data))
	})
}
