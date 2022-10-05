package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name CapacityClient  --srcpkg=go.skia.org/infra/status/go/capacity --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name IncrementalCache  --srcpkg=go.skia.org/infra/status/go/incremental --output ${PWD}
