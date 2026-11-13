// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tracereader

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/types"
)

// fakeRows implements the pgx.Rows methods the reader uses; the embedded nil interface panics if
// any other method is called.
type fakeRows struct {
	pgx.Rows
	rows   [][2]interface{}
	i      int
	closed bool
}

func (f *fakeRows) Next() bool {
	f.i++
	return f.i <= len(f.rows)
}

func (f *fakeRows) Scan(dest ...interface{}) error {
	row := f.rows[f.i-1]
	*dest[0].(*int64) = row[0].(int64)
	*dest[1].(*float32) = row[1].(float32)
	return nil
}

func (f *fakeRows) Err() error { return nil }
func (f *fakeRows) Close()     { f.closed = true }

type fakeDB struct {
	sql  string
	args []interface{}
	rows *fakeRows
	err  error
}

func (f *fakeDB) Query(_ context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	f.sql, f.args = sql, args
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

const key = ",benchmark=v8,bot=x64,master=internal.client.v8,test=JetStream2,"

func TestReadTrace_QueriesByTraceIDAndRangeWithParameters(t *testing.T) {
	db := &fakeDB{rows: &fakeRows{rows: [][2]interface{}{{int64(10), float32(1.5)}, {int64(12), float32(2.5)}}}}
	commits, values, err := New(db).ReadTrace(context.Background(), key, 10, 20)
	require.NoError(t, err)
	assert.Equal(t, []types.CommitNumber{10, 12}, commits)
	assert.Equal(t, []float32{1.5, 2.5}, values)
	assert.True(t, db.rows.closed)

	id := types.TraceIDForSQLInBytesFromTraceName(key)
	assert.Equal(t, []interface{}{id[:], int64(10), int64(20), MaxRows}, db.args)
	assert.NotContains(t, db.sql, "JetStream2", "values are parameters, not part of the SQL")
}

func TestReadTrace_RejectsInvalidInputWithoutQuerying(t *testing.T) {
	db := &fakeDB{rows: &fakeRows{}}
	_, _, err := New(db).ReadTrace(context.Background(), "'; DROP TABLE TraceValues; --", 1, 2)
	assert.Error(t, err)
	_, _, err = New(db).ReadTrace(context.Background(), key, 5, 4)
	assert.Error(t, err)
	_, _, err = New(db).ReadTrace(context.Background(), key, -1, 4)
	assert.Error(t, err)
	assert.Empty(t, db.sql)
}

func TestReadTrace_QueryErrorsAreReturned(t *testing.T) {
	_, _, err := New(&fakeDB{err: errors.New("boom")}).ReadTrace(context.Background(), key, 1, 2)
	assert.Error(t, err)
}
