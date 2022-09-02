package sql

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/golden/go/sql/schema"
)

func TestDigestToBytes_Success(t *testing.T) {

	actualBytes, err := DigestToBytes("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, schema.DigestBytes{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, actualBytes)
}

func TestDigestToBytes_MissingDigest_ReturnsError(t *testing.T) {

	_, err := DigestToBytes("")
	require.Error(t, err)
}

func TestDigestToBytes_InvalidCharacters_ReturnsError(t *testing.T) {

	_, err := DigestToBytes("ZZZZZZZZ89abcdef0123456789abcdef")
	require.Error(t, err)
}

func TestDigestToBytes_InvalidLength_ReturnsError(t *testing.T) {

	_, err := DigestToBytes("abcdef")
	require.Error(t, err)
}

func TestSerializeMap_Success(t *testing.T) {

	mJSON, expectedHash := SerializeMap(map[string]string{
		"opt": "png",
	})
	assert.Equal(t, `{"opt":"png"}`, mJSON)
	assert.Equal(t, "5869c277f132c3d777ebd81d01f694fc", hex.EncodeToString(expectedHash[:]))

	mJSON, expectedHash = SerializeMap(map[string]string{
		"this": "should",
		"be":   "realphabetized",
		"by":   "key",
		"when": "turned into json",
	})
	assert.Equal(t, `{"be":"realphabetized","by":"key","this":"should","when":"turned into json"}`, mJSON)
	assert.Equal(t, "d3f07017f2f702c8337767343860703b", hex.EncodeToString(expectedHash[:]))

	mJSON, expectedHash = SerializeMap(map[string]string{})
	assert.Equal(t, `{}`, mJSON)
	assert.Equal(t, "99914b932bd37a50b983c5e7c90ae93b", hex.EncodeToString(expectedHash[:]))

	// As a special case, we expect nil maps to be treated as empty maps.
	mJSON, expectedHash = SerializeMap(nil)
	assert.Equal(t, `{}`, mJSON)
	assert.Equal(t, "99914b932bd37a50b983c5e7c90ae93b", hex.EncodeToString(expectedHash[:]))
}

func TestComputeTraceValueShard_Success(t *testing.T) {

	assert.Equal(t, byte(0x05), ComputeTraceValueShard(schema.TraceID{0xed, 0x12}))
	assert.Equal(t, byte(0x03), ComputeTraceValueShard(schema.TraceID{0x13, 0x14}))
}

func TestAsMD5Hash_Success(t *testing.T) {

	db, err := DigestToBytes("aaaabbbbccccddddeeeeffff00001111")
	require.NoError(t, err)
	assert.Equal(t, schema.MD5Hash{
		0xaa, 0xaa, 0xbb, 0xbb, 0xcc, 0xcc, 0xdd, 0xdd, 0xee, 0xee, 0xff, 0xff, 0x00, 0x00, 0x11, 0x11,
	}, AsMD5Hash(db))
}

func TestFromMD5Hash_Success(t *testing.T) {
	someHashes := []schema.MD5Hash{
		{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
		{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22},
		{0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33},
	}
	var someGroupingIDs []schema.GroupingID
	for _, h := range someHashes {
		// This test fails the the following naive code.
		// someGroupingIDs = append(someGroupingIDs, h[:])
		someGroupingIDs = append(someGroupingIDs, FromMD5Hash(h))
	}
	assert.Equal(t, []schema.GroupingID{
		{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
		{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22},
		{0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33},
	}, someGroupingIDs)
}

func TestValuesPlaceholders_ValidInputs_Success(t *testing.T) {

	v := ValuesPlaceholders(3, 2)
	assert.Equal(t, "($1,$2,$3),($4,$5,$6)", v)

	v = ValuesPlaceholders(2, 4)
	assert.Equal(t, "($1,$2),($3,$4),($5,$6),($7,$8)", v)

	v = ValuesPlaceholders(1, 1)
	assert.Equal(t, "($1)", v)

	v = ValuesPlaceholders(1, 3)
	assert.Equal(t, "($1),($2),($3)", v)
}

func TestValuesPlaceholders_InvalidInputs_Panics(t *testing.T) {

	assert.Panics(t, func() {
		ValuesPlaceholders(-3, 2)
	})
	assert.Panics(t, func() {
		ValuesPlaceholders(2, -4)
	})
	assert.Panics(t, func() {
		ValuesPlaceholders(0, 0)
	})
}

func TestQualify_Success(t *testing.T) {

	assert.Equal(t, "gerrit_12345", Qualify("gerrit", "12345"))
	assert.Equal(t, "gerrit-internal_6789012", Qualify("gerrit-internal", "6789012"))
}

func TestUnqualify_Success(t *testing.T) {

	assert.Equal(t, "12345", Unqualify("gerrit_12345"))
	assert.Equal(t, "6789012", Unqualify("gerrit-internal_6789012"))
	assert.Equal(t, "1234_6789012", Unqualify("gerrit_1234_6789012"))
	assert.Equal(t, "67890", Unqualify("67890"))
}

func TestSanitize_Success(t *testing.T) {

	assert.Equal(t, "All Good!", Sanitize(`All Good!`))
	assert.Equal(t, "foo OR 1=1", Sanitize(`foo OR '1=1'`))
	assert.Equal(t, "foo OR 1=1", Sanitize(`foo OR "1=1"`))
}
