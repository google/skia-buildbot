package testutils

import (
	"testing"

	"go.skia.org/infra/go/sktest"

	"github.com/stretchr/testify/assert"
)

func TestInterfaces(t *testing.T) {

	// Ensure that our interfaces are compatible.
	var _ assert.TestingT = sktest.TestingT(nil)
	var _ sktest.TestingT = (*testing.T)(nil)
	var _ sktest.TestingT = (*testing.B)(nil)
}
