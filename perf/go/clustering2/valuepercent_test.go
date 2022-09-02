package clustering2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortValuePercentSlice_TwoKeysTwoValuesEach(t *testing.T) {

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

	SortValuePercentSlice(nil)
}

func TestSortValuePercentSlice_OnlyOneValuePerSubSlice(t *testing.T) {

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

func TestSortValuePercentSlice_MultipleKeysWithAllTheSamePercent(t *testing.T) {

	vpSlice := []ValuePercent{
		{"arch=x86", 100},
		{"foo=a", 50},
		{"config=565", 100},
		{"foo=b", 50},
		{"os=linux", 100},
		{"gpu=mali", 100},
	}

	SortValuePercentSlice(vpSlice)

	expected := []ValuePercent{
		{"arch=x86", 100},
		{"config=565", 100},
		{"gpu=mali", 100},
		{"os=linux", 100},
		{"foo=a", 50},
		{"foo=b", 50},
	}

	assert.Equal(t, expected, vpSlice)
}
