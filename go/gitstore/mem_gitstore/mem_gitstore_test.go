package mem_gitstore

import (
	"testing"

	"go.skia.org/infra/go/gitstore/shared_tests"
)

func TestMemGitStore(t *testing.T) {
	shared_tests.TestGitStore(t, New())
}
