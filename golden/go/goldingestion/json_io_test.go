package goldingestion

import (
	"fmt"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	empty := &GoldResults{}
	errMsgs, err := empty.Validate()
	assert.Error(t, err)
	assertErrorFields(t, errMsgs,
		"gitHash",
		"key",
		"results")
	assert.NotNil(t, errMsgs)

	wrongResults := &GoldResults{
		GitHash: "a1b2c3d4e5f6a7b8c9d0e1f2",
		Key:     map[string]string{"param1": "value1"},
	}
	errMsgs, err = wrongResults.Validate()
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")

	wrongResults.Results = []*Result{}
	errMsgs, err = wrongResults.Validate()
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")

	wrongResults.Results = []*Result{
		&Result{Key: map[string]string{}},
	}
	errMsgs, err = wrongResults.Validate()
	assert.Error(t, err)
	assertErrorFields(t, errMsgs, "results")
}

func assertErrorFields(t *testing.T, errMsgs []string, expectedFields ...string) {
	for _, msg := range errMsgs {
		found := false
		for _, ef := range expectedFields {
			found = found || strings.Contains(msg, ef)
		}
		assert.True(t, found, fmt.Sprintf("Could not find %v in msg: %s", expectedFields, msg))
	}
}
