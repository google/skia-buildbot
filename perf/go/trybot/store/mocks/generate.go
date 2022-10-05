package mocks

//go:generate bazelisk run --config=mayberemote //:mockery -- --name TryBotStore  --srcpkg=go.skia.org/infra/perf/go/trybot/store --output ${PWD}
