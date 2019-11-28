package mem_gitstore

import (
	"testing"

	"go.skia.org/infra/go/gitstore/shared_tests"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMemGitStore(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestGitStore(t, New())
}
