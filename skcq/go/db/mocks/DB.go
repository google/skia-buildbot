// Code generated by mockery v2.4.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	db "go.skia.org/infra/skcq/go/db"

	types "go.skia.org/infra/skcq/go/types"
)

// DB is an autogenerated mock type for the DB type
type DB struct {
	mock.Mock
}

// GetChangeAttempts provides a mock function with given fields: ctx, changeID, patchsetID, changesCol
func (_m *DB) GetChangeAttempts(ctx context.Context, changeID int64, patchsetID int64, changesCol db.ChangesCol) (*types.ChangeAttempts, error) {
	ret := _m.Called(ctx, changeID, patchsetID, changesCol)

	var r0 *types.ChangeAttempts
	if rf, ok := ret.Get(0).(func(context.Context, int64, int64, db.ChangesCol) *types.ChangeAttempts); ok {
		r0 = rf(ctx, changeID, patchsetID, changesCol)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.ChangeAttempts)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, int64, db.ChangesCol) error); ok {
		r1 = rf(ctx, changeID, patchsetID, changesCol)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCurrentChanges provides a mock function with given fields: ctx
func (_m *DB) GetCurrentChanges(ctx context.Context) (map[string]*types.CurrentlyProcessingChange, error) {
	ret := _m.Called(ctx)

	var r0 map[string]*types.CurrentlyProcessingChange
	if rf, ok := ret.Get(0).(func(context.Context) map[string]*types.CurrentlyProcessingChange); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]*types.CurrentlyProcessingChange)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PutChangeAttempt provides a mock function with given fields: ctx, newChangeAttempt, changesCol
func (_m *DB) PutChangeAttempt(ctx context.Context, newChangeAttempt *types.ChangeAttempt, changesCol db.ChangesCol) error {
	ret := _m.Called(ctx, newChangeAttempt, changesCol)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.ChangeAttempt, db.ChangesCol) error); ok {
		r0 = rf(ctx, newChangeAttempt, changesCol)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PutCurrentChanges provides a mock function with given fields: ctx, currentChangesCache
func (_m *DB) PutCurrentChanges(ctx context.Context, currentChangesCache interface{}) error {
	ret := _m.Called(ctx, currentChangesCache)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) error); ok {
		r0 = rf(ctx, currentChangesCache)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateChangeAttemptAsAbandoned provides a mock function with given fields: ctx, changeID, patchsetID, changesCol, patchStart
func (_m *DB) UpdateChangeAttemptAsAbandoned(ctx context.Context, changeID int64, patchsetID int64, changesCol db.ChangesCol, patchStart int64) error {
	ret := _m.Called(ctx, changeID, patchsetID, changesCol, patchStart)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, int64, db.ChangesCol, int64) error); ok {
		r0 = rf(ctx, changeID, patchsetID, changesCol, patchStart)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
