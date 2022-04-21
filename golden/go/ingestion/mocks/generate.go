package mocks

//go:generate bazelisk run //:mockery   -- --name FileSearcher  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name Processor  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name Source  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name Store  --srcpkg=go.skia.org/infra/golden/go/ingestion --output ${PWD}
