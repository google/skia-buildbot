package sets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func readAllFromChannel(in <-chan []int) [][]int {
	ret := [][]int{}
	for s := range in {
		ret = append(ret, s)
	}
	return ret
}

func TestCartesianProduct_EmptySlice_EmptyChannel(t *testing.T) {
	in, err := CartesianProduct([]int{})
	assert.NoError(t, err)
	assert.Equal(t, [][]int{}, readAllFromChannel(in))
}

func TestCartesianProduct_SliceOfLengthOne_ChannelJustCountsDown(t *testing.T) {
	in, err := CartesianProduct([]int{5})
	assert.NoError(t, err)
	assert.Equal(t, [][]int{
		{4},
		{3},
		{2},
		{1},
		{0},
	}, readAllFromChannel(in))
}

func TestCartesianProduct_SliceOfLengthTwoButOneCountIsOne_ChannelJustCountsDown(t *testing.T) {
	in, err := CartesianProduct([]int{5, 1})
	assert.NoError(t, err)
	assert.Equal(t, [][]int{
		{4, 0},
		{3, 0},
		{2, 0},
		{1, 0},
		{0, 0},
	}, readAllFromChannel(in))
}

func TestCartesianProduct_SliceOfLengthTwoButOtherCountIsOne_ChannelJustCountsDown(t *testing.T) {
	in, err := CartesianProduct([]int{1, 5})
	assert.NoError(t, err)
	assert.Equal(t, [][]int{
		{0, 4},
		{0, 3},
		{0, 2},
		{0, 1},
		{0, 0},
	}, readAllFromChannel(in))
}

func TestCartesianProduct_SliceOfLengthTwoButCountIsZero_ReturnsError(t *testing.T) {
	_, err := CartesianProduct([]int{1, 0})
	assert.Error(t, err)
}

func TestCartesianProduct_SliceOfLengthTwo_ChannelEmitsCartesianProduct(t *testing.T) {
	in, err := CartesianProduct([]int{3, 2})
	assert.NoError(t, err)
	assert.Equal(t, [][]int{
		{2, 1},
		{1, 1},
		{0, 1},
		{2, 0},
		{1, 0},
		{0, 0},
	}, readAllFromChannel(in))
}

func TestCartesianProduct_SliceOfLengthThree_ChannelEmitsCartesianProduct(t *testing.T) {
	in, err := CartesianProduct([]int{2, 2, 3})
	assert.NoError(t, err)
	assert.Equal(t, [][]int{
		{1, 1, 2},
		{0, 1, 2},
		{1, 0, 2},
		{0, 0, 2},
		{1, 1, 1},
		{0, 1, 1},
		{1, 0, 1},
		{0, 0, 1},
		{1, 1, 0},
		{0, 1, 0},
		{1, 0, 0},
		{0, 0, 0},
	}, readAllFromChannel(in))
}
