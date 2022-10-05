package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name RemoteDB  --srcpkg=go.skia.org/infra/task_scheduler/go/db --output ${PWD}
