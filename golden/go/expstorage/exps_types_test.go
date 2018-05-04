package expstorage

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/golden/go/types"
)

func TestNameDigestLabels(t *testing.T) {
	TEST_1, TEST_2 := "test1", "test2"
	DIGEST_11, DIGEST_12 := "d11", "d12"
	DIGEST_21, DIGEST_22 := "d21", "d22"

	expChange_1 := map[string]types.TestClassification{
		TEST_1: {
			DIGEST_11: types.POSITIVE,
			DIGEST_12: types.NEGATIVE,
		},
		TEST_2: {
			DIGEST_21: types.POSITIVE,
			DIGEST_22: types.NEGATIVE,
		},
	}

	_, expColl := buildTDESlice(expChange_1, "", nil)
	assert.Equal(t, expChange_1, expColl.toExpectations(false).Tests)

	var emptyColl TDESlice
	emptyColl.update(expChange_1)
	assert.Equal(t, expChange_1, emptyColl.toExpectations(false).Tests)
}
