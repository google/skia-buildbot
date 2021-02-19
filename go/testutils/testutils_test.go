package testutils

import (
	"testing"

	"go.skia.org/infra/go/sktest"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestInterfaces(t *testing.T) {
	unittest.SmallTest(t)

	// Ensure that our interfaces are compatible.
	var _ assert.TestingT = sktest.TestingT(nil)
	var _ sktest.TestingT = (*testing.T)(nil)
	var _ sktest.TestingT = (*testing.B)(nil)
}
