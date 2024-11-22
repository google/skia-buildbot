package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.skia.org/infra/go/now"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"

	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/config/validation"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/ingestion/mocks"
)

func TestPubSubSource_IngestFile_PrimaryBranch_NoErrors_Ack(t *testing.T) {

	const realPrimaryBranchFile = "dm-json-v1/2021/03/02/15/a07ced8f471f8139771d045086aa6e2c2d6746ab/waterfall/dm-1614698630345047867.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realPrimaryBranchFile).Return(true)
	mp.On("Process", testutils.AnyContext, realPrimaryBranchFile).Return(nil)

	ms := &mocks.Store{}
	ms.On("SetIngested", testutils.AnyContext, realPrimaryBranchFile, mock.Anything).Return(nil)

	ps := pubSubSource{
		IngestionStore:                 ms,
		PrimaryBranchProcessor:         mp,
		PrimaryBranchStreamingLiveness: nopLiveness{},
		SuccessCounter:                 nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), realPrimaryBranchFile)
	assert.True(t, shouldAck)
	ms.AssertExpectations(t)
}

func TestPubSubSource_IngestFile_PrimaryBranch_NonRetryableError_Ack(t *testing.T) {

	const realPrimaryBranchFile = "dm-json-v1/2021/03/02/15/a07ced8f471f8139771d045086aa6e2c2d6746ab/waterfall/dm-1614698630345047867.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realPrimaryBranchFile).Return(true)
	mp.On("Process", testutils.AnyContext, realPrimaryBranchFile).Return(errors.New("invalid JSON"))

	ms := &mocks.Store{}
	ms.On("SetIngested", testutils.AnyContext, realPrimaryBranchFile, mock.Anything).Return(nil)

	ps := pubSubSource{
		IngestionStore:         ms,
		PrimaryBranchProcessor: mp,
		FailedCounter:          nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), realPrimaryBranchFile)
	assert.True(t, shouldAck)
	ms.AssertExpectations(t)
}

func TestPubSubSource_IngestFile_PrimaryBranch_RetryableError_Nack(t *testing.T) {

	const realPrimaryBranchFile = "dm-json-v1/2021/03/02/15/a07ced8f471f8139771d045086aa6e2c2d6746ab/waterfall/dm-1614698630345047867.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realPrimaryBranchFile).Return(true)
	mp.On("Process", testutils.AnyContext, realPrimaryBranchFile).Return(ingestion.ErrRetryable)

	ps := pubSubSource{
		PrimaryBranchProcessor: mp,
		FailedCounter:          nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), realPrimaryBranchFile)
	assert.False(t, shouldAck)
}

func TestPubSubSource_IngestFile_TryjobData_NoErrors_Ack(t *testing.T) {

	const realTryjobFile = "trybot/dm-json-v1/2021/03/02/17/378362__1/8853853547141503920/dm-1614705135861548495.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realTryjobFile).Return(false)
	mtp := &mocks.Processor{}
	mtp.On("HandlesFile", realTryjobFile).Return(true)
	mtp.On("Process", testutils.AnyContext, realTryjobFile).Return(nil)

	ms := &mocks.Store{}
	ms.On("SetIngested", testutils.AnyContext, realTryjobFile, mock.Anything).Return(nil)

	ps := pubSubSource{
		IngestionStore:                   ms,
		PrimaryBranchProcessor:           mp,
		TryjobProcessor:                  mtp,
		SecondaryBranchStreamingLiveness: nopLiveness{},
		SuccessCounter:                   nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), realTryjobFile)
	assert.True(t, shouldAck)
	ms.AssertExpectations(t)
}

func TestPubSubSource_IngestFile_TryjobData_NonRetryableError_Ack(t *testing.T) {

	const realTryjobFile = "trybot/dm-json-v1/2021/03/02/17/378362__1/8853853547141503920/dm-1614705135861548495.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realTryjobFile).Return(false)
	mtp := &mocks.Processor{}
	mtp.On("HandlesFile", realTryjobFile).Return(true)
	mtp.On("Process", testutils.AnyContext, realTryjobFile).Return(errors.New("invalid JSON"))

	ms := &mocks.Store{}
	ms.On("SetIngested", testutils.AnyContext, realTryjobFile, mock.Anything).Return(nil)

	ps := pubSubSource{
		IngestionStore:         ms,
		PrimaryBranchProcessor: mp,
		TryjobProcessor:        mtp,
		FailedCounter:          nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), realTryjobFile)
	assert.True(t, shouldAck)
	ms.AssertExpectations(t)
}

func TestPubSubSource_IngestFile_TryjobData_RetryableError_Nack(t *testing.T) {

	const realTryjobFile = "trybot/dm-json-v1/2021/03/02/17/378362__1/8853853547141503920/dm-1614705135861548495.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realTryjobFile).Return(false)
	mtp := &mocks.Processor{}
	mtp.On("HandlesFile", realTryjobFile).Return(true)
	mtp.On("Process", testutils.AnyContext, realTryjobFile).Return(ingestion.ErrRetryable)

	ps := pubSubSource{
		PrimaryBranchProcessor: mp,
		TryjobProcessor:        mtp,
		FailedCounter:          nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), realTryjobFile)
	assert.False(t, shouldAck)
}

func TestPubSubSource_IngestFile_InvalidFile_Ack(t *testing.T) {

	const unknownFile = "unknownfile.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", unknownFile).Return(false)
	mtp := &mocks.Processor{}
	mtp.On("HandlesFile", unknownFile).Return(false)

	ps := pubSubSource{
		PrimaryBranchProcessor: mp,
		TryjobProcessor:        mtp,
		FailedCounter:          nopCounter{},
	}
	shouldAck := ps.ingestFile(context.Background(), unknownFile)
	assert.True(t, shouldAck)
}

func TestPubSubSource_IngestFile_InvalidFileType_Ack(t *testing.T) {

	const logFile = "verbose.log"

	ps := pubSubSource{}
	shouldAck := ps.ingestFile(context.Background(), logFile)
	assert.True(t, shouldAck)
}

func TestStartBackupPolling_TwoSources_Success(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fakeNow := time.Date(2021, time.March, 3, 4, 5, 6, 0, time.UTC)
	ctx = context.WithValue(ctx, now.ContextKey, fakeNow)

	isc := ingestionServerConfig{
		BackupPollInterval: config.Duration{Duration: time.Hour},
		BackupPollScope:    config.Duration{Duration: 2 * time.Hour},
	}

	mfs1 := &mocks.FileSearcher{}
	// Between fakeNow - 2 hours and fakeNow
	mfs1.On("SearchForFiles", testutils.AnyContext, time.Date(2021, time.March, 3, 2, 5, 6, 0, time.UTC), fakeNow).Return(
		[]string{"file1.json", "file2.json"})
	mfs2 := &mocks.FileSearcher{}
	// Between fakeNow - 2 hours and fakeNow
	mfs2.On("SearchForFiles", testutils.AnyContext, time.Date(2021, time.March, 3, 2, 5, 6, 0, time.UTC), fakeNow).Return(
		[]string{"file3.json"})

	// Pretend file2.json has been ingested, but the other two have not
	mis := &mocks.Store{}
	mis.On("WasIngested", testutils.AnyContext, "file1.json").Return(false, nil)
	mis.On("WasIngested", testutils.AnyContext, "file2.json").Return(true, nil)
	mis.On("WasIngested", testutils.AnyContext, "file3.json").Return(false, nil)

	mis.On("SetIngested", testutils.AnyContext, "file1.json", fakeNow).Return(nil)
	mis.On("SetIngested", testutils.AnyContext, "file3.json", fakeNow).Return(nil)

	mp := &mocks.Processor{}
	mp.On("HandlesFile", mock.Anything).Return(true)
	mp.On("Process", testutils.AnyContext, "file1.json").Return(nil)
	mp.On("Process", testutils.AnyContext, "file3.json").Return(nil)

	ps := &pubSubSource{
		PrimaryBranchProcessor:         mp,
		IngestionStore:                 mis,
		PrimaryBranchStreamingLiveness: nopLiveness{},
		SuccessCounter:                 nopCounter{},
	}

	startBackupPolling(ctx, isc, []ingestion.FileSearcher{mfs1, mfs2}, ps)
	time.Sleep(500 * time.Millisecond)
	cancel()
	time.Sleep(500 * time.Millisecond) // Wait for first round of polling to finish
	mfs1.AssertExpectations(t)
	mfs2.AssertExpectations(t)
	mis.AssertExpectations(t)
	mp.AssertExpectations(t)
}

func TestLoadExistingConfigs_Valid(t *testing.T) {
	var cfg ingestionServerConfig
	err := validation.ValidateServiceConfigs("ingestion", validation.PrimaryInstances, &cfg)
	require.NoError(t, err)
	assert.NotZero(t, cfg, "Config object should not be nil.")
}

// nopLiveness is a no-op metrics2.Liveness implementation to fake out during tests.
type nopLiveness struct{}

func (n nopLiveness) Get() int64 { return 0 }

func (n nopLiveness) ManualReset(_ time.Time) {}

func (n nopLiveness) Reset() {}

func (n nopLiveness) Close() {}

// Ensure that nopLiveness implements the Liveness interface.
var _ metrics2.Liveness = (*nopLiveness)(nil)

type nopCounter struct{}

func (n nopCounter) Dec(_ int64) {}

func (n nopCounter) Delete() error { return nil }

func (n nopCounter) Get() int64 { return 0 }

func (n nopCounter) Inc(_ int64) {}

func (n nopCounter) Reset() {}

// Ensure that nopLiveness implements the Liveness interface.
var _ metrics2.Counter = (*nopCounter)(nil)
