package logagents

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/skolo/go/logparser"
)

func TestNoRollover(t *testing.T) {
	unittest.SmallTest(t)
	mockOutPersistence()
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
	logger.Flush()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestNoRollover2(t *testing.T) {
	unittest.SmallTest(t)
	// Checks rollover with the rollover file actually having something.
	mockOutPersistence()
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
	logger.Flush()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestRolloverToEmpty(t *testing.T) {
	unittest.SmallTest(t)
	mockOutPersistence()
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
	logger.Flush()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestRolloverWithNew(t *testing.T) {
	unittest.SmallTest(t)
	mockOutPersistence()
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
	logger.Flush()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1 and 4 new lines in pylog 2.0
	assert.Equal(t, 7, logger.Count())
}

func TestWritePersistence(t *testing.T) {
	unittest.SmallTest(t)
	readFromPersistenceFile = func(reportName string, v interface{}) error {
		return nil
	}
	log1 := testutils.MustReadFile("pylog1.0")
	log2 := testutils.MustReadFile("pylog1.1")
	log3 := testutils.MustReadFile("pylog2.0")

	roll := NewRollover(logparser.ParsePythonLog, "pylog", "test", "test.1")
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
	writeToPersistenceFile = func(reportName string, v interface{}) error {
		rlog, ok := v.(*rolloverLog)
		if !ok {
			t.Errorf("The passed in type to write was wrong: %#v", v)
			return nil
		}
		assert.Equal(t, "abcd", rlog.LogHash)
		assert.Equal(t, "rofl", rlog.RolloverHash)
		assert.Equal(t, 2, rlog.LastLine)
		assert.Equal(t, false, rlog.IsFirstScan)
		return nil
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
	writeToPersistenceFile = func(reportName string, v interface{}) error {
		rlog, ok := v.(*rolloverLog)
		if !ok {
			t.Errorf("The passed in type to write was wrong: %#v", v)
			return nil
		}
		assert.Equal(t, "efgh", rlog.LogHash)
		assert.Equal(t, "rofl", rlog.RolloverHash)
		assert.Equal(t, 5, rlog.LastLine)
		assert.Equal(t, false, rlog.IsFirstScan)
		return nil
	}
	logger.Flush()
	assert.NoError(t, roll.Scan(logger))
	// There are 3 new lines in pylog1.1
	assert.Equal(t, 3, logger.Count())
}

func TestReadPersistenceHappy(t *testing.T) {
	unittest.SmallTest(t)
	readFromPersistenceFile = func(reportName string, v interface{}) error {
		rlog, ok := v.(*rolloverLog)
		if !ok {
			t.Errorf("The passed in type to write was wrong: %#v", v)
			return nil
		}
		if reportName != "pylog" {
			t.Errorf("Wrong reportName: %s", reportName)
			return nil
		}
		rlog.LogHash = "abc"
		rlog.RolloverHash = "def"
		rlog.LastLine = 3
		rlog.IsFirstScan = false
		return nil
	}
	r := NewRollover(logparser.ParsePythonLog, "pylog", "test", "test.1").(*rolloverLog)
	assert.Equal(t, "abc", r.LogHash)
	assert.Equal(t, "def", r.RolloverHash)
	assert.Equal(t, 3, r.LastLine)
	assert.Equal(t, false, r.IsFirstScan)
	assert.NotNil(t, r.Parse)
}

func TestReadPersistenceCorrupt(t *testing.T) {
	unittest.SmallTest(t)
	readFromPersistenceFile = func(reportName string, v interface{}) error {
		if reportName != "pylog" {
			t.Errorf("Wrong reportName: %s", reportName)
		}
		return fmt.Errorf("THERE WAS A PROBLEM (for testing purposes)")
	}
	r := NewRollover(logparser.ParsePythonLog, "pylog", "test", "test.1").(*rolloverLog)
	assert.Equal(t, "", r.LogHash)
	assert.Equal(t, "", r.RolloverHash)
	assert.Equal(t, 0, r.LastLine)
	assert.Equal(t, true, r.IsFirstScan)
}

type mockCloudLogger struct {
	callCount int
}

func (m *mockCloudLogger) CloudLog(reportName string, payload *sklog.LogPayload) {
	m.callCount++
}

func (m *mockCloudLogger) BatchCloudLog(reportName string, payloads ...*sklog.LogPayload) {
	m.callCount += len(payloads)
}

func (m *mockCloudLogger) Count() int {
	return m.callCount
}

func (m *mockCloudLogger) Flush() {
	m.callCount = 0
}

func mockOutPersistence() {
	writeToPersistenceFile = func(reportName string, v interface{}) error {
		return nil
	}

	readFromPersistenceFile = func(reportName string, v interface{}) error {
		return nil
	}
}
