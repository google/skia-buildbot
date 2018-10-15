package util

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestJSONCodec(t *testing.T) {
	testutils.SmallTest(t)
	itemCodec := JSONCodec(&myTestType{})
	testInstance := &myTestType{5, "hello"}
	jsonBytes, err := itemCodec.Encode(testInstance)
	assert.NoError(t, err)

	decodedInstance, err := itemCodec.Decode(jsonBytes)
	assert.NoError(t, err)
	assert.IsType(t, &myTestType{}, decodedInstance)
	assert.Equal(t, testInstance, decodedInstance)

	arrCodec := JSONCodec([]*myTestType{})
	testArr := []*myTestType{{1, "1"}, {2, "2"}}
	jsonBytes, err = arrCodec.Encode(testArr)
	assert.NoError(t, err)

	decodedArr, err := arrCodec.Decode(jsonBytes)
	assert.NoError(t, err)
	assert.IsType(t, []*myTestType{}, decodedArr)
	assert.Equal(t, testArr, decodedArr)

	mapCodec := JSONCodec(map[string]map[string]int{})
	testMap := map[string]map[string]int{"hello": {"world": 55}}
	jsonBytes, err = mapCodec.Encode(testMap)
	assert.NoError(t, err)
	found, err := mapCodec.Decode(jsonBytes)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, testMap, found)
}

func TestMemLRUCache(t *testing.T) {
	testutils.SmallTest(t)
	cache := NewMemLRUCache(0)
	UnitTestLRUCache(t, cache)
}
