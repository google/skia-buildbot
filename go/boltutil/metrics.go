package boltutil

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/metrics2"
)

// TxStatsMetric contains sub-metrics for each field of the bolt.TxStats from
// bolt.DB.Stats(). Create via NewDbMetric.
//
// TxStatsMetric does not use aggregating metrics, so it's unlikely to work well
// for per-Tx TxStats.
type TxStatsMetric struct {
	// Page statistics.
	PageCount *metrics2.Int64Metric // number of page allocations
	PageAlloc *metrics2.Int64Metric // total bytes allocated

	// Cursor statistics.
	CursorCount *metrics2.Int64Metric // number of cursors created

	// Node statistics
	NodeCount *metrics2.Int64Metric // number of node allocations
	NodeDeref *metrics2.Int64Metric // number of node dereferences

	// Rebalance statistics.
	Rebalance     *metrics2.Int64Metric // number of node rebalances
	RebalanceTime *metrics2.Int64Metric // total time spent rebalancing

	// Split/Spill statistics.
	Split     *metrics2.Int64Metric // number of nodes split
	Spill     *metrics2.Int64Metric // number of nodes spilled
	SpillTime *metrics2.Int64Metric // total time spent spilling

	// Write statistics.
	Write     *metrics2.Int64Metric // number of writes performed
	WriteTime *metrics2.Int64Metric // total time spent writing to disk
}

// newTxStatsMetric initializes a TxStatsMetric. tags should include "database"
// but should not include "metric".
func newTxStatsMetric(c *metrics2.Client, tags ...map[string]string) *TxStatsMetric {
	return &TxStatsMetric{
		PageCount:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "PageCount"})...),
		PageAlloc:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "PageAllocBytes"})...),
		CursorCount:   c.GetInt64Metric("db", append(tags, map[string]string{"metric": "CursorCount"})...),
		NodeCount:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "NodeCount"})...),
		NodeDeref:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "NodeDerefCount"})...),
		Rebalance:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "RebalanceCount"})...),
		RebalanceTime: c.GetInt64Metric("db", append(tags, map[string]string{"metric": "RebalanceNs"})...),
		Split:         c.GetInt64Metric("db", append(tags, map[string]string{"metric": "SplitCount"})...),
		Spill:         c.GetInt64Metric("db", append(tags, map[string]string{"metric": "SpillCount"})...),
		SpillTime:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "SpillNs"})...),
		Write:         c.GetInt64Metric("db", append(tags, map[string]string{"metric": "WriteCount"})...),
		WriteTime:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "WriteNs"})...),
	}
}

// Update sets all sub-metrics from cur.
func (m *TxStatsMetric) Update(cur bolt.TxStats) {
	m.PageCount.Update(int64(cur.PageCount))
	m.PageAlloc.Update(int64(cur.PageAlloc))
	m.CursorCount.Update(int64(cur.CursorCount))
	m.NodeCount.Update(int64(cur.NodeCount))
	m.NodeDeref.Update(int64(cur.NodeDeref))
	m.Rebalance.Update(int64(cur.Rebalance))
	m.RebalanceTime.Update(cur.RebalanceTime.Nanoseconds())
	m.Split.Update(int64(cur.Split))
	m.Spill.Update(int64(cur.Spill))
	m.SpillTime.Update(cur.SpillTime.Nanoseconds())
	m.Write.Update(int64(cur.Write))
	m.WriteTime.Update(cur.WriteTime.Nanoseconds())
}

func deleteAll(metrics ...*metrics2.Int64Metric) error {
	for _, metric := range metrics {
		if err := metric.Delete(); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes all sub-metrics.
func (m *TxStatsMetric) Delete() error {
	return deleteAll(
		m.PageCount,
		m.PageAlloc,
		m.CursorCount,
		m.NodeCount,
		m.NodeDeref,
		m.Rebalance,
		m.RebalanceTime,
		m.Split,
		m.Spill,
		m.SpillTime,
		m.Write,
		m.WriteTime,
	)
}

// DbStatsMetric contains sub-metrics for each field of bolt.Stats. Create via
// NewDbMetric.
type DbStatsMetric struct {
	// Freelist stats
	FreePageN     *metrics2.Int64Metric // total number of free pages on the freelist
	PendingPageN  *metrics2.Int64Metric // total number of pending pages on the freelist
	FreeAlloc     *metrics2.Int64Metric // total bytes allocated in free pages
	FreelistInuse *metrics2.Int64Metric // total bytes used by the freelist

	// Transaction stats
	TxN     *metrics2.Int64Metric // total number of started read transactions
	OpenTxN *metrics2.Int64Metric // number of currently open read transactions

	TxStatsMetric *TxStatsMetric // global, ongoing stats.
}

// newDbStatsMetric initializes a DbStatsMetric. tags should include "database"
// but should not include "metric".
func newDbStatsMetric(c *metrics2.Client, tags ...map[string]string) *DbStatsMetric {
	return &DbStatsMetric{
		FreePageN:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "FreePageCount"})...),
		PendingPageN:  c.GetInt64Metric("db", append(tags, map[string]string{"metric": "PendingPageCount"})...),
		FreeAlloc:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "FreeAllocBytes"})...),
		FreelistInuse: c.GetInt64Metric("db", append(tags, map[string]string{"metric": "FreelistInuseBytes"})...),
		TxN:           c.GetInt64Metric("db", append(tags, map[string]string{"metric": "TxCount"})...),
		OpenTxN:       c.GetInt64Metric("db", append(tags, map[string]string{"metric": "OpenTxCount"})...),
		TxStatsMetric: newTxStatsMetric(c, tags...),
	}
}

// Update sets sub-metrics based on cur.
func (m *DbStatsMetric) Update(cur bolt.Stats) {
	m.FreePageN.Update(int64(cur.FreePageN))
	m.PendingPageN.Update(int64(cur.PendingPageN))
	m.FreeAlloc.Update(int64(cur.FreeAlloc))
	m.FreelistInuse.Update(int64(cur.FreelistInuse))
	m.TxN.Update(int64(cur.TxN))
	m.OpenTxN.Update(int64(cur.OpenTxN))
	m.TxStatsMetric.Update(cur.TxStats)
}

// Delete deletes all sub-metrics.
func (m *DbStatsMetric) Delete() error {
	if err := m.TxStatsMetric.Delete(); err != nil {
		return err
	}
	return deleteAll(
		m.FreePageN,
		m.PendingPageN,
		m.FreeAlloc,
		m.FreelistInuse,
		m.TxN,
		m.OpenTxN,
	)
}

// BucketStatsMetric contains sub-metrics for each field of bolt.BucketStats.
// Create via NewDbMetric.
type BucketStatsMetric struct {
	// Page count statistics.
	BranchPageN     *metrics2.Int64Metric // number of logical branch pages
	BranchOverflowN *metrics2.Int64Metric // number of physical branch overflow pages
	LeafPageN       *metrics2.Int64Metric // number of logical leaf pages
	LeafOverflowN   *metrics2.Int64Metric // number of physical leaf overflow pages

	// Tree statistics.
	KeyN  *metrics2.Int64Metric // number of keys/value pairs
	Depth *metrics2.Int64Metric // number of levels in B+tree

	// Page size utilization.
	BranchAlloc *metrics2.Int64Metric // bytes allocated for physical branch pages
	BranchInuse *metrics2.Int64Metric // bytes actually used for branch data
	LeafAlloc   *metrics2.Int64Metric // bytes allocated for physical leaf pages
	LeafInuse   *metrics2.Int64Metric // bytes actually used for leaf data

	// Bucket statistics
	BucketN           *metrics2.Int64Metric // total number of buckets including the top bucket
	InlineBucketN     *metrics2.Int64Metric // total number of inlined buckets
	InlineBucketInuse *metrics2.Int64Metric // bytes used for inlined buckets (also accounted for in LeafInuse)
}

// newBucketStatsMetric initializes a BucketStatsMetric. tags should include
// "database" and "bucket-path" but not "metric".
func newBucketStatsMetric(c *metrics2.Client, tags ...map[string]string) *BucketStatsMetric {
	return &BucketStatsMetric{
		BranchPageN:       c.GetInt64Metric("db", append(tags, map[string]string{"metric": "BranchPageCount"})...),
		BranchOverflowN:   c.GetInt64Metric("db", append(tags, map[string]string{"metric": "BranchOverflowCount"})...),
		LeafPageN:         c.GetInt64Metric("db", append(tags, map[string]string{"metric": "LeafPageCount"})...),
		LeafOverflowN:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "LeafOverflowCount"})...),
		KeyN:              c.GetInt64Metric("db", append(tags, map[string]string{"metric": "KeyCount"})...),
		Depth:             c.GetInt64Metric("db", append(tags, map[string]string{"metric": "DepthCount"})...),
		BranchAlloc:       c.GetInt64Metric("db", append(tags, map[string]string{"metric": "BranchAllocBytes"})...),
		BranchInuse:       c.GetInt64Metric("db", append(tags, map[string]string{"metric": "BranchInuseBytes"})...),
		LeafAlloc:         c.GetInt64Metric("db", append(tags, map[string]string{"metric": "LeafAllocBytes"})...),
		LeafInuse:         c.GetInt64Metric("db", append(tags, map[string]string{"metric": "LeafInuseBytes"})...),
		BucketN:           c.GetInt64Metric("db", append(tags, map[string]string{"metric": "BucketCount"})...),
		InlineBucketN:     c.GetInt64Metric("db", append(tags, map[string]string{"metric": "InlineBucketCount"})...),
		InlineBucketInuse: c.GetInt64Metric("db", append(tags, map[string]string{"metric": "InlineBucketInuseBytes"})...),
	}
}

// Update sets all sub-metrics from cur.
func (m *BucketStatsMetric) Update(cur bolt.BucketStats) {
	m.BranchPageN.Update(int64(cur.BranchPageN))
	m.BranchOverflowN.Update(int64(cur.BranchOverflowN))
	m.LeafPageN.Update(int64(cur.LeafPageN))
	m.LeafOverflowN.Update(int64(cur.LeafOverflowN))
	m.KeyN.Update(int64(cur.KeyN))
	m.Depth.Update(int64(cur.Depth))
	m.BranchAlloc.Update(int64(cur.BranchAlloc))
	m.BranchInuse.Update(int64(cur.BranchInuse))
	m.LeafAlloc.Update(int64(cur.LeafAlloc))
	m.LeafInuse.Update(int64(cur.LeafInuse))
	m.BucketN.Update(int64(cur.BucketN))
	m.InlineBucketN.Update(int64(cur.InlineBucketN))
	m.InlineBucketInuse.Update(int64(cur.InlineBucketInuse))
}

// Delete deletes all sub-metrics.
func (m *BucketStatsMetric) Delete() error {
	return deleteAll(
		m.BranchPageN,
		m.BranchOverflowN,
		m.LeafPageN,
		m.LeafOverflowN,
		m.KeyN,
		m.Depth,
		m.BranchAlloc,
		m.BranchInuse,
		m.LeafAlloc,
		m.LeafInuse,
		m.BucketN,
		m.InlineBucketN,
		m.InlineBucketInuse,
	)
}

// DbMetric gathers and reports a number of statistics about a BoltDB using the
// metrics2 package.
type DbMetric struct {
	Liveness           *metrics2.Liveness
	DbStatsMetric      *DbStatsMetric
	BucketStatsMetrics map[string]*BucketStatsMetric
	db                 *bolt.DB
	stop               chan bool
}

// NewDbMetric initializes a DbMetric and starts a goroutine to periodically
// update the sub-metrics from the given bolt.DB. Bucket stats are reported only
// for the given buckets. tags should include "database" and should not include
// "metric" or "bucket-path". Returns an error if the initial update fails for
// any reason.
func NewDbMetric(d *bolt.DB, bucketNames []string, tags ...map[string]string) (*DbMetric, error) {
	return NewDbMetricWithClient(metrics2.DefaultClient, d, bucketNames, tags...)
}

// NewDbMetricWithClient is the same as NewDbMetric, but uses the specified
// metrics2.Client rather than the default client.
func NewDbMetricWithClient(c *metrics2.Client, d *bolt.DB, bucketNames []string, tags ...map[string]string) (*DbMetric, error) {
	m := &DbMetric{
		Liveness:           c.NewLiveness("DbMetric", tags...),
		DbStatsMetric:      newDbStatsMetric(c, tags...),
		BucketStatsMetrics: make(map[string]*BucketStatsMetric, len(bucketNames)),
		db:                 d,
		stop:               make(chan bool),
	}
	for _, name := range bucketNames {
		// TODO(benjaminwagner): Add support for sub-buckets, specified as a
		// path to the sub-bucket from the root.
		m.BucketStatsMetrics[name] = newBucketStatsMetric(c, append(tags, map[string]string{"bucket-path": name})...)
	}
	if err := m.Update(); err != nil {
		return nil, err
	}
	m.Liveness.Reset()
	go func() {
		t := time.Tick(metrics2.DEFAULT_REPORT_FREQUENCY)
		for {
			select {
			case <-m.stop:
				return
			case <-t:
				if err := m.Update(); err != nil {
					glog.Error(err)
				} else {
					m.Liveness.Reset()
				}
			}
		}
	}()
	return m, nil
}

// Update retrieves DB Stats and BucketStats from the DbMetric's BoltDB and
// updates all sub-metrics with new data. Returns an error if the read
// transaction fails or if a bucket is not found.
func (m *DbMetric) Update() error {
	m.DbStatsMetric.Update(m.db.Stats())
	return m.db.View(func(tx *bolt.Tx) error {
		var err error
		for name, metric := range m.BucketStatsMetrics {
			b := tx.Bucket([]byte(name))
			if b == nil {
				err = fmt.Errorf("Bucket %q does not exist.", name)
				continue
			}
			metric.Update(b.Stats())
		}
		return err
	})
}

// Delete stops the update goroutine and deletes all sub-metrics.
func (m *DbMetric) Delete() error {
	m.stop <- true
	if err := m.DbStatsMetric.Delete(); err != nil {
		return err
	}
	for _, metric := range m.BucketStatsMetrics {
		if err := metric.Delete(); err != nil {
			return err
		}
	}
	return nil
}
