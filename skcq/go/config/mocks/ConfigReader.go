// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	config "go.skia.org/infra/skcq/go/config"

	mock "github.com/stretchr/testify/mock"

	specs "go.skia.org/infra/task_scheduler/go/specs"
)

// ConfigReader is an autogenerated mock type for the ConfigReader type
type ConfigReader struct {
	mock.Mock
}

// GetAuthorsFileContents provides a mock function with given fields: ctx, authorsPath
func (_m *ConfigReader) GetAuthorsFileContents(ctx context.Context, authorsPath string) (string, error) {
	ret := _m.Called(ctx, authorsPath)

	if len(ret) == 0 {
		panic("no return value specified for GetAuthorsFileContents")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (string, error)); ok {
		return rf(ctx, authorsPath)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) string); ok {
		r0 = rf(ctx, authorsPath)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, authorsPath)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSkCQCfg provides a mock function with given fields: ctx
func (_m *ConfigReader) GetSkCQCfg(ctx context.Context) (*config.SkCQCfg, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetSkCQCfg")
	}

	var r0 *config.SkCQCfg
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*config.SkCQCfg, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *config.SkCQCfg); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*config.SkCQCfg)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTasksCfg provides a mock function with given fields: ctx, tasksJSONPath
func (_m *ConfigReader) GetTasksCfg(ctx context.Context, tasksJSONPath string) (*specs.TasksCfg, error) {
	ret := _m.Called(ctx, tasksJSONPath)

	if len(ret) == 0 {
		panic("no return value specified for GetTasksCfg")
	}

	var r0 *specs.TasksCfg
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*specs.TasksCfg, error)); ok {
		return rf(ctx, tasksJSONPath)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *specs.TasksCfg); ok {
		r0 = rf(ctx, tasksJSONPath)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*specs.TasksCfg)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, tasksJSONPath)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewConfigReader creates a new instance of ConfigReader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewConfigReader(t interface {
	mock.TestingT
	Cleanup(func())
}) *ConfigReader {
	mock := &ConfigReader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
