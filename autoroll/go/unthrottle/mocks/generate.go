package mocks

//go:generate bazelisk run --config=mayberemote //:mockery -- --name Throttle  --srcpkg=go.skia.org/infra/autoroll/go/unthrottle --output ${PWD}
