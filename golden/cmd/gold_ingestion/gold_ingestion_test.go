package main

import (
	"context"
	"errors"
	"testing"

	"go.skia.org/infra/golden/go/ingestion"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ingestion/mocks"
)

func TestPubSubSource_IngestFile_PrimaryBranch_NoErrors_Ack(t *testing.T) {
	unittest.SmallTest(t)

	const realPrimaryBranchFile = "dm-json-v1/2021/03/02/15/a07ced8f471f8139771d045086aa6e2c2d6746ab/waterfall/dm-1614698630345047867.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realPrimaryBranchFile).Return(true)
	mp.On("Process", testutils.AnyContext, realPrimaryBranchFile).Return(nil)

	ps := pubSubSource{
		PrimaryBranchProcessor: mp,
	}

	ctx := context.Background()
	// This is a real-world file path
	shouldAck := ps.ingestFile(ctx, realPrimaryBranchFile)
	assert.True(t, shouldAck)
}

func TestPubSubSource_IngestFile_PrimaryBranch_NonRetryableError_Ack(t *testing.T) {
	unittest.SmallTest(t)

	const realPrimaryBranchFile = "dm-json-v1/2021/03/02/15/a07ced8f471f8139771d045086aa6e2c2d6746ab/waterfall/dm-1614698630345047867.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realPrimaryBranchFile).Return(true)
	mp.On("Process", testutils.AnyContext, realPrimaryBranchFile).Return(errors.New("invalid JSON"))

	ps := pubSubSource{
		PrimaryBranchProcessor: mp,
	}

	ctx := context.Background()
	// This is a real-world file path
	shouldAck := ps.ingestFile(ctx, realPrimaryBranchFile)
	assert.True(t, shouldAck)
}

func TestPubSubSource_IngestFile_PrimaryBranch_RetryableError_Nack(t *testing.T) {
	unittest.SmallTest(t)

	const realPrimaryBranchFile = "dm-json-v1/2021/03/02/15/a07ced8f471f8139771d045086aa6e2c2d6746ab/waterfall/dm-1614698630345047867.json"

	mp := &mocks.Processor{}
	mp.On("HandlesFile", realPrimaryBranchFile).Return(true)
	mp.On("Process", testutils.AnyContext, realPrimaryBranchFile).Return(ingestion.ErrRetryable)

	ps := pubSubSource{
		PrimaryBranchProcessor: mp,
	}

	ctx := context.Background()
	// This is a real-world file path
	shouldAck := ps.ingestFile(ctx, realPrimaryBranchFile)
	assert.False(t, shouldAck)
}
