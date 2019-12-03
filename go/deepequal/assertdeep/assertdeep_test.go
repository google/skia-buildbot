package assertdeep

import (
	"testing"

	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAssertJSONRoundTrip(t *testing.T) {
	unittest.SmallTest(t)

	type Success struct {
		Public int `json:"public"`
	}
	JSONRoundTripEqual(t, &Success{
		Public: 123,
	})

	type Unencodable struct {
		Unsupported map[Success]struct{} `json:"unsupported"`
	}
	testutils.AssertFails(t, `unsupported type: map\[\w+\.Success]struct`, func(t sktest.TestingT) {
		JSONRoundTripEqual(t, &Unencodable{
			Unsupported: map[Success]struct{}{
				{
					Public: 5,
				}: {},
			},
		})
	})

	type CantRoundTrip struct {
		// go vet complains if we add a json struct field tag to a private field.
		private int
	}
	testutils.AssertFails(t, "Objects do not match", func(t sktest.TestingT) {
		JSONRoundTripEqual(t, &CantRoundTrip{
			private: 123,
		})
	})
}
