package format

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte("{")))
	assert.Error(t, err)
}

func TestParse_GoodVersion(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte("{\"version\":1}")))
	assert.NoError(t, err)
}

func TestParse_BadVersion(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte("{\"version\":2}")))
	assert.Error(t, err)
}

func TestParse_BadVersionNotNumber(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte("{\"version\":\"1\"}")))
	assert.Error(t, err)
}

func TestValidate_EmptyObject_ReturnsError(t *testing.T) {
	r := strings.NewReader("{}")
	_, err := Validate(context.Background(), r)
	require.Error(t, err)
}

func TestValidate_VersionOnlyIsCorrect_ReturnsError(t *testing.T) {
	r := strings.NewReader(`{"version" : 1}`)
	schemaViolations, err := Validate(context.Background(), r)
	require.Error(t, err)
	require.NotEmpty(t, schemaViolations)
}

func TestValidate_MinimumValidVersion_Success(t *testing.T) {
	r := strings.NewReader(`{
		"version" : 1,
		"git_hash": "1234567890",
		"results" : []
		}`)
	schemaViolations, err := Validate(context.Background(), r)
	require.NoError(t, err)
	require.Empty(t, schemaViolations)
}

func TestValidate_ExampleWithData_Success(t *testing.T) {
	r := strings.NewReader(`{
		"version": 1,
		"git_hash": "cd5...663",
		"key": {
			"config": "8888",
			"arch": "x86"
		},
		"results": [
			{
				"key": {
					"test": "some_test_name"
				},
				"measurements": {
					"ms": [
						{
							"value": "min",
							"measurement": 1.2
						},
						{
							"value": "max",
							"measurement": 2.4
						},
						{
							"value": "median",
							"measurement": 1.5
						}
					]
				}
			}
		]
	}`)
	schemaViolations, err := Validate(context.Background(), r)
	require.NoError(t, err)
	require.Empty(t, schemaViolations)
}
