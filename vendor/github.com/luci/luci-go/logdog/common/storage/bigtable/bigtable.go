// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bigtable

import (
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/common/storage"
	"golang.org/x/net/context"
)

const (
	logColumnFamily = "log"

	// The data column stores raw low row data (RecordIO blob).
	logColumn  = "data"
	logColName = logColumnFamily + ":" + logColumn
)

// Limits taken from here:
// https://cloud.google.com/bigtable/docs/schema-design
const (
	// bigTableRowMaxBytes is the maximum number of bytes that a single BigTable
	// row may hold.
	bigTableRowMaxBytes = 1024 * 1024 * 10 // 10MB
)

// btGetCallback is a callback that is invoked for each log data row returned
// by getLogData.
//
// If an error is encountered, no more log data will be fetched. The error will
// be propagated to the getLogData call.
type btGetCallback func(*rowKey, []byte) error

// btTable is a general interface for BigTable operations intended to enable
// unit tests to stub out BigTable without adding runtime inefficiency.
type btTable interface {
	// putLogData adds new log data to BigTable.
	//
	// If data already exists for the named row, it will return storage.ErrExists
	// and not add the data.
	putLogData(context.Context, *rowKey, []byte) error

	// getLogData retrieves rows belonging to the supplied stream record, starting
	// with the first index owned by that record. The supplied callback is invoked
	// once per retrieved row.
	//
	// rk is the starting row key.
	//
	// If the supplied limit is nonzero, no more than limit rows will be
	// retrieved.
	//
	// If keysOnly is true, then the callback will return nil row data.
	getLogData(c context.Context, rk *rowKey, limit int, keysOnly bool, cb btGetCallback) error

	// setMaxLogAge updates the maximum log age policy for the log family.
	setMaxLogAge(context.Context, time.Duration) error
}

// btTableProd is an implementation of the btTable interface that uses a real
// production BigTable connection.
type btTableProd struct {
	*btStorage
}

func (t *btTableProd) putLogData(c context.Context, rk *rowKey, data []byte) error {
	m := bigtable.NewMutation()
	m.Set(logColumnFamily, logColumn, bigtable.ServerTime, data)
	cm := bigtable.NewCondMutation(bigtable.RowKeyFilter(rk.encode()), nil, m)

	rowExists := false
	if err := t.logTable.Apply(c, rk.encode(), cm, bigtable.GetCondMutationResult(&rowExists)); err != nil {
		return grpcutil.WrapIfTransient(err)
	}
	if rowExists {
		return storage.ErrExists
	}
	return nil
}

func (t *btTableProd) getLogData(c context.Context, rk *rowKey, limit int, keysOnly bool, cb btGetCallback) error {
	// Construct read options based on Get request.
	ropts := []bigtable.ReadOption{
		bigtable.RowFilter(bigtable.FamilyFilter(logColumnFamily)),
		bigtable.RowFilter(bigtable.ColumnFilter(logColumn)),
		nil,
	}[:2]
	if keysOnly {
		ropts = append(ropts, bigtable.RowFilter(bigtable.StripValueFilter()))
	}
	if limit > 0 {
		ropts = append(ropts, bigtable.LimitRows(int64(limit)))
	}

	// This will limit the range to the immediate row key ("ASDF~INDEX") to
	// immediately after the row key ("ASDF~~"). See rowKey for more information.
	rng := bigtable.NewRange(rk.encode(), rk.pathPrefixUpperBound())

	var innerErr error
	err := t.logTable.ReadRows(c, rng, func(row bigtable.Row) bool {
		data, err := getLogRowData(row)
		if err != nil {
			innerErr = storage.ErrBadData
			return false
		}

		drk, err := decodeRowKey(row.Key())
		if err != nil {
			innerErr = err
			return false
		}

		if err := cb(drk, data); err != nil {
			innerErr = err
			return false
		}
		return true
	}, ropts...)
	if err != nil {
		return grpcutil.WrapIfTransient(err)
	}
	if innerErr != nil {
		return innerErr
	}
	return nil
}

func (t *btTableProd) setMaxLogAge(c context.Context, d time.Duration) error {
	var logGCPolicy bigtable.GCPolicy
	if d > 0 {
		logGCPolicy = bigtable.MaxAgePolicy(d)
	}
	if err := t.adminClient.SetGCPolicy(c, t.LogTable, logColumnFamily, logGCPolicy); err != nil {
		return grpcutil.WrapIfTransient(err)
	}
	return nil
}

// getLogRowData loads the []byte contents of the supplied log row.
//
// If the row doesn't exist, storage.ErrDoesNotExist will be returned.
func getLogRowData(row bigtable.Row) (data []byte, err error) {
	items, ok := row[logColumnFamily]
	if !ok {
		err = storage.ErrDoesNotExist
		return
	}

	for _, item := range items {
		switch item.Column {
		case logColName:
			data = item.Value
			return
		}
	}

	// If no fields could be extracted, the rows does not exist.
	err = storage.ErrDoesNotExist
	return
}

// getReadItem retrieves a specific RowItem from the supplied Row.
func getReadItem(row bigtable.Row, family, column string) *bigtable.ReadItem {
	// Get the row for our family.
	items, ok := row[logColumnFamily]
	if !ok {
		return nil
	}

	// Get the specific ReadItem for our column
	colName := fmt.Sprintf("%s:%s", family, column)
	for _, item := range items {
		if item.Column == colName {
			return &item
		}
	}
	return nil
}
