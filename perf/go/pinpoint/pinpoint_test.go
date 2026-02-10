package pinpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractErrorMessageReturnsErrorMessage(t *testing.T) {
	input := []byte(`{"error": "something went wrong"}`)
	expected := "something went wrong"
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestExtractErrorMessageReturnsRawString(t *testing.T) {
	input := []byte(`Internal Server Error`)
	expected := "Internal Server Error"
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestExtractErrorMessageReturnsRawStringWhenEmptyError(t *testing.T) {
	input := []byte(`{"error": ""}`)
	expected := `{"error": ""}`
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestExtractErrorMessageReturnsRawStringWhenNoError(t *testing.T) {
	input := []byte(`{"message": "some error"}`)
	expected := `{"message": "some error"}`
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}
