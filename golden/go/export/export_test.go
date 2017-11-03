package export

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/util"
)

const TEST_FILE_NAME = "gold_index.json"

func TestWriteRead(t *testing.T) {
	testRecs := []*TestRecord{
		{},
	}

	assert.NoError(t, WriteTestRecords(TEST_FILE_NAME, testRecs))
	defer util.Remove(TEST_FILE_NAME)
	found, err := ReadTestRecords(TEST_FILE_NAME)
	assert.NoError(t, err)
	assert.Equal(t, testRecs, found)
}
