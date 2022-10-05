package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name LookupSystem  --srcpkg=go.skia.org/infra/golden/go/ingestion_processors --output ${PWD}
