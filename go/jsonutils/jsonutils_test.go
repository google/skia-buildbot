package jsonutils

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNumber(t *testing.T) {
	unittest.SmallTest(t)
	type testCase struct {
		in  string
		out int64
		err string
	}
	cases := []testCase{
		{
			in:  "abc",
			out: 0,
			err: "invalid character 'a' looking for beginning of value",
		},
		{
			in:  "0",
			out: 0,
			err: "",
		},
		{
			in:  "\"0\"",
			out: 0,
			err: "",
		},
		{
			in:  "9.345",
			out: 0,
			err: "parsing \"9.345\": invalid syntax",
		},
		{
			in:  "2147483647",
			out: 2147483647,
			err: "",
		},
		{
			in:  "2147483648",
			out: 2147483648,
			err: "",
		},
		{
			in:  "9223372036854775807",
			out: 9223372036854775807,
			err: "",
		},
		{
			in:  "\"9223372036854775807\"",
			out: 9223372036854775807,
			err: "",
		},
		{
			in:  "9223372036854775808",
			out: 0,
			err: "parsing \"9223372036854775808\": value out of range",
		},
	}
	for _, tc := range cases {
		var got Number
		err := json.Unmarshal([]byte(tc.in), &got)
		if tc.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.out, int64(got))
		}
	}
}

func TestTime(t *testing.T) {
	unittest.SmallTest(t)
	type testCase struct {
		in  time.Time
		out string
	}
	cases := []testCase{
		{
			in:  time.Unix(0, 0),
			out: "0",
		},
		{
			in:  time.Unix(1475508449, 0),
			out: "1475508449000000",
		},
	}
	for _, tc := range cases {
		inp := Time(tc.in)
		b, err := json.Marshal(&inp)
		require.NoError(t, err)
		require.Equal(t, []byte(tc.out), b)
		var got Time
		err = json.Unmarshal(b, &got)
		require.NoError(t, err)
		require.Equal(t, tc.in.UTC(), time.Time(got).UTC())
	}
}
func TestMarshalStringMap_NonEmptyMap_MatchesBuiltInImpl(t *testing.T) {
	unittest.MediumTest(t)
	input := map[string]string{}
	testutils.ReadJSONFile(t, "mediumparams.json", &input)
	require.Len(t, input, 50)
	actual := MarshalStringMap(input)
	expected, err := json.Marshal(input)
	require.NoError(t, err)
	assert.Equal(t, expected, actual, "%s != %s", string(expected), string(actual))

	input2 := map[string]string{}
	testutils.ReadJSONFile(t, "smallparams.json", &input2)
	require.Len(t, input2, 10)
	actual = MarshalStringMap(input2)
	expected, err = json.Marshal(input2)
	require.NoError(t, err)
	assert.Equal(t, expected, actual, "%s != %s", string(expected), string(actual))
}

func TestMarshalStringMap_EmptyMap_MatchesBuiltInImpl(t *testing.T) {
	unittest.SmallTest(t)
	input := map[string]string{}
	actual := MarshalStringMap(input)
	expected, err := json.Marshal(input)
	require.NoError(t, err)
	assert.Equal(t, expected, actual, "%s != %s", string(expected), string(actual))
}

func TestMarshalStringMap_NilMap_MatchesBuiltInImpl(t *testing.T) {
	unittest.SmallTest(t)
	var input map[string]string
	actual := MarshalStringMap(input)
	expected, err := json.Marshal(input)
	require.NoError(t, err)
	assert.Equal(t, expected, actual, "%s != %s", string(expected), string(actual))
}

func BenchmarkMarshalStringMap_10Items(b *testing.B) {
	input := map[string]string{}
	testutils.ReadJSONFile(b, "smallparams.json", &input)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b := MarshalStringMap(input)
		if b[0] == 'N' {
			panic("this is to keep the call from being optimized out")
		}
	}
}

func BenchmarkMarshalStringMap_50Items(b *testing.B) {
	input := map[string]string{}
	testutils.ReadJSONFile(b, "mediumparams.json", &input)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b := MarshalStringMap(input)
		if b[0] == 'N' {
			panic("this is to keep the call from being optimized out")
		}
	}
}

func BenchmarkBuiltInJSONMarshal_10Items(b *testing.B) {
	input := map[string]string{}
	testutils.ReadJSONFile(b, "smallparams.json", &input)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b, _ := json.Marshal(input)
		if b[0] == 'N' {
			panic("this is to keep the call from being optimized out")
		}
	}
}

func BenchmarkBuiltInJSONMarshal_50Items(b *testing.B) {
	input := map[string]string{}
	testutils.ReadJSONFile(b, "mediumparams.json", &input)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b, _ := json.Marshal(input)
		if b[0] == 'N' {
			panic("this is to keep the call from being optimized out")
		}
	}
}

func TestGenerateSchema_NotAnInstanceThatCanBeSerialized_Panics(t *testing.T) {
	require.Panics(t, func() {
		GenerateSchema("/tmp/not-used.json", struct{ ComplexValuesAreNotValidJSON complex128 }{})
	})
}

type testStruct struct {
	A int64
}

const testStructSchema = `{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "$ref": "#/definitions/testStruct",
  "definitions": {
    "testStruct": {
      "required": [
        "A"
      ],
      "properties": {
        "A": {
          "type": "integer"
        }
      },
      "additionalProperties": false,
      "type": "object"
    }
  }
}`

func TestGenerateSchema_ValidStruct_WritesSchemaFile(t *testing.T) {
	unittest.MediumTest(t)
	filename := filepath.Join(t.TempDir(), "schema.json")
	GenerateSchema(filename, testStruct{})
	b, err := ioutil.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, testStructSchema, string(b))
}

func TestValidate_JSONConformsToSchema_Success(t *testing.T) {
	unittest.SmallTest(t)
	schemaViolations, err := Validate(context.Background(), []byte(`{"A": 12}`), []byte(testStructSchema))
	require.NoError(t, err)
	require.Empty(t, schemaViolations)
}

func TestValidate_JSONDoesNotConformsToSchema_Success(t *testing.T) {
	unittest.SmallTest(t)
	schemaViolations, err := Validate(context.Background(), []byte(`{"B": 12}`), []byte(testStructSchema))
	require.Error(t, err)
	require.Len(t, schemaViolations, 2)
	require.Contains(t, schemaViolations[0], "A is required")
	require.Contains(t, schemaViolations[1], "Additional property B is not allowed")
}
