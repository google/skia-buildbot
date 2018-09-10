package goldingestion

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	empty := &GoldResults{}
	fields, err := empty.Validate()
	assert.Error(t, err)
	assertErrorFields(t, fields,
		"gitHash",
		"key",
		"results")
	assert.NotNil(t, fields)

	wrongResults := &GoldResults{
		GitHash: "a1b2c3d4e5f6a7b8c9d0e1f2",
		Key:     map[string]string{"param1": "value1"},
	}
	fields, err = wrongResults.Validate()
	assert.Error(t, err)
	assertErrorFields(t, fields, "results")

	wrongResults.Results = []*Result{}
	fields, err = wrongResults.Validate()
	assert.Error(t, err)
	assertErrorFields(t, fields, "results")

	wrongResults.Results = []*Result{
		&Result{Key: map[string]string{}},
	}
	fields, err = wrongResults.Validate()
	assert.Error(t, err)
	assertErrorFields(t, fields, "results")
}

func assertErrorFields(t *testing.T, fields map[string]string, expectedFields ...string) {
	for _, f := range expectedFields {
		_, ok := fields[f]
		assert.True(t, ok, fmt.Sprintf("Expected %s to be in error fields", f))
	}
}
