package mocks

// Generate mocks.
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name DB  --srcpkg=go.skia.org/infra/go/louhi --output ${PWD}
