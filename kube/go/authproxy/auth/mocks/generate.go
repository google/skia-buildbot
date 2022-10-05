package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Auth  --srcpkg=go.skia.org/infra/kube/go/authproxy/auth --output ${PWD}
