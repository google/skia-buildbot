package mem_ignorestore

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore/testutils"
)

func TestTestMemIgnoreStore(t *testing.T) {
	unittest.SmallTest(t)
	memStore := New()
	testutils.IgnoreStoreAll(t, memStore)
}
