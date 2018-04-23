package expstorage

import (
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

func TestNameDigestLabels(t *testing.T) {
	nowMs := util.TimeStamp(time.Millisecond)

	k1, ts1 := newKey("k1"), nowMs-100
	k2, ts2 := newKey("k2"), nowMs-80
	k3, ts3 := newKey("k3"), nowMs-60
	k4, ts4 := newKey("k4"), nowMs-40

	matView := emptyMatView()
	matView.Update(k3, ts3)
	matView.Update(k2, ts2)
	matView.Update(k4, ts4)
	matView.Update(k1, ts1)

	expMatView := &MatView{
		RecentChanges: []*datastore.Key{k1, k2, k3, k4},
		TimeStamps:    []int64{ts1, ts2, ts3, ts4},
	}
	assert.Equal(t, expMatView, matView)

	evConsistentDeltaMs = 1000
	time.Sleep(1100 * time.Millisecond)
	k5, ts5 := newKey("k5"), util.TimeStamp(time.Millisecond)
	matView.Update(k5, ts5)
	expMatView.RecentChanges = []*datastore.Key{k5}
	expMatView.TimeStamps = []int64{ts5}
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
	fmt.Printf("expCol: %s", spew.Sdump(expColl))
	assert.Equal(t, expChange_1, expColl.toExpectations().Tests)

	var emptyColl ExpCollection
	emptyColl.update(expChange_1)
	assert.Equal(t, expChange_1, emptyColl.toExpectations().Tests)
}

func newKey(name string) *datastore.Key {
	k := ds.NewKey(ds.MASTER_EXP_CHANGE)
	k.Name = name
	return k
}
