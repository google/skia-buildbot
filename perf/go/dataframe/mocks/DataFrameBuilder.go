// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	dataframe "go.skia.org/infra/perf/go/dataframe"

	paramtools "go.skia.org/infra/go/paramtools"

	progress "go.skia.org/infra/perf/go/progress"

	query "go.skia.org/infra/go/query"

	time "time"
)

// DataFrameBuilder is an autogenerated mock type for the DataFrameBuilder type
type DataFrameBuilder struct {
	mock.Mock
}

// NewFromKeysAndRange provides a mock function with given fields: ctx, keys, begin, end, downsample, _a5
func (_m *DataFrameBuilder) NewFromKeysAndRange(ctx context.Context, keys []string, begin time.Time, end time.Time, downsample bool, _a5 progress.Progress) (*dataframe.DataFrame, error) {
	ret := _m.Called(ctx, keys, begin, end, downsample, _a5)

	if len(ret) == 0 {
		panic("no return value specified for NewFromKeysAndRange")
	}

	var r0 *dataframe.DataFrame
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time, bool, progress.Progress) (*dataframe.DataFrame, error)); ok {
		return rf(ctx, keys, begin, end, downsample, _a5)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time, bool, progress.Progress) *dataframe.DataFrame); ok {
		r0 = rf(ctx, keys, begin, end, downsample, _a5)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dataframe.DataFrame)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string, time.Time, time.Time, bool, progress.Progress) error); ok {
		r1 = rf(ctx, keys, begin, end, downsample, _a5)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewFromQueryAndRange provides a mock function with given fields: ctx, begin, end, q, downsample, _a5
func (_m *DataFrameBuilder) NewFromQueryAndRange(ctx context.Context, begin time.Time, end time.Time, q *query.Query, downsample bool, _a5 progress.Progress) (*dataframe.DataFrame, error) {
	ret := _m.Called(ctx, begin, end, q, downsample, _a5)

	if len(ret) == 0 {
		panic("no return value specified for NewFromQueryAndRange")
	}

	var r0 *dataframe.DataFrame
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, time.Time, *query.Query, bool, progress.Progress) (*dataframe.DataFrame, error)); ok {
		return rf(ctx, begin, end, q, downsample, _a5)
	}
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, time.Time, *query.Query, bool, progress.Progress) *dataframe.DataFrame); ok {
		r0 = rf(ctx, begin, end, q, downsample, _a5)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dataframe.DataFrame)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, time.Time, time.Time, *query.Query, bool, progress.Progress) error); ok {
		r1 = rf(ctx, begin, end, q, downsample, _a5)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewNFromKeys provides a mock function with given fields: ctx, end, keys, n, _a4
func (_m *DataFrameBuilder) NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, _a4 progress.Progress) (*dataframe.DataFrame, error) {
	ret := _m.Called(ctx, end, keys, n, _a4)

	if len(ret) == 0 {
		panic("no return value specified for NewNFromKeys")
	}

	var r0 *dataframe.DataFrame
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, []string, int32, progress.Progress) (*dataframe.DataFrame, error)); ok {
		return rf(ctx, end, keys, n, _a4)
	}
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, []string, int32, progress.Progress) *dataframe.DataFrame); ok {
		r0 = rf(ctx, end, keys, n, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dataframe.DataFrame)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, time.Time, []string, int32, progress.Progress) error); ok {
		r1 = rf(ctx, end, keys, n, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewNFromQuery provides a mock function with given fields: ctx, end, q, n, _a4
func (_m *DataFrameBuilder) NewNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, _a4 progress.Progress) (*dataframe.DataFrame, error) {
	ret := _m.Called(ctx, end, q, n, _a4)

	if len(ret) == 0 {
		panic("no return value specified for NewNFromQuery")
	}

	var r0 *dataframe.DataFrame
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, *query.Query, int32, progress.Progress) (*dataframe.DataFrame, error)); ok {
		return rf(ctx, end, q, n, _a4)
	}
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, *query.Query, int32, progress.Progress) *dataframe.DataFrame); ok {
		r0 = rf(ctx, end, q, n, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dataframe.DataFrame)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, time.Time, *query.Query, int32, progress.Progress) error); ok {
		r1 = rf(ctx, end, q, n, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NumMatches provides a mock function with given fields: ctx, q
func (_m *DataFrameBuilder) NumMatches(ctx context.Context, q *query.Query) (int64, error) {
	ret := _m.Called(ctx, q)

	if len(ret) == 0 {
		panic("no return value specified for NumMatches")
	}

	var r0 int64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *query.Query) (int64, error)); ok {
		return rf(ctx, q)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *query.Query) int64); ok {
		r0 = rf(ctx, q)
	} else {
		r0 = ret.Get(0).(int64)
	}

	if rf, ok := ret.Get(1).(func(context.Context, *query.Query) error); ok {
		r1 = rf(ctx, q)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PreflightQuery provides a mock function with given fields: ctx, q, referenceParamSet
func (_m *DataFrameBuilder) PreflightQuery(ctx context.Context, q *query.Query, referenceParamSet paramtools.ReadOnlyParamSet) (int64, paramtools.ParamSet, error) {
	ret := _m.Called(ctx, q, referenceParamSet)

	if len(ret) == 0 {
		panic("no return value specified for PreflightQuery")
	}

	var r0 int64
	var r1 paramtools.ParamSet
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *query.Query, paramtools.ReadOnlyParamSet) (int64, paramtools.ParamSet, error)); ok {
		return rf(ctx, q, referenceParamSet)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *query.Query, paramtools.ReadOnlyParamSet) int64); ok {
		r0 = rf(ctx, q, referenceParamSet)
	} else {
		r0 = ret.Get(0).(int64)
	}

	if rf, ok := ret.Get(1).(func(context.Context, *query.Query, paramtools.ReadOnlyParamSet) paramtools.ParamSet); ok {
		r1 = rf(ctx, q, referenceParamSet)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(paramtools.ParamSet)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *query.Query, paramtools.ReadOnlyParamSet) error); ok {
		r2 = rf(ctx, q, referenceParamSet)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// NewDataFrameBuilder creates a new instance of DataFrameBuilder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDataFrameBuilder(t interface {
	mock.TestingT
	Cleanup(func())
}) *DataFrameBuilder {
	mock := &DataFrameBuilder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
