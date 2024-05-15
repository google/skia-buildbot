// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// GetAllSubscriptions provides a mock function with given fields: ctx
func (_m *Store) GetAllSubscriptions(ctx context.Context) ([]*v1.Subscription, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetAllSubscriptions")
	}

	var r0 []*v1.Subscription
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*v1.Subscription, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*v1.Subscription); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.Subscription)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSubscription provides a mock function with given fields: ctx, name, revision
func (_m *Store) GetSubscription(ctx context.Context, name string, revision string) (*v1.Subscription, error) {
	ret := _m.Called(ctx, name, revision)

	if len(ret) == 0 {
		panic("no return value specified for GetSubscription")
	}

	var r0 *v1.Subscription
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Subscription, error)); ok {
		return rf(ctx, name, revision)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Subscription); ok {
		r0 = rf(ctx, name, revision)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Subscription)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, name, revision)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// InsertSubscriptions provides a mock function with given fields: ctx, _a1
func (_m *Store) InsertSubscriptions(ctx context.Context, _a1 []*v1.Subscription) error {
	ret := _m.Called(ctx, _a1)

	if len(ret) == 0 {
		panic("no return value specified for InsertSubscriptions")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []*v1.Subscription) error); ok {
		r0 = rf(ctx, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewStore creates a new instance of Store. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *Store {
	mock := &Store{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
