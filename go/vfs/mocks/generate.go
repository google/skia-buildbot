package mocks

//go:generate bazelisk run //:mockery   -- --name FS  --srcpkg=go.skia.org/infra/go/vfs --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name File  --srcpkg=go.skia.org/infra/go/vfs --output ${PWD}
