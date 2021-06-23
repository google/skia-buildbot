// Code generated by mockery v2.4.0. DO NOT EDIT.

package mocks

import (
	context "context"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	gerrit "go.skia.org/infra/go/gerrit"

	mock "github.com/stretchr/testify/mock"
)

// GerritInterface is an autogenerated mock type for the GerritInterface type
type GerritInterface struct {
	mock.Mock
}

// Abandon provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) Abandon(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddCC provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) AddCC(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 []string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, []string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddComment provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) AddComment(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Approve provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) Approve(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Config provides a mock function with given fields:
func (_m *GerritInterface) Config() *gerrit.Config {
	ret := _m.Called()

	var r0 *gerrit.Config
	if rf, ok := ret.Get(0).(func() *gerrit.Config); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gerrit.Config)
		}
	}

	return r0
}

// CreateChange provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *GerritInterface) CreateChange(_a0 context.Context, _a1 string, _a2 string, _a3 string, _a4 string) (*gerrit.ChangeInfo, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4)

	var r0 *gerrit.ChangeInfo
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) *gerrit.ChangeInfo); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gerrit.ChangeInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteChangeEdit provides a mock function with given fields: _a0, _a1
func (_m *GerritInterface) DeleteChangeEdit(_a0 context.Context, _a1 *gerrit.ChangeInfo) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteFile provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) DeleteFile(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Disapprove provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) Disapprove(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DownloadCommitMsgHook provides a mock function with given fields: ctx, dest
func (_m *GerritInterface) DownloadCommitMsgHook(ctx context.Context, dest string) error {
	ret := _m.Called(ctx, dest)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, dest)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// EditFile provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *GerritInterface) EditFile(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string, _a3 string) error {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string, string) error); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExtractIssueFromCommit provides a mock function with given fields: _a0
func (_m *GerritInterface) ExtractIssueFromCommit(_a0 string) (int64, error) {
	ret := _m.Called(_a0)

	var r0 int64
	if rf, ok := ret.Get(0).(func(string) int64); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Files provides a mock function with given fields: ctx, issue, patch
func (_m *GerritInterface) Files(ctx context.Context, issue int64, patch string) (map[string]*gerrit.FileInfo, error) {
	ret := _m.Called(ctx, issue, patch)

	var r0 map[string]*gerrit.FileInfo
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) map[string]*gerrit.FileInfo); ok {
		r0 = rf(ctx, issue, patch)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]*gerrit.FileInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, string) error); ok {
		r1 = rf(ctx, issue, patch)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetChange provides a mock function with given fields: ctx, id
func (_m *GerritInterface) GetChange(ctx context.Context, id string) (*gerrit.ChangeInfo, error) {
	ret := _m.Called(ctx, id)

	var r0 *gerrit.ChangeInfo
	if rf, ok := ret.Get(0).(func(context.Context, string) *gerrit.ChangeInfo); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gerrit.ChangeInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetFileNames provides a mock function with given fields: ctx, issue, patch
func (_m *GerritInterface) GetFileNames(ctx context.Context, issue int64, patch string) ([]string, error) {
	ret := _m.Called(ctx, issue, patch)

	var r0 []string
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) []string); ok {
		r0 = rf(ctx, issue, patch)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, string) error); ok {
		r1 = rf(ctx, issue, patch)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetIssueProperties provides a mock function with given fields: _a0, _a1
func (_m *GerritInterface) GetIssueProperties(_a0 context.Context, _a1 int64) (*gerrit.ChangeInfo, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *gerrit.ChangeInfo
	if rf, ok := ret.Get(0).(func(context.Context, int64) *gerrit.ChangeInfo); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gerrit.ChangeInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPatch provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) GetPatch(_a0 context.Context, _a1 int64, _a2 string) (string, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) string); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, string) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRepoUrl provides a mock function with given fields:
func (_m *GerritInterface) GetRepoUrl() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetTrybotResults provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) GetTrybotResults(_a0 context.Context, _a1 int64, _a2 int64) ([]*buildbucketpb.Build, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 []*buildbucketpb.Build
	if rf, ok := ret.Get(0).(func(context.Context, int64, int64) []*buildbucketpb.Build); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*buildbucketpb.Build)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, int64) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUserEmail provides a mock function with given fields: _a0
func (_m *GerritInterface) GetUserEmail(_a0 context.Context) (string, error) {
	ret := _m.Called(_a0)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Initialized provides a mock function with given fields:
func (_m *GerritInterface) Initialized() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// IsBinaryPatch provides a mock function with given fields: ctx, issue, patch
func (_m *GerritInterface) IsBinaryPatch(ctx context.Context, issue int64, patch string) (bool, error) {
	ret := _m.Called(ctx, issue, patch)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) bool); ok {
		r0 = rf(ctx, issue, patch)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, string) error); ok {
		r1 = rf(ctx, issue, patch)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MoveFile provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *GerritInterface) MoveFile(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string, _a3 string) error {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string, string) error); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NoScore provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) NoScore(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PublishChangeEdit provides a mock function with given fields: _a0, _a1
func (_m *GerritInterface) PublishChangeEdit(_a0 context.Context, _a1 *gerrit.ChangeInfo) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveFromCQ provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) RemoveFromCQ(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Search provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *GerritInterface) Search(_a0 context.Context, _a1 int, _a2 bool, _a3 ...*gerrit.SearchTerm) ([]*gerrit.ChangeInfo, error) {
	_va := make([]interface{}, len(_a3))
	for _i := range _a3 {
		_va[_i] = _a3[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1, _a2)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []*gerrit.ChangeInfo
	if rf, ok := ret.Get(0).(func(context.Context, int, bool, ...*gerrit.SearchTerm) []*gerrit.ChangeInfo); ok {
		r0 = rf(_a0, _a1, _a2, _a3...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*gerrit.ChangeInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int, bool, ...*gerrit.SearchTerm) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SelfApprove provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) SelfApprove(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendToCQ provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) SendToCQ(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendToDryRun provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) SendToDryRun(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetCommitMessage provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) SetCommitMessage(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetReadyForReview provides a mock function with given fields: _a0, _a1
func (_m *GerritInterface) SetReadyForReview(_a0 context.Context, _a1 *gerrit.ChangeInfo) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetReview provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4, _a5, _a6, _a7
func (_m *GerritInterface) SetReview(_a0 context.Context, _a1 *gerrit.ChangeInfo, _a2 string, _a3 map[string]int, _a4 []string, _a5 gerrit.NotifyOption, _a6 string, _a7 int) error {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4, _a5, _a6, _a7)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo, string, map[string]int, []string, gerrit.NotifyOption, string, int) error); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4, _a5, _a6, _a7)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetTopic provides a mock function with given fields: _a0, _a1, _a2
func (_m *GerritInterface) SetTopic(_a0 context.Context, _a1 string, _a2 int64) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Submit provides a mock function with given fields: _a0, _a1
func (_m *GerritInterface) Submit(_a0 context.Context, _a1 *gerrit.ChangeInfo) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gerrit.ChangeInfo) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Url provides a mock function with given fields: _a0
func (_m *GerritInterface) Url(_a0 int64) string {
	ret := _m.Called(_a0)

	var r0 string
	if rf, ok := ret.Get(0).(func(int64) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
