package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name BugsDB  --srcpkg=go.skia.org/infra/bugs-central/go/types --output ${PWD}
