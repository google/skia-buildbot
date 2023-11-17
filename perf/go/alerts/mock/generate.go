package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ConfigProvider --srcpkg=go.skia.org/infra/perf/go/alerts --output ${PWD}

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Store --srcpkg=go.skia.org/infra/perf/go/alerts --output ${PWD}
