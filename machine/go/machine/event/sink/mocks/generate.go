package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Sink  --srcpkg=go.skia.org/infra/machine/go/machine/event/sink --output ${PWD}
