package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff/mocks"
	"go.skia.org/infra/golden/go/types"
)

func TestProcessPubSubMessage_ValidJSON_CalculateSucceeds_Ack(t *testing.T) {
	unittest.SmallTest(t)

	mc := mocks.Calculator{}

	expectedGrouping := paramtools.Params{
		types.CorpusField:     "any-corpus",
		types.PrimaryKeyField: "any-test",
		"other grouping":      "something",
	}
	expectedDigests := []types.Digest{"abcd", "ef123"}

	mc.On("CalculateDiffs", testutils.AnyContext, expectedGrouping, expectedDigests).Return(nil)

	p := processor{calculator: &mc}

	messageBytes := []byte(`{"grouping":{"name":"any-test","other grouping":"something","source_type":"any-corpus"},"additional_digests":["abcd","ef123"]}`)
	shouldAck := p.processMessage(context.Background(), messageBytes)
	assert.True(t, shouldAck)
	mc.AssertExpectations(t)
}

func TestProcessPubSubMessage_ValidJSON_CalculateFails_Nack(t *testing.T) {
	unittest.SmallTest(t)

	mc := mocks.Calculator{}

	expectedGrouping := paramtools.Params{
		types.CorpusField:     "any-corpus",
		types.PrimaryKeyField: "any-test",
	}
	var noExpectedDigests []types.Digest

	mc.On("CalculateDiffs", testutils.AnyContext, expectedGrouping, noExpectedDigests).Return(skerr.Fmt("boom"))

	p := processor{calculator: &mc}

	messageBytes := []byte(`{"grouping":{"name":"any-test","source_type":"any-corpus"}}`)
	shouldAck := p.processMessage(context.Background(), messageBytes)
	assert.False(t, shouldAck)
	mc.AssertExpectations(t)
}

func TestProcessPubSubMessage_InvalidJSON_Ack(t *testing.T) {
	unittest.SmallTest(t)

	p := processor{}
	messageBytes := []byte(`invalid json`)
	shouldAck := p.processMessage(context.Background(), messageBytes)
	assert.True(t, shouldAck)
}
