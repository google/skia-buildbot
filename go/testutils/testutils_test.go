package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/sktest"
)

func TestInterfaces(t *testing.T) {
	SmallTest(t)

	// Ensure that our interfaces are compatible.
	var _ assert.TestingT = sktest.TestingT(nil)
	var _ sktest.TestingT = (*testing.T)(nil)
	var _ sktest.TestingT = (*testing.B)(nil)
}

func TestAssertFails(t *testing.T) {
	SmallTest(t)

	AssertFails(t, `Not equal:\s+expected: 123\s+actual\s+: 124`, func(inner sktest.TestingT) {
		assert.Equal(inner, 123, 124)
	})
	// "We must go deeper."
	AssertFails(t, `In AssertFails, the test function did not fail\.`, func(inner1 sktest.TestingT) {
		AssertFails(inner1, "blah", func(inner2 sktest.TestingT) {
			assert.Equal(inner2, 123, 123)
		})
	})
	AssertFails(t, `In AssertFails, the test function did not produce any failure messages\.`, func(inner1 sktest.TestingT) {
		AssertFails(inner1, "blah", func(inner2 sktest.TestingT) {
			inner2.Fail()
		})
	})
	AssertFails(t, `Expect "misunderestimate" to match "blah"`, func(inner1 sktest.TestingT) {
		AssertFails(inner1, "blah", func(inner2 sktest.TestingT) {
			inner2.Fatalf("misunderestimate")
		})
	})
}
