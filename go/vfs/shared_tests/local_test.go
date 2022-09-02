package shared_tests

import (
	"context"
	"testing"

	"go.skia.org/infra/go/vfs"
)

func TestLocal(t *testing.T) {

	ctx := context.Background()
	tmp := MakeTestFiles(t)
	fs := vfs.Local(tmp)
	TestFS(ctx, t, fs)
}
