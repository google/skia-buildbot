package dstestutil

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/ds"
	"google.golang.org/api/iterator"
)

// InitDatastore is a common utitity function used in tests. It sets up the
// datastore to connect to the emulator and also clears out all instances of
// the given 'kind' from the datastore.
func InitDatastore(t *testing.T, kind ds.Kind) {
	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Skip(`Skipping tests that require a local Cloud Datastore emulaor.

Run "gcloud beta emulators datastore start --no-store-on-disk"
and set the environment variable DATASTORE_EMULATOR_HOST to run these tests.`)
	}
	ds.InitForTesting("test-project", "test-namespace")
	q := ds.NewQuery(kind).KeysOnly()
	it := ds.DS.Run(context.TODO(), q)
	for {
		k, err := it.Next(nil)
		if err == iterator.Done {
			break
		} else if err != nil {
			t.Fatalf("Failed to clean database: %s", err)
		}
		err = ds.DS.Delete(context.Background(), k)
		assert.NoError(t, err)
	}
}
