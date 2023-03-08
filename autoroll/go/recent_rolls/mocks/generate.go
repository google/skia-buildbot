package mocks

//go:generate bazelisk run --config=mayberemote //:mockery -- --name DB  --srcpkg=go.skia.org/infra/autoroll/go/recent_rolls --output ${PWD}
