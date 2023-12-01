package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Cacher  --srcpkg=go.skia.org/infra/task_scheduler/go/cacher --output ${PWD}
