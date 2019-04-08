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

func TestAssertFails(t *testing.T) {
	SmallTest(t)

	AssertFails(t, `Not equal:\s+expected: 123\s+actual\s+: 124`, func(inner TestingT) {
		assert.Equal(inner, 123, 124)
	})
	// "We must go deeper."
	AssertFails(t, `In AssertFails, the test function did not fail\.`, func(inner1 TestingT) {
		AssertFails(inner1, "blah", func(inner2 TestingT) {
			assert.Equal(inner2, 123, 123)
		})
	})
	AssertFails(t, `In AssertFails, the test function did not produce any failure messages\.`, func(inner1 TestingT) {
		AssertFails(inner1, "blah", func(inner2 TestingT) {
			inner2.Fail()
		})
	})
	AssertFails(t, `Expect "misunderestimate" to match "blah"`, func(inner1 TestingT) {
		AssertFails(inner1, "blah", func(inner2 TestingT) {
			inner2.Fatalf("misunderestimate")
		})
	})
}
