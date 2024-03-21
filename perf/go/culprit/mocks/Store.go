// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "go.skia.org/infra/perf/go/culprit/proto/v1"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// Get provides a mock function with given fields: ctx, ids
func (_m *Store) Get(ctx context.Context, ids []string) ([]*v1.Culprit, error) {
	ret := _m.Called(ctx, ids)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 []*v1.Culprit
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]*v1.Culprit, error)); ok {
		return rf(ctx, ids)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*v1.Culprit); ok {
		r0 = rf(ctx, ids)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.Culprit)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, ids)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Upsert provides a mock function with given fields: ctx, anomaly_group_id, _a2
func (_m *Store) Upsert(ctx context.Context, anomaly_group_id string, _a2 []*v1.Culprit) error {
	ret := _m.Called(ctx, anomaly_group_id, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Upsert")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []*v1.Culprit) error); ok {
		r0 = rf(ctx, anomaly_group_id, _a2)
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
