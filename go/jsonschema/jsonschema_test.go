package jsonschema

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGenerateSchema_NotAnInstanceThatCanBeSerialized_Panics(t *testing.T) {
	unittest.SmallTest(t)
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
