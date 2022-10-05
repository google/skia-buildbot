package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name TaskCache  --srcpkg=go.skia.org/infra/task_scheduler/go/db/cache --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name JobCache  --srcpkg=go.skia.org/infra/task_scheduler/go/db/cache --output ${PWD}
