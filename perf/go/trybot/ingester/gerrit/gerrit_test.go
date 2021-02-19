package gerrit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/types"
)

var createdTime = time.Date(2020, 01, 01, 00, 00, 00, 0000, time.UTC)

func setupForTest(t *testing.T, filename string) (*parser.Parser, file.File) {
	unittest.SmallTest(t)
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: []string{}, // Branches are ignored by ParseTryBot.
		},
	}
	p := parser.New(instanceConfig)

	return p, file.File{
		Name:     filename,
		Contents: testutils.GetReader(t, filename),
		Created:  createdTime,
	}
}

func TestGerrit_InvalidTryBotFile_DoesNotProduceTryFile(t *testing.T) {
	unittest.SmallTest(t)

	parser, invalidFile := setupForTest(t, "invalid.json")
	ingester := New(parser)
	ingester.parseCounter.Reset()
	ingester.parseFailCounter.Reset()
	fileCh := make(chan file.File)
	tryFileCh, err := ingester.Start(fileCh)
	assert.NoError(t, err)
	fileCh <- invalidFile
	close(fileCh)

	// Wait for channel to close.
	numTryFiles := 0
	for range tryFileCh {
		numTryFiles++
	}
	assert.Equal(t, 0, numTryFiles)
	assert.Equal(t, int64(0), ingester.parseCounter.Get())
	assert.Equal(t, int64(1), ingester.parseFailCounter.Get())
}

func TestGerrit_InvalidPatchNumber_DoesNotProduceTryFile(t *testing.T) {
	unittest.SmallTest(t)

	parser, invalidFile := setupForTest(t, "invalid_patch_number.json")
	ingester := New(parser)
	ingester.parseCounter.Reset()
	ingester.parseFailCounter.Reset()
	fileCh := make(chan file.File)
	tryFileCh, err := ingester.Start(fileCh)
	assert.NoError(t, err)
	fileCh <- invalidFile
	close(fileCh)

	// Wait for channel to close.
	numTryFiles := 0
	for range tryFileCh {
		numTryFiles++
	}
	assert.Equal(t, 0, numTryFiles)
	assert.Equal(t, int64(0), ingester.parseCounter.Get())
	assert.Equal(t, int64(1), ingester.parseFailCounter.Get())
}

func TestGerrit_ValidTryBotFile_Success(t *testing.T) {
	unittest.SmallTest(t)

	const filename = "success.json"
	parser, invalidFile := setupForTest(t, filename)
	ingester := New(parser)
	ingester.parseCounter.Reset()
	ingester.parseFailCounter.Reset()
	fileCh := make(chan file.File)
	tryFileCh, err := ingester.Start(fileCh)
	assert.NoError(t, err)
	fileCh <- invalidFile
	close(fileCh)

	tryFile := <-tryFileCh

	// Wait for channel to close.
	for range tryFileCh {
	}

	assert.Equal(t, types.CL("327697"), tryFile.CL)
	assert.Equal(t, 1, tryFile.PatchNumber)
	assert.Equal(t, createdTime, tryFile.Timestamp)
	assert.Equal(t, filename, tryFile.Filename)
	assert.Equal(t, int64(1), ingester.parseCounter.Get())
	assert.Equal(t, int64(0), ingester.parseFailCounter.Get())
}
