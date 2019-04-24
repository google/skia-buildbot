package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	itesting "go.skia.org/infra/go/testing"
)

func TestInterfaces(t *testing.T) {
	SmallTest(t)

	// Ensure that our interfaces are compatible.
	var _ assert.TestingT = itesting.TestingT(nil)
	var _ itesting.TestingT = (*testing.T)(nil)
	var _ itesting.TestingT = (*testing.B)(nil)
}

func TestAssertFails(t *testing.T) {
	SmallTest(t)

	AssertFails(t, `Not equal:\s+expected: 123\s+actual\s+: 124`, func(inner itesting.TestingT) {
		assert.Equal(inner, 123, 124)
	})
	// "We must go deeper."
	AssertFails(t, `In AssertFails, the test function did not fail\.`, func(inner1 itesting.TestingT) {
		AssertFails(inner1, "blah", func(inner2 itesting.TestingT) {
			assert.Equal(inner2, 123, 123)
		})
	})
	AssertFails(t, `In AssertFails, the test function did not produce any failure messages\.`, func(inner1 itesting.TestingT) {
		AssertFails(inner1, "blah", func(inner2 itesting.TestingT) {
			inner2.Fail()
		})
	})
	AssertFails(t, `Expect "misunderestimate" to match "blah"`, func(inner1 itesting.TestingT) {
		AssertFails(inner1, "blah", func(inner2 itesting.TestingT) {
			inner2.Fatalf("misunderestimate")
		})
	})
}
