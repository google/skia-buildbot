package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Calculator  --srcpkg=go.skia.org/infra/golden/go/diff --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ImageSource  --srcpkg=go.skia.org/infra/golden/go/diff/worker --output ${PWD}
