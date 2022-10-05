package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name APIClient  --srcpkg=go.skia.org/infra/am/go/alertclient --output ${PWD}
