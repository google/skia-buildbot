package dstestutil

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/ds"
	"google.golang.org/api/iterator"
)

type CleanupFunc func()

func cleanup(t *testing.T, kind ds.Kind) {
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
	fmt.Printf("cleanup called: %s\n", ds.Namespace)
}

// InitDatastore is a common utitity function used in tests. It sets up the
// datastore to connect to the emulator and also clears out all instances of
// the given 'kind' from the datastore.
func InitDatastore(t *testing.T, kind ds.Kind) CleanupFunc {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Skip(`Skipping tests that require a local Cloud Datastore emulator.

Run "gcloud beta emulators datastore start --no-store-on-disk"
and set the environment variable DATASTORE_EMULATOR_HOST to run these tests.`)
	}
	err := ds.InitForTesting("test-project", fmt.Sprintf("test-namespace-%d", r.Uint64()))
	assert.NoError(t, err)
	cleanup(t, kind)
	return func() {
		cleanup(t, kind)
	}
}
