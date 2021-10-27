package mocks

import (
	mock "github.com/stretchr/testify/mock"
	specs "go.skia.org/infra/task_scheduler/go/specs"
)

// FixedTasksCfg returns a TaskCfgCache which always produces the given
// TasksCfg instance.
func FixedTasksCfg(cfg *specs.TasksCfg) *TaskCfgCache {
	tcc := &TaskCfgCache{}
	tcc.On("Get", mock.Anything, mock.Anything).Return(cfg, nil, nil)
	return tcc
}

// TasksAlwaysDefined returns a TaskCfgCache which always produces a TasksCfg
// with the given tasks defined. It does not provide details for the tasks.
func TasksAlwaysDefined(taskNames ...string) *TaskCfgCache {
	cfg := &specs.TasksCfg{
		Tasks: map[string]*specs.TaskSpec{},
	}
	for _, tn := range taskNames {
		cfg.Tasks[tn] = &specs.TaskSpec{} // just needs to have a key, the value does not matter
	}
	return FixedTasksCfg(cfg)
}
