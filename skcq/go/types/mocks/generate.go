package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ThrottlerManager  --srcpkg=go.skia.org/infra/skcq/go/types --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Verifier  --srcpkg=go.skia.org/infra/skcq/go/types --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name VerifiersManager  --srcpkg=go.skia.org/infra/skcq/go/types --output ${PWD}
