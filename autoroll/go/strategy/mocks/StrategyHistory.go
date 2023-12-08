// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	strategy "go.skia.org/infra/autoroll/go/strategy"
)

// StrategyHistory is an autogenerated mock type for the StrategyHistory type
type StrategyHistory struct {
	mock.Mock
}

// Add provides a mock function with given fields: ctx, s, user, message
func (_m *StrategyHistory) Add(ctx context.Context, s string, user string, message string) error {
	ret := _m.Called(ctx, s, user, message)

	if len(ret) == 0 {
		panic("no return value specified for Add")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) error); ok {
		r0 = rf(ctx, s, user, message)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CurrentStrategy provides a mock function with given fields:
func (_m *StrategyHistory) CurrentStrategy() *strategy.StrategyChange {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CurrentStrategy")
	}

	var r0 *strategy.StrategyChange
	if rf, ok := ret.Get(0).(func() *strategy.StrategyChange); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*strategy.StrategyChange)
		}
	}

	return r0
}

// GetHistory provides a mock function with given fields: ctx, offset
func (_m *StrategyHistory) GetHistory(ctx context.Context, offset int) ([]*strategy.StrategyChange, int, error) {
	ret := _m.Called(ctx, offset)

	if len(ret) == 0 {
		panic("no return value specified for GetHistory")
	}

	var r0 []*strategy.StrategyChange
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, int) ([]*strategy.StrategyChange, int, error)); ok {
		return rf(ctx, offset)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) []*strategy.StrategyChange); ok {
		r0 = rf(ctx, offset)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*strategy.StrategyChange)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) int); ok {
		r1 = rf(ctx, offset)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, int) error); ok {
		r2 = rf(ctx, offset)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Update provides a mock function with given fields: ctx
func (_m *StrategyHistory) Update(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewStrategyHistory creates a new instance of StrategyHistory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewStrategyHistory(t interface {
	mock.TestingT
	Cleanup(func())
}) *StrategyHistory {
	mock := &StrategyHistory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
