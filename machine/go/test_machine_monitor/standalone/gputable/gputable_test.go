package gputable

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPUVendorNameToID_CanonicalizesCase(t *testing.T) {
	assert.Equal(t, VendorNameToID("Nvidia"), VendorID("10de"))
}
