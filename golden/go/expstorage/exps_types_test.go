package expstorage

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/types"
)

// TODO(stephana): Add tests for some of the special cases, e.g.:
//  - buildTDESlice where input is larger than nDigestsPerRec and/or TDESlice.add when full
//  - TDESlice.update for non-empty TDESlice
//  - TDESlice.update when pre-existing untriaged
//  - TDESlice.update when new untriaged

func TestNameDigestLabels(t *testing.T) {
	testutils.SmallTest(t)

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

	expColl := buildTDESlice(expChange_1)
	assert.Equal(t, expChange_1, expColl.toExpectations(false).Tests)

	var emptyColl TDESlice
	emptyColl.update(expChange_1)
	assert.Equal(t, expChange_1, emptyColl.toExpectations(false).Tests)
}
