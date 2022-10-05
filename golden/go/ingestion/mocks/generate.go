package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name FileSearcher  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Processor  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Source  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Store  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
