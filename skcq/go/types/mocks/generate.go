package mocks

//go:generate bazelisk run //:mockery   -- --name ThrottlerManager  --srcpkg=go.skia.org/infra/skcq/go/types --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name Verifier  --srcpkg=go.skia.org/infra/skcq/go/types --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name VerifiersManager  --srcpkg=go.skia.org/infra/skcq/go/types --output ${PWD}
