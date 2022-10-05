package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name TaskCfgCache  --srcpkg=go.skia.org/infra/task_scheduler/go/task_cfg_cache --output ${PWD}
