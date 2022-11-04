package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Server  --srcpkg=go.skia.org/infra/go/sser --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name PeerFinder  --srcpkg=go.skia.org/infra/go/sser --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Interface  --srcpkg=k8s.io/client-go/kubernetes --output ${PWD}
