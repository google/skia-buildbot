package sql

import (
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
	assert.Equal(t, schema.Digest{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, actualBytes)
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
