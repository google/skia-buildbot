// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	pgconn "github.com/jackc/pgconn"
	mock "github.com/stretchr/testify/mock"

	pgx "github.com/jackc/pgx/v4"

	pgxpool "github.com/jackc/pgx/v4/pgxpool"

	testing "testing"
)

// Pool is an autogenerated mock type for the Pool type
type Pool struct {
	mock.Mock
}

// Acquire provides a mock function with given fields: ctx
func (_m *Pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	ret := _m.Called(ctx)

	var r0 *pgxpool.Conn
	if rf, ok := ret.Get(0).(func(context.Context) *pgxpool.Conn); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pgxpool.Conn)
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

// AcquireAllIdle provides a mock function with given fields: ctx
func (_m *Pool) AcquireAllIdle(ctx context.Context) []*pgxpool.Conn {
	ret := _m.Called(ctx)

	var r0 []*pgxpool.Conn
	if rf, ok := ret.Get(0).(func(context.Context) []*pgxpool.Conn); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*pgxpool.Conn)
		}
	}

	return r0
}

// AcquireFunc provides a mock function with given fields: ctx, f
func (_m *Pool) AcquireFunc(ctx context.Context, f func(*pgxpool.Conn) error) error {
	ret := _m.Called(ctx, f)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, func(*pgxpool.Conn) error) error); ok {
		r0 = rf(ctx, f)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Begin provides a mock function with given fields: ctx
func (_m *Pool) Begin(ctx context.Context) (pgx.Tx, error) {
	ret := _m.Called(ctx)

	var r0 pgx.Tx
	if rf, ok := ret.Get(0).(func(context.Context) pgx.Tx); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgx.Tx)
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

// BeginFunc provides a mock function with given fields: ctx, f
func (_m *Pool) BeginFunc(ctx context.Context, f func(pgx.Tx) error) error {
	ret := _m.Called(ctx, f)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, func(pgx.Tx) error) error); ok {
		r0 = rf(ctx, f)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// BeginTx provides a mock function with given fields: ctx, txOptions
func (_m *Pool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	ret := _m.Called(ctx, txOptions)

	var r0 pgx.Tx
	if rf, ok := ret.Get(0).(func(context.Context, pgx.TxOptions) pgx.Tx); ok {
		r0 = rf(ctx, txOptions)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgx.Tx)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, pgx.TxOptions) error); ok {
		r1 = rf(ctx, txOptions)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// BeginTxFunc provides a mock function with given fields: ctx, txOptions, f
func (_m *Pool) BeginTxFunc(ctx context.Context, txOptions pgx.TxOptions, f func(pgx.Tx) error) error {
	ret := _m.Called(ctx, txOptions, f)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, pgx.TxOptions, func(pgx.Tx) error) error); ok {
		r0 = rf(ctx, txOptions, f)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields:
func (_m *Pool) Close() {
	_m.Called()
}

// Config provides a mock function with given fields:
func (_m *Pool) Config() *pgxpool.Config {
	ret := _m.Called()

	var r0 *pgxpool.Config
	if rf, ok := ret.Get(0).(func() *pgxpool.Config); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pgxpool.Config)
		}
	}

	return r0
}

// CopyFrom provides a mock function with given fields: ctx, tableName, columnNames, rowSrc
func (_m *Pool) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	ret := _m.Called(ctx, tableName, columnNames, rowSrc)

	var r0 int64
	if rf, ok := ret.Get(0).(func(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) int64); ok {
		r0 = rf(ctx, tableName, columnNames, rowSrc)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) error); ok {
		r1 = rf(ctx, tableName, columnNames, rowSrc)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Exec provides a mock function with given fields: ctx, sql, arguments
func (_m *Pool) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	var _ca []interface{}
	_ca = append(_ca, ctx, sql)
	_ca = append(_ca, arguments...)
	ret := _m.Called(_ca...)

	var r0 pgconn.CommandTag
	if rf, ok := ret.Get(0).(func(context.Context, string, ...interface{}) pgconn.CommandTag); ok {
		r0 = rf(ctx, sql, arguments...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgconn.CommandTag)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, ...interface{}) error); ok {
		r1 = rf(ctx, sql, arguments...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ping provides a mock function with given fields: ctx
func (_m *Pool) Ping(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Query provides a mock function with given fields: ctx, sql, args
func (_m *Pool) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	var _ca []interface{}
	_ca = append(_ca, ctx, sql)
	_ca = append(_ca, args...)
	ret := _m.Called(_ca...)

	var r0 pgx.Rows
	if rf, ok := ret.Get(0).(func(context.Context, string, ...interface{}) pgx.Rows); ok {
		r0 = rf(ctx, sql, args...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgx.Rows)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, ...interface{}) error); ok {
		r1 = rf(ctx, sql, args...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// QueryFunc provides a mock function with given fields: ctx, sql, args, scans, f
func (_m *Pool) QueryFunc(ctx context.Context, sql string, args []interface{}, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	ret := _m.Called(ctx, sql, args, scans, f)

	var r0 pgconn.CommandTag
	if rf, ok := ret.Get(0).(func(context.Context, string, []interface{}, []interface{}, func(pgx.QueryFuncRow) error) pgconn.CommandTag); ok {
		r0 = rf(ctx, sql, args, scans, f)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgconn.CommandTag)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, []interface{}, []interface{}, func(pgx.QueryFuncRow) error) error); ok {
		r1 = rf(ctx, sql, args, scans, f)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// QueryRow provides a mock function with given fields: ctx, sql, args
func (_m *Pool) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	var _ca []interface{}
	_ca = append(_ca, ctx, sql)
	_ca = append(_ca, args...)
	ret := _m.Called(_ca...)

	var r0 pgx.Row
	if rf, ok := ret.Get(0).(func(context.Context, string, ...interface{}) pgx.Row); ok {
		r0 = rf(ctx, sql, args...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgx.Row)
		}
	}

	return r0
}

// SendBatch provides a mock function with given fields: ctx, b
func (_m *Pool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	ret := _m.Called(ctx, b)

	var r0 pgx.BatchResults
	if rf, ok := ret.Get(0).(func(context.Context, *pgx.Batch) pgx.BatchResults); ok {
		r0 = rf(ctx, b)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pgx.BatchResults)
		}
	}

	return r0
}

// NewPool creates a new instance of Pool. It also registers a cleanup function to assert the mocks expectations.
func NewPool(t testing.TB) *Pool {
	mock := &Pool{}

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}