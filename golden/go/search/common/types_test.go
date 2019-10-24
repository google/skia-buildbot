package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// All this test data is valid, but arbitrary.
const (
	alphaPositiveDigest = types.Digest("aaa884cd5ac3d6785c35cff8f26d2da5")
	betaNegativeDigest  = types.Digest("bbb8d94852dfde3f3bebcc000be60153")
	gammaPositiveDigest = types.Digest("ccc84ad6f1a0c628d5f27180e497309e")
	untriagedDigest     = types.Digest("7bf4d4e913605c0781697df4004191c5")
	testName            = types.TestName("some_test")
)

func TestExpSliceMasterBranch(t *testing.T) {
	unittest.SmallTest(t)

	var expOne expectations.Expectations
	expOne.Set(testName, alphaPositiveDigest, expectations.Positive)
	expOne.Set(testName, betaNegativeDigest, expectations.Negative)

	e := ExpSlice{&expOne}

	assert.Equal(t, expectations.Positive, e.Classification(testName, alphaPositiveDigest))
	assert.Equal(t, expectations.Negative, e.Classification(testName, betaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(testName, untriagedDigest))
}

func TestExpSliceCL(t *testing.T) {
	unittest.SmallTest(t)

	var masterE expectations.Expectations
	masterE.Set(testName, alphaPositiveDigest, expectations.Positive)
	masterE.Set(testName, betaNegativeDigest, expectations.Positive)

	var changeListE expectations.Expectations
	changeListE.Set(testName, gammaPositiveDigest, expectations.Positive)
	changeListE.Set(testName, betaNegativeDigest, expectations.Negative) // this should win

	e := ExpSlice{&changeListE, &masterE}

	assert.Equal(t, expectations.Positive, e.Classification(testName, alphaPositiveDigest))
	assert.Equal(t, expectations.Positive, e.Classification(testName, gammaPositiveDigest))
	assert.Equal(t, expectations.Negative, e.Classification(testName, betaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(testName, untriagedDigest))
}
