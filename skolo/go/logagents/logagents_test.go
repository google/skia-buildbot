package logagents

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"
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
	assert.NoError(t, err)
	assert.NoError(t, SetPersistenceDir(dir))

	test := testStruct{}
	expected := testStruct{
		Hello: "hello",
		World: 1234,
	}
	assert.NoError(t, _readFromPersistenceFile("rolloverPersist", &test))
	assert.Equal(t, expected, test)
}

func TestWritePersistenceActual(t *testing.T) {
	unittest.SmallTest(t)
	dir, err := ioutil.TempDir("", "writepersist")
	assert.NoError(t, err)
	assert.NoError(t, SetPersistenceDir(dir))

	test := testStruct{
		Hello: "hello",
		World: 1234,
	}
	assert.NoError(t, _writeToPersistenceFile("testpersist", test))
	f, err := os.Open(filepath.Join(dir, "testpersist"))
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	assert.NoError(t, err)
	expected := testutils.MustReadFile("rolloverPersist")
	assert.Equal(t, expected, string(b))
}
