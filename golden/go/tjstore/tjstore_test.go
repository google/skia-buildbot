package tjstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombinedPSID_Equal(t *testing.T) {
	assert.True(t, CombinedPSID{
		CL:  "alpha",
		CRS: "beta",
		PS:  "gamma",
	}.Equal(CombinedPSID{
		CL:  "alpha",
		CRS: "beta",
		PS:  "gamma",
	}))

	assert.False(t, CombinedPSID{
		CL:  "alpha",
		CRS: "beta",
		PS:  "alabama",
	}.Equal(CombinedPSID{
		CL:  "alpha",
		CRS: "beta",
		PS:  "gamma",
	}))
	assert.False(t, CombinedPSID{
		CL:  "alpha",
		CRS: "beta",
		PS:  "alabama",
	}.Equal(CombinedPSID{}))
}
