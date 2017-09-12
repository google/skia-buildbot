package regression

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
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
	q := ds.NewQuery(ds.REGRESSION).KeysOnly()
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

// TestSetLowWithMissingDS test storing a low cluster to the datastore.
func TestDS(t *testing.T) {
	testutils.MediumTest(t)
	initDatastore(t)
	defer Init(false)

	st := NewStore()

	r := New()
	c := &cid.CommitDetail{
		CommitID: cid.CommitID{
			Source: "master",
			Offset: 1,
		},
		Timestamp: 1479235651,
	}

	// Test Regressions.
	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r.SetLow("foo", df, cl)
	_, ok := r.ByAlertID["foo"]
	assert.True(t, ok)
	assert.False(t, r.Triaged())

	// Test store.

	// Create a new regression.
	isNew, err := st.SetLow(c, "foo", df, cl)
	assert.True(t, isNew)
	assert.NoError(t, err)

	// Overwrite a regression.
	isNew, err = st.SetLow(c, "foo", df, cl)
	assert.False(t, isNew)
	assert.NoError(t, err)

	// Confirm new regression is present.
	ranges, err := st.Range(0, 0, UNTRIAGED_SUBSET)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	count, err := st.Untriaged()
	assert.Equal(t, count, 1)

	// Triage existing regression.
	tr := TriageStatus{
		Status:  POSITIVE,
		Message: "bad",
	}
	err = st.TriageLow(c, "foo", tr)
	assert.NoError(t, err)

	// Confirm regression is triaged.
	ranges, err = st.Range(0, 0, UNTRIAGED_SUBSET)
	assert.NoError(t, err)
	assert.Len(t, ranges, 0)

	count, err = st.Untriaged()
	assert.Equal(t, count, 0)

	now := time.Unix(c.Timestamp, 0)
	begin := now.Add(-time.Hour).Unix()
	end := now.Add(time.Hour).Unix()

	ranges, err = st.Range(begin, end, ALL_SUBSET)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	// Try triaging a regression that doesn't exist.
	err = st.TriageHigh(c, "bar", tr)
	assert.Error(t, err)

	ranges, err = st.Range(begin, end, ALL_SUBSET)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	count, err = st.Untriaged()
	assert.Equal(t, count, 0)
}
