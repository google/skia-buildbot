package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name GCSClient  --srcpkg=go.skia.org/infra/golden/go/storage --output ${PWD}
