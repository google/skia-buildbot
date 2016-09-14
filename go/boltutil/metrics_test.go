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
			if metricName == "KeyCount" {
				bucket, ok := tags["bucket-path"]
				assert.True(t, ok)
				localSeenMetrics = append(localSeenMetrics, metricName+bucket)
				assert.True(t, bucket == "A" || bucket == "B")
			}
			// BoldDB updates Stats asynchronously, so we can't assert on the values of the metrics.
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
