package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterfaces(t *testing.T) {
	SmallTest(t)

	// Ensure that our interfaces are compatible.
	var _ assert.TestingT = TestingT(nil)
	var _ TestingT = (*testing.T)(nil)
	var _ TestingT = (*testing.B)(nil)
}
