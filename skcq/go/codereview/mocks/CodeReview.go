// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	codereview "go.skia.org/infra/skcq/go/codereview"

	gerrit "go.skia.org/infra/go/gerrit"

	mock "github.com/stretchr/testify/mock"
)

// CodeReview is an autogenerated mock type for the CodeReview type
type CodeReview struct {
	mock.Mock
}

// AddComment provides a mock function with given fields: ctx, ci, comment, notify, notifyReason
func (_m *CodeReview) AddComment(ctx context.Context, ci *gerrit.ChangeInfo, comment string, notify codereview.NotifyOption, notifyReason string) error {
	ret := _m.Called(ctx, ci, comment, notify, notifyReason)

	if len(ret) == 0 {
		panic("no return value specified for AddComment")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string, codereview.NotifyOption, string) error); ok {
		r0 = rf(ctx, ci, comment, notify, notifyReason)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetCQVoters provides a mock function with given fields: ctx, ci
func (_m *CodeReview) GetCQVoters(ctx context.Context, ci *gerrit.ChangeInfo) []string {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for GetCQVoters")
	}

	var r0 []string
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) []string); ok {
		r0 = rf(ctx, ci)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}

// GetChangeRef provides a mock function with given fields: ci
func (_m *CodeReview) GetChangeRef(ci *gerrit.ChangeInfo) string {
	ret := _m.Called(ci)

	if len(ret) == 0 {
		panic("no return value specified for GetChangeRef")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func(*gerrit.ChangeInfo) string); ok {
		r0 = rf(ci)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetCommitAuthor provides a mock function with given fields: ctx, issue, revision
func (_m *CodeReview) GetCommitAuthor(ctx context.Context, issue int64, revision string) (string, error) {
	ret := _m.Called(ctx, issue, revision)

	if len(ret) == 0 {
		panic("no return value specified for GetCommitAuthor")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) (string, error)); ok {
		return rf(ctx, issue, revision)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) string); ok {
		r0 = rf(ctx, issue, revision)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64, string) error); ok {
		r1 = rf(ctx, issue, revision)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCommitMessage provides a mock function with given fields: ctx, issue
func (_m *CodeReview) GetCommitMessage(ctx context.Context, issue int64) (string, error) {
	ret := _m.Called(ctx, issue)

	if len(ret) == 0 {
		panic("no return value specified for GetCommitMessage")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (string, error)); ok {
		return rf(ctx, issue)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) string); ok {
		r0 = rf(ctx, issue)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, issue)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEarliestEquivalentPatchSetID provides a mock function with given fields: ci
func (_m *CodeReview) GetEarliestEquivalentPatchSetID(ci *gerrit.ChangeInfo) int64 {
	ret := _m.Called(ci)

	if len(ret) == 0 {
		panic("no return value specified for GetEarliestEquivalentPatchSetID")
	}

	var r0 int64
	if rf, ok := ret.Get(0).(func(*gerrit.ChangeInfo) int64); ok {
		r0 = rf(ci)
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

// GetEquivalentPatchSetIDs provides a mock function with given fields: ci, patchsetID
func (_m *CodeReview) GetEquivalentPatchSetIDs(ci *gerrit.ChangeInfo, patchsetID int64) []int64 {
	ret := _m.Called(ci, patchsetID)

	if len(ret) == 0 {
		panic("no return value specified for GetEquivalentPatchSetIDs")
	}

	var r0 []int64
	if rf, ok := ret.Get(0).(func(*gerrit.ChangeInfo, int64) []int64); ok {
		r0 = rf(ci, patchsetID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]int64)
		}
	}

	return r0
}

// GetFileNames provides a mock function with given fields: ctx, ci
func (_m *CodeReview) GetFileNames(ctx context.Context, ci *gerrit.ChangeInfo) ([]string, error) {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for GetFileNames")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) ([]string, error)); ok {
		return rf(ctx, ci)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) []string); ok {
		r0 = rf(ctx, ci)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r1 = rf(ctx, ci)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetIssueProperties provides a mock function with given fields: ctx, issue
func (_m *CodeReview) GetIssueProperties(ctx context.Context, issue int64) (*gerrit.ChangeInfo, error) {
	ret := _m.Called(ctx, issue)

	if len(ret) == 0 {
		panic("no return value specified for GetIssueProperties")
	}

	var r0 *gerrit.ChangeInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*gerrit.ChangeInfo, error)); ok {
		return rf(ctx, issue)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *gerrit.ChangeInfo); ok {
		r0 = rf(ctx, issue)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gerrit.ChangeInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, issue)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestPatchSetID provides a mock function with given fields: ci
func (_m *CodeReview) GetLatestPatchSetID(ci *gerrit.ChangeInfo) int64 {
	ret := _m.Called(ci)

	if len(ret) == 0 {
		panic("no return value specified for GetLatestPatchSetID")
	}

	var r0 int64
	if rf, ok := ret.Get(0).(func(*gerrit.ChangeInfo) int64); ok {
		r0 = rf(ci)
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

// GetRepoUrl provides a mock function with given fields: ci
func (_m *CodeReview) GetRepoUrl(ci *gerrit.ChangeInfo) string {
	ret := _m.Called(ci)

	if len(ret) == 0 {
		panic("no return value specified for GetRepoUrl")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func(*gerrit.ChangeInfo) string); ok {
		r0 = rf(ci)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetSubmittedTogether provides a mock function with given fields: ctx, ci
func (_m *CodeReview) GetSubmittedTogether(ctx context.Context, ci *gerrit.ChangeInfo) ([]*gerrit.ChangeInfo, error) {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for GetSubmittedTogether")
	}

	var r0 []*gerrit.ChangeInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) ([]*gerrit.ChangeInfo, error)); ok {
		return rf(ctx, ci)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) []*gerrit.ChangeInfo); ok {
		r0 = rf(ctx, ci)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*gerrit.ChangeInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r1 = rf(ctx, ci)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IsCQ provides a mock function with given fields: ctx, ci
func (_m *CodeReview) IsCQ(ctx context.Context, ci *gerrit.ChangeInfo) bool {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for IsCQ")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) bool); ok {
		r0 = rf(ctx, ci)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// IsDryRun provides a mock function with given fields: ctx, ci
func (_m *CodeReview) IsDryRun(ctx context.Context, ci *gerrit.ChangeInfo) bool {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for IsDryRun")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) bool); ok {
		r0 = rf(ctx, ci)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RemoveFromCQ provides a mock function with given fields: ctx, ci, comment, notifyReason
func (_m *CodeReview) RemoveFromCQ(ctx context.Context, ci *gerrit.ChangeInfo, comment string, notifyReason string) {
	_m.Called(ctx, ci, comment, notifyReason)
}

// Search provides a mock function with given fields: ctx
func (_m *CodeReview) Search(ctx context.Context) ([]*gerrit.ChangeInfo, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Search")
	}

	var r0 []*gerrit.ChangeInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*gerrit.ChangeInfo, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*gerrit.ChangeInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*gerrit.ChangeInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetReadyForReview provides a mock function with given fields: ctx, ci
func (_m *CodeReview) SetReadyForReview(ctx context.Context, ci *gerrit.ChangeInfo) error {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for SetReadyForReview")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r0 = rf(ctx, ci)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Submit provides a mock function with given fields: ctx, ci
func (_m *CodeReview) Submit(ctx context.Context, ci *gerrit.ChangeInfo) error {
	ret := _m.Called(ctx, ci)

	if len(ret) == 0 {
		panic("no return value specified for Submit")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r0 = rf(ctx, ci)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Url provides a mock function with given fields: issueID
func (_m *CodeReview) Url(issueID int64) string {
	ret := _m.Called(issueID)

	if len(ret) == 0 {
		panic("no return value specified for Url")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func(int64) string); ok {
		r0 = rf(issueID)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
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
