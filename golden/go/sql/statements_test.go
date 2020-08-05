package sql

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_SerializeMap_Success(t *testing.T) {
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
}

func Test_DigestToBytes_Success(t *testing.T) {
	unittest.SmallTest(t)

	actualBytes, err := DigestToBytes("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, actualBytes)
}

func Test_DigestToBytes_MissingDigest_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := DigestToBytes("")
	require.Error(t, err)
}

func Test_ConvertLabelFromString_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, LabelUntriaged, ConvertLabelFromString(expectations.Untriaged))
	assert.Equal(t, LabelPositive, ConvertLabelFromString(expectations.Positive))
	assert.Equal(t, LabelNegative, ConvertLabelFromString(expectations.Negative))
}
