package clustering2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSortValuePercentSlice_TwoKeysTwoValuesEach(t *testing.T) {
	unittest.SmallTest(t)

	vpSlice := []ValuePercent{
		{"arch=x86", 80},
		{"config=565", 10},
		{"config=8888", 90},
		{"arch=arm", 20},
	}

	SortValuePercentSlice(vpSlice)

	expected := []ValuePercent{
		{"config=8888", 90},
		{"config=565", 10},
		{"arch=x86", 80},
		{"arch=arm", 20},
	}

	assert.Equal(t, expected, vpSlice)
}

func TestSortValuePercentSlice_Empty(t *testing.T) {
	unittest.SmallTest(t)

	SortValuePercentSlice(nil)
}

func TestSortValuePercentSlice_OnlyOneValuePerSubSlice(t *testing.T) {
	unittest.SmallTest(t)

	vpSlice := []ValuePercent{
		{"config=565", 10},
		{"arch=x86", 80},
	}

	SortValuePercentSlice(vpSlice)

	expected := []ValuePercent{
		{"arch=x86", 80},
		{"config=565", 10},
	}

	assert.Equal(t, expected, vpSlice)
}
