// Code generated by mockery v2.4.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	vcsinfo "go.skia.org/infra/go/vcsinfo"
)

// ChangelistLandedUpdater is an autogenerated mock type for the ChangelistLandedUpdater type
type ChangelistLandedUpdater struct {
	mock.Mock
}

// UpdateChangelistsAsLanded provides a mock function with given fields: ctx, commits
func (_m *ChangelistLandedUpdater) UpdateChangelistsAsLanded(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	ret := _m.Called(ctx, commits)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []*vcsinfo.LongCommit) error); ok {
		r0 = rf(ctx, commits)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
