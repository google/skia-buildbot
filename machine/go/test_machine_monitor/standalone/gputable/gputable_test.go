package gputable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGPUVendorNameToID_CanonicalizesCase(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, VendorNameToID("Nvidia"), VendorID("10de"))
}
