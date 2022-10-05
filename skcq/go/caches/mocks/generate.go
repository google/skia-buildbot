package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name CurrentChangesCache  --srcpkg=go.skia.org/infra/skcq/go/caches --output ${PWD}
