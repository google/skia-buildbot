package sklog

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestLineRef(t *testing.T) {
	assert.Equal(t, "sklog_test.go:10", LineRef())
}
