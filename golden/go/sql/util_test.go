package sql

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/schema"
)

func TestDigestToBytes_Success(t *testing.T) {
	unittest.SmallTest(t)

	actualBytes, err := DigestToBytes("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, schema.DigestBytes{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, actualBytes)
}

func TestDigestToBytes_MissingDigest_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := DigestToBytes("")
	require.Error(t, err)
}

func TestDigestToBytes_InvalidCharacters_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := DigestToBytes("ZZZZZZZZ89abcdef0123456789abcdef")
	require.Error(t, err)
}

func TestDigestToBytes_InvalidLength_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := DigestToBytes("abcdef")
	require.Error(t, err)
}

func TestSerializeMap_Success(t *testing.T) {
	unittest.SmallTest(t)

	mJSON, expectedHash := SerializeMap(map[string]string{
		"opt": "png",
	})
	assert.Equal(t, schema.SerializedJSON(`{"opt":"png"}`), mJSON)
	assert.Equal(t, "5869c277f132c3d777ebd81d01f694fc", hex.EncodeToString(expectedHash[:]))

	mJSON, expectedHash = SerializeMap(map[string]string{
		"this": "should",
		"be":   "realphabetized",
		"by":   "key",
		"when": "turned into json",
	})
	assert.Equal(t, schema.SerializedJSON(`{"be":"realphabetized","by":"key","this":"should","when":"turned into json"}`), mJSON)
	assert.Equal(t, "d3f07017f2f702c8337767343860703b", hex.EncodeToString(expectedHash[:]))

	mJSON, expectedHash = SerializeMap(map[string]string{})
	assert.Equal(t, schema.SerializedJSON(`{}`), mJSON)
	assert.Equal(t, "99914b932bd37a50b983c5e7c90ae93b", hex.EncodeToString(expectedHash[:]))

	// As a special case, we expect nil maps to be treated as empty maps.
	mJSON, expectedHash = SerializeMap(nil)
	assert.Equal(t, schema.SerializedJSON(`{}`), mJSON)
	assert.Equal(t, "99914b932bd37a50b983c5e7c90ae93b", hex.EncodeToString(expectedHash[:]))
}

func TestComputeTraceValueShard_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, byte(0x05), ComputeTraceValueShard(schema.TraceID{0xed, 0x12}))
	assert.Equal(t, byte(0x03), ComputeTraceValueShard(schema.TraceID{0x13, 0x14}))
}
