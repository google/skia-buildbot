package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Interface  --srcpkg=k8s.io/apimachinery/pkg/watch --output ${PWD}
