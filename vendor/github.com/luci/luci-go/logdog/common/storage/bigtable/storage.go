// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bigtable

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/recordio"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"

	"cloud.google.com/go/bigtable"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

var (
	// StorageScopes is the set of OAuth scopes needed to use the storage
	// functionality.
	StorageScopes = []string{
		bigtable.Scope,
	}

	// StorageReadOnlyScopes is the set of OAuth scopes needed to use the storage
	// functionality.
	StorageReadOnlyScopes = []string{
		bigtable.ReadonlyScope,
	}
)

const (
	// tailRowCount is the size of the block of rows that tail read operations
	// pull from BigTable. This is designed to be large enough for efficient
	// buffering while staying small enough to avoid wasteful reads or
	// excessive in-memory buffering.
	//
	// This is simply the maximum number of rows (limit). The actual number of
	// rows will be further constrained by tailRowMaxSize.
	tailRowCount = 128

	// tailRowMaxSize is the maximum number of bytes of tail row data that will be
	// buffered during Tail row reading.
	tailRowMaxSize = 1024 * 1024 * 16
)

var (
	// errStop is an internal sentinel error used to indicate "stop iteration"
	// to btTable.getLogData iterator.
	errStop = errors.New("bigtable: stop iteration")
)

// Options is a set of configuration options for BigTable storage.
type Options struct {
	// Project is the name of the project to connect to.
	Project string
	// Instance is the name of the instance to connect to.
	Instance string
	// ClientOptions are additional client options to use when instantiating the
	// client instance.
	ClientOptions []option.ClientOption

	// Table is the name of the BigTable table to use for logs.
	LogTable string
}

func (o *Options) client(ctx context.Context) (*bigtable.Client, error) {
	return bigtable.NewClient(ctx, o.Project, o.Instance, o.ClientOptions...)
}

func (o *Options) adminClient(ctx context.Context) (*bigtable.AdminClient, error) {
	return bigtable.NewAdminClient(ctx, o.Project, o.Instance, o.ClientOptions...)
}

// btStorage is a storage.Storage implementation that uses Google Cloud BigTable
// as a backend.
type btStorage struct {
	*Options

	// Context is the bound supplied with New. It is retained (rather than
	// supplied on a per-call basis) because a special Storage Context devoid of
	// gRPC metadata is needed for Storage calls.
	context.Context

	client      *bigtable.Client
	logTable    *bigtable.Table
	adminClient *bigtable.AdminClient

	// raw is the underlying btTable instance to use for raw operations.
	raw btTable
	// maxRowSize is the maxmium number of bytes that can be stored in a single
	// BigTable row. This is a function of BigTable, and constant in production
	// (bigTableRowMaxBytes), but variable here to allow for testing to control.
	maxRowSize int
}

// New instantiates a new Storage instance connected to a BigTable instance.
//
// The returned Storage instance will close the Client when its Close() method
// is called.
func New(ctx context.Context, o Options) (storage.Storage, error) {
	client, err := o.client(ctx)
	if err != nil {
		return nil, err
	}

	admin, err := o.adminClient(ctx)
	if err != nil {
		return nil, err
	}

	return newBTStorage(ctx, o, client, admin), nil
}

func newBTStorage(ctx context.Context, o Options, client *bigtable.Client, adminClient *bigtable.AdminClient) *btStorage {
	s := &btStorage{
		Options: &o,
		Context: ctx,

		client:      client,
		adminClient: adminClient,
		maxRowSize:  bigTableRowMaxBytes,
	}
	if s.client != nil {
		s.logTable = s.client.Open(o.LogTable)
	}
	s.raw = &btTableProd{s}
	return s
}

func (s *btStorage) Close() {
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	if s.adminClient != nil {
		s.adminClient.Close()
		s.adminClient = nil
	}
}

func (s *btStorage) Config(cfg storage.Config) error {
	if err := s.raw.setMaxLogAge(s, cfg.MaxLogAge); err != nil {
		log.WithError(err).Errorf(s, "Failed to set 'log' GC policy.")
		return err
	}
	log.Fields{
		"maxLogAge": cfg.MaxLogAge,
	}.Infof(s, "Set maximum log age.")
	return nil
}

func (s *btStorage) Put(r storage.PutRequest) error {
	rw := rowWriter{
		threshold: s.maxRowSize,
	}

	for len(r.Values) > 0 {
		// Add the next entry to the writer.
		if appended := rw.append(r.Values[0]); !appended {
			// We have failed to append our maximum BigTable row size. Flush any
			// currently-buffered row data and try again with an empty buffer.
			count, err := rw.flush(s, s.raw, r.Index, r.Project, r.Path)
			if err != nil {
				return err
			}

			if count == 0 {
				// Nothing was buffered, but we still couldn't append an entry. The
				// current entry is too large by itself, so we must fail.
				return fmt.Errorf("single row entry exceeds maximum size (%d > %d)", len(r.Values[0]), bigTableRowMaxBytes)
			}

			r.Index += types.MessageIndex(count)
			continue
		}

		// We successfully appended this entry, so advance.
		r.Values = r.Values[1:]
	}

	// Flush any buffered rows.
	if _, err := rw.flush(s, s.raw, r.Index, r.Project, r.Path); err != nil {
		return err
	}
	return nil
}

func (s *btStorage) Get(r storage.GetRequest, cb storage.GetCallback) error {
	startKey := newRowKey(string(r.Project), string(r.Path), int64(r.Index), 0)
	ctx := log.SetFields(s, log.Fields{
		"project":     r.Project,
		"path":        r.Path,
		"index":       r.Index,
		"limit":       r.Limit,
		"startRowKey": startKey,
		"keysOnly":    r.KeysOnly,
	})

	// If we issue a query and get back a legacy row, it will have no count
	// associated with it. We will fast-exit

	limit := r.Limit
	err := s.raw.getLogData(ctx, startKey, r.Limit, r.KeysOnly, func(rk *rowKey, data []byte) error {
		// Does this key match our requested log stream? If not, we've moved past
		// this stream's records and must stop iteration.
		if !rk.sharesPathWith(startKey) {
			return errStop
		}

		// Calculate the start index of the contiguous row. Since we index the row
		// on the LAST entry in the row, count backwards to get the index of the
		// first entry.
		startIndex := rk.index - rk.count + 1
		if startIndex < 0 {
			return storage.ErrBadData
		}

		// Split our data into records. Leave the records slice nil if we're doing
		// a keys-only get.
		var records [][]byte
		if !r.KeysOnly {
			var err error
			if records, err = recordio.Split(data); err != nil {
				return storage.ErrBadData
			}

			if rk.count != int64(len(records)) {
				log.Fields{
					"count":       rk.count,
					"recordCount": len(records),
				}.Errorf(ctx, "Record count doesn't match declared count.")
				return storage.ErrBadData
			}
		}

		// If we are indexed somewhere within this entry's records, discard any
		// records before our index.
		if discard := int64(r.Index) - startIndex; discard > 0 {
			if discard > rk.count {
				// This should never happen unless there is corrupt or conflicting data.
				return nil
			}
			startIndex += discard
			if !r.KeysOnly {
				records = records[discard:]
			}
		}

		log.Fields{
			"rk":         rk.encode(),
			"rkIndex":    rk.index,
			"rkCount":    rk.count,
			"startIndex": startIndex,
		}.Debugf(ctx, "Punting row key range [%d - %d]...", startIndex, rk.index)

		for index := startIndex; index <= rk.index; index++ {
			// If we're not doing keys-only, consume the row.
			var row []byte
			if !r.KeysOnly {
				row, records = records[0], records[1:]
			}

			if !cb(types.MessageIndex(index), row) {
				return errStop
			}
			r.Index = types.MessageIndex(index + 1)

			// Artificially apply limit within our row records.
			if limit > 0 {
				limit--
				if limit == 0 {
					return errStop
				}
			}
		}
		return nil
	})

	switch err {
	case nil, errStop:
		return nil

	default:
		log.WithError(err).Errorf(ctx, "Failed to retrieve row range.")
		return err
	}
}

func (s *btStorage) Tail(project config.ProjectName, path types.StreamPath) ([]byte, types.MessageIndex, error) {
	ctx := log.SetFields(s, log.Fields{
		"project": project,
		"path":    path,
	})

	// Iterate through all log keys in the stream. Record the latest one.
	rk := newRowKey(string(project), string(path), 0, 0)
	var latest *rowKey
	err := s.raw.getLogData(ctx, rk, 0, true, func(rk *rowKey, data []byte) error {
		latest = rk
		return nil
	})
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"project":    s.Project,
			"instance":   s.Instance,
			"table":      s.LogTable,
		}.Errorf(ctx, "Failed to scan for tail.")
	}

	if latest == nil {
		// No rows for the specified stream.
		return nil, 0, storage.ErrDoesNotExist
	}

	// Fetch the latest row's data.
	var d []byte
	err = s.raw.getLogData(ctx, latest, 1, false, func(rk *rowKey, data []byte) error {
		records, err := recordio.Split(data)
		if err != nil || len(records) == 0 {
			return storage.ErrBadData
		}
		d = records[len(records)-1]
		return errStop
	})
	if err != nil && err != errStop {
		log.Fields{
			log.ErrorKey: err,
			"project":    s.Project,
			"instance":   s.Instance,
			"table":      s.LogTable,
		}.Errorf(ctx, "Failed to retrieve tail row.")
	}

	return d, types.MessageIndex(latest.index), nil
}

// rowWriter facilitates writing several consecutive data values to a single
// BigTable row.
type rowWriter struct {
	// buf is the current set of buffered data.
	buf bytes.Buffer

	// count is the number of rows in the writer.
	count int

	// threshold is the maximum number of bytes that we can write.
	threshold int
}

func (w *rowWriter) append(d []byte) (appended bool) {
	origSize := w.buf.Len()
	defer func() {
		// Restore our previous buffer state if we are reporting the write as
		// failed.
		if !appended {
			w.buf.Truncate(origSize)
		}
	}()

	// Serialize the next entry as a recordio blob.
	if _, err := recordio.WriteFrame(&w.buf, d); err != nil {
		return
	}

	// If we have exceeded our threshold, report a failure.
	appended = (w.buf.Len() <= w.threshold)
	if appended {
		w.count++
	}
	return
}

func (w *rowWriter) flush(ctx context.Context, raw btTable, index types.MessageIndex,
	project config.ProjectName, path types.StreamPath) (int, error) {
	flushCount := w.count
	if flushCount == 0 {
		return 0, nil
	}

	// Write the current set of buffered rows to the table. Index on the LAST
	// row index.
	lastIndex := int64(index) + int64(flushCount) - 1
	rk := newRowKey(string(project), string(path), lastIndex, int64(w.count))

	log.Fields{
		"rowKey":    rk,
		"project":   project,
		"path":      path,
		"index":     index,
		"lastIndex": lastIndex,
		"count":     w.count,
		"size":      w.buf.Len(),
	}.Debugf(ctx, "Adding entries to BigTable.")
	if err := raw.putLogData(ctx, rk, w.buf.Bytes()); err != nil {
		return 0, err
	}

	// Reset our buffer state.
	w.buf.Reset()
	w.count = 0
	return flushCount, nil
}
