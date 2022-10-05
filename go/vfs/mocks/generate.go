package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name FS  --srcpkg=go.skia.org/infra/go/vfs --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name File  --srcpkg=go.skia.org/infra/go/vfs --output ${PWD}
