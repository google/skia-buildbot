package logagents

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

type testStruct struct {
	Hello string
	World int
}

func TestReadPersistenceActual(t *testing.T) {
	unittest.SmallTest(t)
	dir, err := testutils.TestDataDir()
	require.NoError(t, err)
	require.NoError(t, SetPersistenceDir(dir))

	test := testStruct{}
	expected := testStruct{
		Hello: "hello",
		World: 1234,
	}
	require.NoError(t, _readFromPersistenceFile("rolloverPersist", &test))
	require.Equal(t, expected, test)
}

func TestWritePersistenceActual(t *testing.T) {
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "writepersist")
	require.NoError(t, err)
	require.NoError(t, SetPersistenceDir(dir))

	test := testStruct{
		Hello: "hello",
		World: 1234,
	}
	require.NoError(t, _writeToPersistenceFile("testpersist", test))
	f, err := os.Open(filepath.Join(dir, "testpersist"))
	require.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	require.NoError(t, err)
	expected := testutils.MustReadFile("rolloverPersist")
	require.Equal(t, expected, string(b))
}
