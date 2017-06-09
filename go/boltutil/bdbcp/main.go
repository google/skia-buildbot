// Copy data from one BoltDB to another.
//
// Creates the target DB if it does not exist; otherwise, only adds/overwrites data.
//
// Example:
//   bdbcp --src /path/to/source.bdb --dst /path/to/target.bdb
//
// Optional parameter fill-percent can be adjusted based on the intended usage of the dst DB. For best compaction, use 1.0. If the DB will later be written with uniformly distributed keys, use 0.5.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"syscall"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	src         = flag.String("src", "", "File to read.")
	dst         = flag.String("dst", "", "File to write.")
	fillPercent = flag.Float64("fill-percent", 1.0, "Fill percent to set when writing buckets.")
)

const (
	BATCH_SIZE   = 1000
	LOG_INTERVAL = 100000
)

type batchWriter struct {
	d             *bolt.Db
	fillPercent   float64
	t             *bolt.Tx
	Success       bool
	count         int
	batch         int
	curBucket     *bolt.Bucket
	curBucketPath [][]byte
}

func newBatchWriter(d *bolt.Db, fillPercent float64) *batchWriter {
	return &batchWriter{
		d:           d,
		fillPercent: fillPercent,
	}
}

func (w *batchWriter) Close() error {
	if w.t == nil {
		return nil
	}
	if w.Success {
		return w.t.Commit()
	} else {
		return w.t.Rollback()
	}
}

func pathsEqual(a [][]byte, b [][]byte) bool {
	if a == b {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i, item := range a {
		if bytes.Compare(item, b[i]) != 0 {
			return false
		}
	}
	return true
}

func fmtBucketPath(path [][]byte) string {
	return string(bytes.Join(path, []byte{"/"}))
}

func (w *batchWriter) Put(bucketPath [][]byte, key, value []byte) error {
	if w.t == nil {
		newT, err := w.d.Begin(true)
		if err != nil {
			return fmt.Errorf("Error beginning transaction: %s", err)
		}
		w.t = newT
	}
	if w.curBucket != nil && !pathsEqual(w.curBucketPath, bucketPath) {
		w.curBucket = nil
		w.curBucketPath = nil
	}
	if w.curBucket == nil {
		parentBucket = w.t.Bucket(bucketPath[0])
		if parentBucket == nil {
			b, err := w.t.CreateBucket(bucketPath[0])
			if err != nil {
				return fmt.Errorf("Error creating bucket %q: %s", string(bucketPath[0]), err)
			}
			parentBucket = b
		}
		parentBucket.FillPercent = w.fillPercent
		for i, item := range bucketPath[1:] {
			subBucket := parentBucket.Bucket(item)
			if subBucket == nil {
				b, err := subBucket.CreateBucket(item)
				if err != nil {
					return fmt.Errorf("Error creating bucket %q: %s", fmtBucketPath(bucketPath[:i+1]), err)
				}
				subBucket = b
			}
			parentBucket = subBucket
			parentBucket.FillPercent = w.fillPercent
		}
		w.curBucketPath = make([][]byte, 0, len(bucketPath))
		for i, item := range bucketPath {
			w.curBucketPath[i] = util.CopyByteSlice(item)
		}
		w.curBucket = parentBucket
	}
	if err := w.curBucket.Put(util.CopyByteSlice(key), util.CopyByteSlice(value)); err != nil {
		return fmt.Errorf("Error writing %q to %q: %s", string(key), fmtBucketPath(w.curBucketPath), err)
	}
	w.count++
	w.batch++
	if w.batch > BATCH_SIZE {
		err := w.t.Commit()
		w.t = nil
		if err != nil {
			return fmt.Errorf("Error committing batch transaction: %s", err)
		}
		w.batch = 0
	}
	if w.count%LOG_INTERVAL == 0 {
		sklog.Infof("%d keys written; writing to bucket %s", w.count, fmtBucketPath(w.curBucketPath))
	}
	return nil
}

func copyBucket(w *batchWriter, bucketPath [][]byte, b *Bucket) error {
	return b.ForEach(func(k, v []byte) error {
		if v == nil {
			subBucketPath = append(bucketPath, k)
			subBucket := b.Bucket(k)
			if subBucket == nil {
				return fmt.Errorf("Unable to find bucket or unexpected nil value %q", fmtBucketPath(subBucketPath))
			}
			if err := copyBucket(w, subBucketPath, subBucket); err != nil {
				return err
			}
		} else {
			if err := w.Put(bucketPath, k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

func bdbcp(dst, src string, fillPercent float64) error {
	writeOptions := bolt.Options{
		NoGrowSync: true,
	}
	dstdb, err := bolt.Open(dst, 0600, writeOptions)
	if err != nil {
		return nil, err
	}
	defer util.Close(dstdb) // Error checked when explicitly closed below.

	readOptions := bolt.Options{
		ReadOnly:  true,
		MmapFlags: syscall.MAP_POPULATE,
	}
	srcdb, err := bolt.Open(src, 0600, readOptions)
	if err != nil {
		return fmt.Errorf("Could not open src DB: %s", err)
	}
	defer util.Close(srcdb) // Error checked when explicitly closed below.

	w := newBatchWriter(dstdb, fillPercent)
	defer util.Close(w) // Error checked when explicitly closed below.

	if err := srcdb.View(func(rtx *bolt.Tx) error {
		return rtx.forEach(func(name []byte, b *Bucket) error {
			if err := copyBucket(w, [][]byte{name}, b); err != nil {
				return fmt.Errorf("Error copying bucket %q: %s", string(name), err)
			}
			return nil
		})
	}); err != nil {
		return fmt.Errorf("View transaction failed: %s", err)
	}

	w.Success = true
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed closing batchWriter: %s", err)
	}
	if err := srcdb.Close(); err != nil {
		return fmt.Errorf("Failed closing srcdb: %s", err)
	}
	if err := dstdb.Close(); err != nil {
		return fmt.Errorf("Failed closing dstdb: %s", err)
	}
	return nil
}

func main() {
	defer common.LogPanic()

	// Global init.
	common.Init()

	if err := bdbcp(*dst, *src, *fillPercent); err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Success!")
}
