package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Interface       --srcpkg=k8s.io/client-go/kubernetes --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name CoreV1Interface --srcpkg=k8s.io/client-go/kubernetes/typed/core/v1 --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name PodInterface    --srcpkg=k8s.io/client-go/kubernetes/typed/core/v1 --output ${PWD}
