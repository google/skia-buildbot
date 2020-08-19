package sql

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
)

func TestValuesPlaceholders_ValidInputs_Success(t *testing.T) {
	unittest.SmallTest(t)

	v, err := ValuesPlaceholders(3, 2)
	require.NoError(t, err)
	assert.Equal(t, "($1,$2,$3),($4,$5,$6)", v)

	v, err = ValuesPlaceholders(2, 4)
	require.NoError(t, err)
	assert.Equal(t, "($1,$2),($3,$4),($5,$6),($7,$8)", v)

	v, err = ValuesPlaceholders(1, 1)
	require.NoError(t, err)
	assert.Equal(t, "($1)", v)

	v, err = ValuesPlaceholders(1, 3)
	require.NoError(t, err)
	assert.Equal(t, "($1),($2),($3)", v)
}

func TestValuesPlaceholders_InvalidInputs_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := ValuesPlaceholders(-3, 2)
	assert.Error(t, err)

	_, err = ValuesPlaceholders(2, -4)
	assert.Error(t, err)

	_, err = ValuesPlaceholders(0, 0)
	assert.Error(t, err)
}

func TestInPlaceholders_ValidInputs_Success(t *testing.T) {
	unittest.SmallTest(t)

	v, err := InPlaceholders(1, 4)
	require.NoError(t, err)
	assert.Equal(t, "($1,$2,$3,$4)", v)

	v, err = InPlaceholders(5, 3)
	require.NoError(t, err)
	assert.Equal(t, "($5,$6,$7)", v)

	v, err = InPlaceholders(1, 1)
	require.NoError(t, err)
	assert.Equal(t, "($1)", v)
}

func TestInPlaceholders_InvalidInputs_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := InPlaceholders(-3, 2)
	assert.Error(t, err)

	_, err = InPlaceholders(2, -4)
	assert.Error(t, err)

	_, err = InPlaceholders(0, 0)
	assert.Error(t, err)
}

func TestSerializeMap_Success(t *testing.T) {
	unittest.SmallTest(t)

	mJSON, expectedHash, err := SerializeMap(map[string]string{
		"opt": "png",
	})
	require.NoError(t, err)
	assert.Equal(t, `{"opt":"png"}`, mJSON)
	assert.Equal(t, "5869c277f132c3d777ebd81d01f694fc", hex.EncodeToString(expectedHash))

	mJSON, expectedHash, err = SerializeMap(map[string]string{
		"this": "should",
		"be":   "realphabetized",
		"by":   "key",
		"when": "turned into json",
	})
	require.NoError(t, err)
	assert.Equal(t, `{"be":"realphabetized","by":"key","this":"should","when":"turned into json"}`, mJSON)
	assert.Equal(t, "d3f07017f2f702c8337767343860703b", hex.EncodeToString(expectedHash))

	mJSON, expectedHash, err = SerializeMap(map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, `{}`, mJSON)
	assert.Equal(t, "99914b932bd37a50b983c5e7c90ae93b", hex.EncodeToString(expectedHash))

	// As a special case, we expect nil maps to be treated as empty maps.
	mJSON, expectedHash, err = SerializeMap(nil)
	require.NoError(t, err)
	assert.Equal(t, `{}`, mJSON)
	assert.Equal(t, "99914b932bd37a50b983c5e7c90ae93b", hex.EncodeToString(expectedHash))
}

func TestComputeTraceValueShard_Success(t *testing.T) {
	unittest.SmallTest(t)

	shard := ComputeTraceValueShard([]byte{0xed, 0x12})
	assert.Equal(t, []byte{0x05}, shard)
}

func TestDigestToBytes_Success(t *testing.T) {
	unittest.SmallTest(t)

	actualBytes, err := DigestToBytes("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, actualBytes)
}

func TestDigestToBytes_MissingDigest_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := DigestToBytes("")
	require.Error(t, err)
}

func TestConvertLabelFromString_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, LabelUntriaged, ConvertLabelFromString(expectations.Untriaged))
	assert.Equal(t, LabelPositive, ConvertLabelFromString(expectations.Positive))
	assert.Equal(t, LabelNegative, ConvertLabelFromString(expectations.Negative))
}

func TestConvertIgnoreRules(t *testing.T) {
	unittest.SmallTest(t)

	condition, args := ConvertIgnoreRules(nil)
	assert.Equal(t, "false", condition)
	assert.Empty(t, args)

	condition, args = ConvertIgnoreRules([]paramtools.ParamSet{
		{
			"key1": []string{"alpha"},
		},
	})
	assert.Equal(t, `((keys ->> $1::STRING IN ($2)))`, condition)
	assert.Equal(t, []interface{}{"key1", "alpha"}, args)

	condition, args = ConvertIgnoreRules([]paramtools.ParamSet{
		{
			"key1": []string{"alpha", "beta"},
			"key2": []string{"gamma"},
		},
		{
			"key3": []string{"delta", "epsilon", "zeta"},
		},
	})
	const expectedCondition = `((keys ->> $1::STRING IN ($2, $3) AND keys ->> $4::STRING IN ($5))
OR (keys ->> $6::STRING IN ($7, $8, $9)))`
	assert.Equal(t, expectedCondition, condition)
	assert.Equal(t, []interface{}{"key1", "alpha", "beta", "key2", "gamma", "key3", "delta", "epsilon", "zeta"}, args)
}
