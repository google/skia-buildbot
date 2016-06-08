package logagents

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/skolo/go/gcl"
	"go.skia.org/infra/skolo/go/logparser"
)

func TestNoRollover(t *testing.T) {
	log1 := testutils.MustReadFile("pylog1.0")
	log2 := testutils.MustReadFile("pylog1.1")

	roll := NewRollover(logparser.ParsePythonLog, "dontcare", "test", "test.1")
	logger := &mockCloudLogger{}

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log1, "abcd", nil
		}
		if path == "test.1" {
			return "", "", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	assert.NoError(t, roll.Scan(logger))
	// Should be 2 lines here.  See parser_test for more thorough assertions.
	assert.Equal(t, 2, logger.Count())

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log2, "efgh", nil
		}
		if path == "test.1" {
			return "", "", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	logger.Reset()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestNoRollover2(t *testing.T) {
	// Checks rollover with the rollover file actually having something.
	log1 := testutils.MustReadFile("pylog1.0")
	log2 := testutils.MustReadFile("pylog1.1")
	log3 := testutils.MustReadFile("pylog2.0")

	roll := NewRollover(logparser.ParsePythonLog, "dontcare", "test", "test.1")
	logger := &mockCloudLogger{}

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log1, "abcd", nil
		}
		if path == "test.1" {
			return log3, "rofl", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	assert.NoError(t, roll.Scan(logger))
	// Should be 2 lines here.  See parser_test for more thorough assertions.
	assert.Equal(t, 2, logger.Count())

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log2, "efgh", nil
		}
		if path == "test.1" {
			return log3, "rofl", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	logger.Reset()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestRolloverToEmpty(t *testing.T) {
	log1 := testutils.MustReadFile("pylog1.0")
	log2 := testutils.MustReadFile("pylog1.1")

	roll := NewRollover(logparser.ParsePythonLog, "dontcare", "test", "test.1")
	logger := &mockCloudLogger{}

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log1, "abcd", nil
		}
		if path == "test.1" {
			return "", "", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	assert.NoError(t, roll.Scan(logger))
	// Should be 2 lines here.  See parser_test for more thorough assertions.
	assert.Equal(t, 2, logger.Count())

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return "", "", nil
		}
		if path == "test.1" {
			return log2, "efgh", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	logger.Reset()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestRolloverWithNew(t *testing.T) {
	log1 := testutils.MustReadFile("pylog1.0")
	log2 := testutils.MustReadFile("pylog1.1")
	log3 := testutils.MustReadFile("pylog2.0")

	roll := NewRollover(logparser.ParsePythonLog, "dontcare", "test", "test.1")
	logger := &mockCloudLogger{}

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log1, "abcd", nil
		}
		if path == "test.1" {
			return "", "", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	assert.NoError(t, roll.Scan(logger))
	// Should be 2 lines here.  See parser_test for more thorough assertions.
	assert.Equal(t, 2, logger.Count())

	readAndHashFile = func(path string) (contents, hash string, err error) {
		if path == "test" {
			return log3, "efgh", nil
		}
		if path == "test.1" {
			return log2, "rofl", nil
		}
		t.Errorf("Unexpected call to readAndhashFile: %s", path)
		return "", "", nil
	}
	logger.Reset()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1 and 4 new lines in pylog 2.0
	assert.Equal(t, 7, logger.Count())
}

type mockCloudLogger struct {
	callCount int
}

func (m *mockCloudLogger) Log(reportName string, payload *gcl.LogPayload) {
	m.callCount++
}

func (m *mockCloudLogger) Count() int {
	return m.callCount
}

func (m *mockCloudLogger) Reset() {
	m.callCount = 0
}
