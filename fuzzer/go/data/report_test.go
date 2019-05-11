package data

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSortedFuzzReports(t *testing.T) {
	unittest.SmallTest(t)
	a := make(SortedFuzzReports, 0, 5)
	addingOrder := []string{"gggg", "aaaa", "cccc", "eeee", "dddd", "bbbb",
		"ffff"}

	for _, key := range addingOrder {
		a = a.Append(MockReport("skpicture", key))
	}

	b := make(SortedFuzzReports, 0, 5)
	sortedOrder := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee",
		"ffff", "gggg"}

	for _, key := range sortedOrder {
		// just add them in already sorted order
		b = append(b, MockReport("skpicture", key))
	}

	assert.Equal(t, b, a, "SortedFuzzReports Not Sorted")

	// test replace
	r := MockReport("skpicture", "hhhh")
	r.FuzzName = "cccc"
	assert.NotEqual(t, r, a[2], "Reports shouldn't be equal yet")

	a.Append(r)
	assert.Equal(t, 7, len(a), "Replacing shouldn't have changed the size")
	assert.Equal(t, r, a[2], "Report 'cccc' should have been overwritten")
}
