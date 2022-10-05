package mocks

//go:generate bazelisk run --config=mayberemote //:mockery -- --name DB  --srcpkg=go.skia.org/infra/autoroll/go/status --output ${PWD}
