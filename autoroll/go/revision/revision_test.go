package revision

import (
	"testing"
	"time"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCopyRevision(t *testing.T) {
	unittest.SmallTest(t)

	v := &Revision{
		Id:          "abc123",
		Display:     "abc",
		Description: "This is a great commit.",
		Timestamp:   time.Now(),
		URL:         "www.best-commit.com",
	}
	deepequal.AssertCopy(t, v, v.Copy())
}
