// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	config "go.skia.org/infra/skcq/go/config"

	gerrit "go.skia.org/infra/go/gerrit"

	mock "github.com/stretchr/testify/mock"

	types "go.skia.org/infra/skcq/go/types"
)

// VerifiersManager is an autogenerated mock type for the VerifiersManager type
type VerifiersManager struct {
	mock.Mock
}

// GetVerifiers provides a mock function with given fields: ctx, cfg, ci, isSubmittedTogetherChange, configReader
func (_m *VerifiersManager) GetVerifiers(ctx context.Context, cfg *config.SkCQCfg, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, configReader config.ConfigReader) ([]types.Verifier, []string, error) {
	ret := _m.Called(ctx, cfg, ci, isSubmittedTogetherChange, configReader)

	if len(ret) == 0 {
		panic("no return value specified for GetVerifiers")
	}

	var r0 []types.Verifier
	var r1 []string
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *config.SkCQCfg, *gerrit.ChangeInfo, bool, config.ConfigReader) ([]types.Verifier, []string, error)); ok {
		return rf(ctx, cfg, ci, isSubmittedTogetherChange, configReader)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *config.SkCQCfg, *gerrit.ChangeInfo, bool, config.ConfigReader) []types.Verifier); ok {
		r0 = rf(ctx, cfg, ci, isSubmittedTogetherChange, configReader)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Verifier)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *config.SkCQCfg, *gerrit.ChangeInfo, bool, config.ConfigReader) []string); ok {
		r1 = rf(ctx, cfg, ci, isSubmittedTogetherChange, configReader)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]string)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *config.SkCQCfg, *gerrit.ChangeInfo, bool, config.ConfigReader) error); ok {
		r2 = rf(ctx, cfg, ci, isSubmittedTogetherChange, configReader)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// RunVerifiers provides a mock function with given fields: ctx, ci, verifiers, startTime
func (_m *VerifiersManager) RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []types.Verifier, startTime int64) []*types.VerifierStatus {
	ret := _m.Called(ctx, ci, verifiers, startTime)

	if len(ret) == 0 {
		panic("no return value specified for RunVerifiers")
	}

	var r0 []*types.VerifierStatus
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, []types.Verifier, int64) []*types.VerifierStatus); ok {
		r0 = rf(ctx, ci, verifiers, startTime)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.VerifierStatus)
		}
	}

	return r0
}

// NewVerifiersManager creates a new instance of VerifiersManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewVerifiersManager(t interface {
	mock.TestingT
	Cleanup(func())
}) *VerifiersManager {
	mock := &VerifiersManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
