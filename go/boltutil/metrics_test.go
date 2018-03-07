package boltutil

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestDbMetric(t *testing.T) {
	testutils.MediumTest(t)

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

	database := "TestDbMetricDEF"

	client := metrics2.GetDefaultClient()
	m, err := NewDbMetricWithClient(client, boltdb, []string{"A", "B"}, map[string]string{"database": database})
	assert.NoError(t, err)

	assert.NoError(t, m.Update())

	assert.NotZero(t, client.GetInt64Metric("bolt_tx", map[string]string{"metric": "WriteCount", "database": database}).Get())
	assert.NotZero(t, client.GetInt64Metric("bolt_tx", map[string]string{"metric": "WriteNs", "database": database}).Get())
	assert.Equal(t, int64(2), client.GetInt64Metric("bolt_bucket", map[string]string{"metric": "KeyCount", "database": database, "bucket_path": "A"}).Get())
	assert.Equal(t, int64(1), client.GetInt64Metric("bolt_bucket", map[string]string{"metric": "KeyCount", "database": database, "bucket_path": "B"}).Get())

	assert.NoError(t, m.Delete())
}
