package format

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
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
