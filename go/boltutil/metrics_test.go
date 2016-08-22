package boltutil

import (
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	influx_client "github.com/skia-dev/influxdb/client/v2"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestDbMetric(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestDbMetric")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpdir)
	boltdb, err := bolt.Open(filepath.Join(tmpdir, "bolt.db"), 0600, nil)
	assert.NoError(t, err)
	defer testutils.AssertCloses(t, boltdb)

	assert.NoError(t, boltdb.Update(func(tx *bolt.Tx) error {
		if bucketA, err := tx.CreateBucketIfNotExists([]byte("A")); err != nil {
			return err
		} else {
			if err := bucketA.Put([]byte("Akey1"), []byte("Avalue1")); err != nil {
				return err
			}
			if err := bucketA.Put([]byte("Akey2"), []byte("Avalue2")); err != nil {
				return err
			}
		}
		if bucketB, err := tx.CreateBucketIfNotExists([]byte("B")); err != nil {
			return err
		} else {
			if err := bucketB.Put([]byte("Bkey1"), []byte("Bvalue1")); err != nil {
				return err
			}
		}
		return nil
	}))

	// Perform a read transaction just to ensure TxCount > 0.
	assert.NoError(t, boltdb.View(func(tx *bolt.Tx) error {
		bucketA := tx.Bucket([]byte("A"))
		assert.NotNil(t, bucketA)
		v := bucketA.Get([]byte("Akey1"))
		assert.Equal(t, "Avalue1", string(v))
		return nil
	}))

	appname := "TestDbMetricABC"
	database := "TestDbMetricDEF"

	mtx := sync.Mutex{} // Protects seenMetrics.
	seenMetrics := map[string]bool{}

	checkBatchPoints := func(bp influx_client.BatchPoints) error {
		localSeenMetrics := []string{}
		for _, p := range bp.Points() {
			t.Log(p.String())
			if p.Name() != "db" {
				continue
			}
			tags := p.Tags()
			assert.Equal(t, appname, tags["appname"])
			assert.Equal(t, database, tags["database"])
			metricName := tags["metric"]
			assert.NotEqual(t, "", metricName)
			localSeenMetrics = append(localSeenMetrics, metricName)
			var value int64
			for k, v := range p.Fields() {
				assert.Equal(t, "value", k)
				_, ok := v.(int64)
				assert.True(t, ok)
				value = v.(int64)
			}
			// Assert on a sampling of metrics.
			switch metricName {
			case "TxCount":
				assert.True(t, value > 0)
			case "WriteCount":
				assert.True(t, value > 0)
			case "WriteNs":
				assert.True(t, value > 0)
			case "KeyCount":
				bucket, ok := tags["bucket-path"]
				assert.True(t, ok)
				localSeenMetrics = append(localSeenMetrics, metricName+bucket)
				switch bucket {
				case "A":
					assert.Equal(t, int64(2), value)
				case "B":
					assert.Equal(t, int64(1), value)
				default:
					assert.Fail(t, "Unexpected bucket metric %q", bucket)
				}
			}
		}
		mtx.Lock()
		defer mtx.Unlock()
		for _, m := range localSeenMetrics {
			seenMetrics[m] = true
		}
		return nil
	}

	testInfluxClient := influxdb.NewTestClientWithMockWrite(checkBatchPoints)
	client, err := metrics2.NewClient(testInfluxClient, map[string]string{"appname": appname}, time.Millisecond)
	assert.NoError(t, err)

	m, err := NewDbMetricWithClient(client, boltdb, []string{"A", "B"}, map[string]string{"database": database})
	assert.NoError(t, err)

	assert.NoError(t, client.Flush())

	mtx.Lock()
	assert.True(t, len(seenMetrics) > 0)
	for _, metric := range []string{"TxCount", "WriteCount", "WriteNs", "KeyCountA", "KeyCountB"} {
		assert.True(t, seenMetrics[metric], "Still missing %q", metric)
	}
	mtx.Unlock()

	assert.NoError(t, m.Delete())
}
