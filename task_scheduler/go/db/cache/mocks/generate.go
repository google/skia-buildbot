package mocks

//go:generate bazelisk run //:mockery   -- --name TaskCache  --srcpkg=go.skia.org/infra/task_scheduler/go/db/cache --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name JobCache  --srcpkg=go.skia.org/infra/task_scheduler/go/db/cache --output ${PWD}
