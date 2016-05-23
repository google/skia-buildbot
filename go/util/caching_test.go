package util

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestJSONCodec(t *testing.T) {
	itemCodec := JSONCodec(&myTestType{})
	testInstance := &myTestType{5, "hello"}
	jsonBytes, err := itemCodec.Encode(testInstance)
	assert.NoError(t, err)

	decodedInstance, err := itemCodec.Decode(jsonBytes)
	assert.NoError(t, err)
	assert.IsType(t, &myTestType{}, decodedInstance)
	assert.Equal(t, testInstance, decodedInstance)

	arrCodec := JSONCodec([]*myTestType{})
	testArr := []*myTestType{&myTestType{1, "1"}, &myTestType{2, "2"}}
	jsonBytes, err = arrCodec.Encode(testArr)
	assert.NoError(t, err)

	decodedArr, err := arrCodec.Decode(jsonBytes)
	assert.NoError(t, err)
	assert.IsType(t, []*myTestType{}, decodedArr)
	assert.Equal(t, testArr, decodedArr)
}

func TestMemLRUCache(t *testing.T) {
	cache := NewMemLRUCache(0)
	UnitTestLRUCache(t, cache)
}
