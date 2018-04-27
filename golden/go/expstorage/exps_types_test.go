package expstorage

import (
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

func TestNameDigestLabels(t *testing.T) {
	nowMs := util.TimeStamp(time.Millisecond)

	k1 := newKey(nowMs - 100)
	k2 := newKey(nowMs - 80)
	k3 := newKey(nowMs - 60)
	k4 := newKey(nowMs - 40)

	matView := emptyMatView()
	matView.Update(k3)
	matView.Update(k2)
	matView.Update(k4)
	matView.Update(k1)

	expMatView := &MatView{
		RecentChanges: []*datastore.Key{k4, k3, k2, k1},
	}
	assert.Equal(t, expMatView, matView)

	evConsistentDeltaMs = 1000
	time.Sleep(1100 * time.Millisecond)
	k5 := newKey(util.TimeStamp(time.Millisecond))
	matView.Update(k5)
	expMatView.RecentChanges = []*datastore.Key{k5}
	assert.Equal(t, expMatView, matView)

	TEST_1, TEST_2 := "test1", "test2"
	DIGEST_11, DIGEST_12 := "d11", "d12"
	DIGEST_21, DIGEST_22 := "d21", "d22"

	expChange_1 := map[string]types.TestClassification{
		TEST_1: {
			DIGEST_11: types.POSITIVE,
			DIGEST_12: types.NEGATIVE,
		},
		TEST_2: {
			DIGEST_21: types.POSITIVE,
			DIGEST_22: types.NEGATIVE,
		},
	}

	_, expColl := buildExpCollection(expChange_1, "", nil)
	assert.Equal(t, expChange_1, expColl.toExpectations(false).Tests)

	var emptyColl ExpCollection
	emptyColl.update(expChange_1)
	assert.Equal(t, expChange_1, emptyColl.toExpectations(false).Tests)
}

func TestTimeBasedKeyID(t *testing.T) {
	ts := util.TimeStamp(time.Millisecond)
	keyID := getSortableTimeID(ts)
	assert.Equal(t, ts, getTimeFromID(keyID))
}

func newKey(timeStampMs int64) *datastore.Key {
	k := ds.NewKey(ds.MASTER_EXP_CHANGE)
	k.ID = getSortableTimeID(timeStampMs)
	return k
}
