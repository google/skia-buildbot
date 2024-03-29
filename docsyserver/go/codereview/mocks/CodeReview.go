// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	codereview "go.skia.org/infra/docsyserver/go/codereview"

	mock "github.com/stretchr/testify/mock"
)

// CodeReview is an autogenerated mock type for the CodeReview type
type CodeReview struct {
	mock.Mock
}

// GetFile provides a mock function with given fields: ctx, filename, ref
func (_m *CodeReview) GetFile(ctx context.Context, filename string, ref string) ([]byte, error) {
	ret := _m.Called(ctx, filename, ref)

	if len(ret) == 0 {
		panic("no return value specified for GetFile")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) ([]byte, error)); ok {
		return rf(ctx, filename, ref)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) []byte); ok {
		r0 = rf(ctx, filename, ref)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, filename, ref)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPatchsetInfo provides a mock function with given fields: ctx, issue
func (_m *CodeReview) GetPatchsetInfo(ctx context.Context, issue codereview.Issue) (string, bool, error) {
	ret := _m.Called(ctx, issue)

	if len(ret) == 0 {
		panic("no return value specified for GetPatchsetInfo")
	}

	var r0 string
	var r1 bool
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, codereview.Issue) (string, bool, error)); ok {
		return rf(ctx, issue)
	}
	if rf, ok := ret.Get(0).(func(context.Context, codereview.Issue) string); ok {
		r0 = rf(ctx, issue)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, codereview.Issue) bool); ok {
		r1 = rf(ctx, issue)
	} else {
		r1 = ret.Get(1).(bool)
	}

	if rf, ok := ret.Get(2).(func(context.Context, codereview.Issue) error); ok {
		r2 = rf(ctx, issue)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ListModifiedFiles provides a mock function with given fields: ctx, issue, ref
func (_m *CodeReview) ListModifiedFiles(ctx context.Context, issue codereview.Issue, ref string) ([]codereview.ListModifiedFilesResult, error) {
	ret := _m.Called(ctx, issue, ref)

	if len(ret) == 0 {
		panic("no return value specified for ListModifiedFiles")
	}

	var r0 []codereview.ListModifiedFilesResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, codereview.Issue, string) ([]codereview.ListModifiedFilesResult, error)); ok {
		return rf(ctx, issue, ref)
	}
	if rf, ok := ret.Get(0).(func(context.Context, codereview.Issue, string) []codereview.ListModifiedFilesResult); ok {
		r0 = rf(ctx, issue, ref)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]codereview.ListModifiedFilesResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, codereview.Issue, string) error); ok {
		r1 = rf(ctx, issue, ref)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewCodeReview creates a new instance of CodeReview. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewCodeReview(t interface {
	mock.TestingT
	Cleanup(func())
}) *CodeReview {
	mock := &CodeReview{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
