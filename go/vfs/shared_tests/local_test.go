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

func TestLocal_ReadOnly(t *testing.T) {
	tmp := MakeTestFiles(t)
	fs := vfs.Local(tmp)
	TestVFS_ReadOnly(t, fs)
}

func TestLocal_ReadWrite(t *testing.T) {
	tmp := MakeTestFiles(t)
	fs := vfs.Local(tmp)
	TestVFS_ReadWrite(t, fs)
}

func TestLocal_MultiWrite_ChangedToOriginal(t *testing.T) {
	tmp := MakeTestFiles(t)
	fs := vfs.Local(tmp)
	TestVFS_MultiWrite_ChangedToOriginal(t, fs)
}
