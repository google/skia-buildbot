package influxdb

import (
	"reflect"
	"testing"
)

type singleInt struct {
	Value int `influxdb:"value"`
}

type singleInt8 struct {
	Value int8 `influxdb:"value"`
}

type singleInt16 struct {
	Value int16 `influxdb:"value"`
}

type singleInt32 struct {
	Value int32 `influxdb:"value"`
}

type singleInt64 struct {
	Value int64 `influxdb:"value"`
}

type singleBool struct {
	Value bool `influxdb:"value"`
}

type singleFloat32 struct {
	Value float32 `influxdb:"value"`
}

type singleFloat64 struct {
	Value float64 `influxdb:"value"`
}

type singleString struct {
	Value string `influxdb:"value"`
}

type singleUint struct {
	Value uint `influxdb:"value"`
}

type singleUint8 struct {
	Value uint8 `influxdb:"value"`
}

type singleUint16 struct {
	Value uint16 `influxdb:"value"`
}

type singleUint32 struct {
	Value uint32 `influxdb:"value"`
}

type singleUint64 struct {
	Value uint64 `influxdb:"value"`
}

type singleRune struct {
	Value rune `influxdb:"value"`
}

func TestEncodeDecode(t *testing.T) {
	testCases := []struct {
		test   interface{}
		result interface{}
	}{
		{&singleInt{Value: 42}, &singleInt{}},
		{&singleInt8{Value: 42}, &singleInt8{}},
		{&singleInt16{Value: 42}, &singleInt16{}},
		{&singleInt32{Value: 42}, &singleInt32{}},
		{&singleInt64{Value: 42}, &singleInt64{}},
		{&singleBool{Value: true}, &singleBool{}},
		{&singleFloat32{Value: 3.14159}, &singleFloat32{}},
		{&singleFloat64{Value: 3.14159}, &singleFloat64{}},
		{&singleString{Value: "blahblah"}, &singleString{}},
		{&singleUint{Value: 42}, &singleUint{}},
		{&singleUint8{Value: 42}, &singleUint8{}},
		{&singleUint16{Value: 42}, &singleUint16{}},
		{&singleUint32{Value: 42}, &singleUint32{}},
		{&singleUint64{Value: 42}, &singleUint64{}},
		{&singleRune{Value: 'r'}, &singleRune{}},
	}
	for _, tc := range testCases {
		s, err := structToSeries(tc.test, "dummyseries")
		if err != nil {
			t.Fatal(err)
		}
		if err := seriesToStruct(tc.result, s); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tc.test, tc.result) {
			t.Fatalf("Expected: \n%v\nGot: \n%v", tc.test, tc.result)
		}
	}
}
