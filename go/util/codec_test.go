package util

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

type myTestType struct {
	A int
	B string
}

func TestJSONCodec(t *testing.T) {
	itemCodec := NewJSONCodec(&myTestType{})
	testInstance := &myTestType{5, "hello"}
	jsonBytes, err := itemCodec.Encode(testInstance)
	require.NoError(t, err)

	decodedInstance, err := itemCodec.Decode(jsonBytes)
	require.NoError(t, err)
	require.IsType(t, &myTestType{}, decodedInstance)
	require.Equal(t, testInstance, decodedInstance)

	arrCodec := NewJSONCodec([]*myTestType{})
	testArr := []*myTestType{{1, "1"}, {2, "2"}}
	jsonBytes, err = arrCodec.Encode(testArr)
	require.NoError(t, err)

	decodedArr, err := arrCodec.Decode(jsonBytes)
	require.NoError(t, err)
	require.IsType(t, []*myTestType{}, decodedArr)
	require.Equal(t, testArr, decodedArr)

	mapCodec := NewJSONCodec(map[string]map[string]int{})
	testMap := map[string]map[string]int{"hello": {"world": 55}}
	jsonBytes, err = mapCodec.Encode(testMap)
	require.NoError(t, err)
	found, err := mapCodec.Decode(jsonBytes)
	require.NoError(t, err)
	assertdeep.Equal(t, testMap, found)
}
