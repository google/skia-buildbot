package alerts

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/ds"
	"google.golang.org/api/iterator"
)

func initDatastore(t *testing.T) {
	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Skip(`Skipping tests that require a local Cloud Datastore emulaor.

Run "gcloud beta emulators datastore start --no-store-on-disk"
and set the environment variable DATASTORE_EMULATOR_HOST to run these tests.`)
	}
	ds.InitForTesting("test-project", "test-namespace")
	Init(true)
	q := ds.NewQuery(ds.ALERT).KeysOnly()
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

func TestDS(t *testing.T) {
	testutils.MediumTest(t)
	initDatastore(t)
	defer Init(false)

	a := NewStore()
	cfg := NewConfig()
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	err := a.Save(cfg)
	assert.NoError(t, err)

	cfgs, err := a.List(false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)

	err = a.Delete(int(cfgs[0].ID))
	assert.NoError(t, err)

	cfgs, err = a.List(true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)

	cfgs, err = a.List(false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 0)

	cfg = NewConfig()
	cfg.Query = "source_type=skp"
	cfg.DisplayName = "foo"
	a.Save(cfg)

	cfgs, err = a.List(true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 2)
	assert.Equal(t, "bar", cfgs[0].DisplayName)
	assert.Equal(t, "foo", cfgs[1].DisplayName)
}
