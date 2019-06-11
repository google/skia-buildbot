package mem_ignorestore

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore/common_test"
)

func TestTestMemIgnoreStore(t *testing.T) {
	unittest.SmallTest(t)
	memStore := New()
	common_test.IgnoreStoreAll(t, memStore)
}
