package ds_expstore

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

var testKinds = []ds.Kind{
	ds.MASTER_EXP_CHANGE,
	ds.TRYJOB_EXP_CHANGE,
	ds.TRYJOB_TEST_DIGEST_EXP,
	ds.HELPER_RECENT_KEYS,
	ds.EXPECTATIONS_BLOB_ROOT,
	ds.EXPECTATIONS_BLOB,
}

func TestMasterCloudExpectationsStore(t *testing.T) {
	unittest.LargeTest(t)

	cleanup := initDS(t)
	defer cleanup()

	// Test the DS backed store for master.
	masterEventBus := eventbus.New()
	cloudStore, _, err := New(ds.DS, masterEventBus)
	assert.NoError(t, err)
	testExpectationStore(t, cloudStore, masterEventBus, 0, expstorage.EV_EXPSTORAGE_CHANGED)
	testCloudExpstoreClear(t, cloudStore)
}

func testCloudExpstoreClear(t *testing.T, cloudStore expstorage.ExpectationsStore) {
	// Make sure the clear works.
	assert.NoError(t, cloudStore.Clear())
	assert.NoError(t, testutils.EventuallyConsistent(5*time.Second, func() error {
		for _, kind := range testKinds {
			count, err := ds.DS.Count(context.TODO(), ds.NewQuery(kind).KeysOnly())
			assert.NoError(t, err)
			if count > 0 {
				return testutils.TryAgainErr
			}
		}
		return nil
	}))
}

func TestIssueCloudExpectationsStore(t *testing.T) {
	unittest.LargeTest(t)

	cleanup := initDS(t)
	defer cleanup()

	// Test the expectation store for an individual issue.
	masterEventBus := eventbus.New()
	_, issueStoreFactory, err := New(ds.DS, masterEventBus)
	assert.NoError(t, err)
	issueID := int64(1234567)
	issueStore := issueStoreFactory(issueID)
	testExpectationStore(t, issueStore, masterEventBus, issueID, expstorage.EV_TRYJOB_EXP_CHANGED)
	testCloudExpstoreClear(t, issueStore)
}

// initDS initializes the datastore for testing.
func initDS(t *testing.T, kinds ...ds.Kind) func() {
	initKinds := []ds.Kind{}
	initKinds = append(initKinds, testKinds...)
	initKinds = append(initKinds, kinds...)
	return ds_testutil.InitDatastore(t, initKinds...)
}

const hexLetters = "0123456789abcdef"
const md5Length = 32

func randomDigest() types.Digest {
	ret := make([]byte, md5Length)
	for i := 0; i < md5Length; i++ {
		ret[i] = hexLetters[rand.Intn(len(hexLetters))]
	}
	return types.Digest(ret)
}

func getRandomChange(nTests, nDigests int) types.Expectations {
	labels := []types.Label{types.POSITIVE, types.NEGATIVE, types.UNTRIAGED}
	ret := make(types.Expectations, nTests)
	for i := 0; i < nTests; i++ {
		digests := make(map[types.Digest]types.Label, nDigests)
		for j := 0; j < nDigests; j++ {
			digests[randomDigest()] = labels[rand.Intn(len(labels))]
		}
		ret[types.TestName(util.RandomName())] = digests
	}
	return ret
}

// Test against the expectation store interface.
func testExpectationStore(t *testing.T, store expstorage.ExpectationsStore, eventBus eventbus.EventBus, issueID int64, eventType string) {
	// Get the initial log size. This is necessary because we
	// call this function multiple times with the same underlying
	// ExpectationStore.
	initialLogRecs, initialLogTotal, err := store.QueryLog(0, 100, true)
	assert.NoError(t, err)
	initialLogRecsLen := len(initialLogRecs)

	// Request expectations and make sure they are empty.
	emptyExp, err := store.Get()
	assert.NoError(t, err)
	assert.Empty(t, emptyExp)

	// If we have an event bus then keep gathering events.
	callbackCh := make(chan types.TestNameSlice, 3)
	if eventBus != nil {
		eventBus.SubscribeAsync(eventType, func(e interface{}) {
			evData := e.(*expstorage.EventExpectationChange)
			if (issueID > 0) && (evData.IssueID != issueID) {
				return
			}

			testNames := make(types.TestNameSlice, 0, len(evData.TestChanges))
			for testName := range evData.TestChanges {
				testNames = append(testNames, testName)
			}
			sort.Sort(testNames)
			callbackCh <- testNames
		})
	}

	TEST_1, TEST_2 := types.TestName("test1"), types.TestName("test2")

	// digests
	DIGEST_11, DIGEST_12 := types.Digest("d11"), types.Digest("d12")
	DIGEST_21, DIGEST_22 := types.Digest("d21"), types.Digest("d22")

	expChange_1 := types.Expectations{
		TEST_1: {
			DIGEST_11: types.POSITIVE,
			DIGEST_12: types.NEGATIVE,
		},
		TEST_2: {
			DIGEST_21: types.POSITIVE,
			DIGEST_22: types.NEGATIVE,
		},
	}
	logEntry_1 := []*expstorage.TriageDetail{
		{TestName: TEST_1, Digest: DIGEST_11, Label: "positive"},
		{TestName: TEST_1, Digest: DIGEST_12, Label: "negative"},
		{TestName: TEST_2, Digest: DIGEST_21, Label: "positive"},
		{TestName: TEST_2, Digest: DIGEST_22, Label: "negative"},
	}

	assert.NoError(t, store.AddChange(expChange_1, "user-0"))
	if eventBus != nil {
		found := waitForChanLen(t, callbackCh, 1)
		assert.Equal(t, types.TestNameSlice{TEST_1, TEST_2}, found[0])
	}

	// TODO(kjlubick): assert something with foundExps
	foundExps, err := store.Get()
	assert.NoError(t, err)

	assert.Equal(t, expChange_1, foundExps)
	checkLogEntry(t, store, expChange_1)

	// Update digests.
	expChange_2 := types.Expectations{
		TEST_1: {
			DIGEST_11: types.NEGATIVE,
		},
		TEST_2: {
			DIGEST_22: types.UNTRIAGED,
		},
	}
	logEntry_2 := []*expstorage.TriageDetail{
		{TestName: TEST_1, Digest: DIGEST_11, Label: "negative"},
		{TestName: TEST_2, Digest: DIGEST_22, Label: "untriaged"},
	}

	assert.NoError(t, store.AddChange(expChange_2, "user-1"))
	if eventBus != nil {
		found := waitForChanLen(t, callbackCh, 1)
		assert.Equal(t, types.TestNameSlice{TEST_1, TEST_2}, found[0])
	}

	foundTestExp, err := store.Get()
	assert.NoError(t, err)
	assert.Equal(t, types.NEGATIVE, foundTestExp[TEST_1][DIGEST_11])
	assert.Equal(t, types.UNTRIAGED, foundTestExp[TEST_2][DIGEST_22])
	checkLogEntry(t, store, expChange_2)

	// Send empty changes to test the event bus.
	emptyChanges := types.Expectations{}
	assert.NoError(t, store.AddChange(emptyChanges, "user-2"))
	if eventBus != nil {
		found := waitForChanLen(t, callbackCh, 1)
		assert.Empty(t, found[0])
	}
	checkLogEntry(t, store, emptyChanges)

	foundExps, err = store.Get()
	assert.NoError(t, err)

	// Make sure we added the correct number of triage log entries.
	addedRecs := 3
	logEntries, total, err := store.QueryLog(0, 5, true)
	assert.NoError(t, err)
	assert.Equal(t, addedRecs+initialLogTotal, total)
	assert.Equal(t, util.MinInt(addedRecs+initialLogRecsLen, 5), len(logEntries))
	lastRec := logEntries[0]
	secondToLastRec := logEntries[1]

	assert.Equal(t, 0, len(logEntries[0].Details))
	assert.Equal(t, logEntry_2, logEntries[1].Details)
	assert.Equal(t, logEntry_1, logEntries[2].Details)

	logEntries, total, err = store.QueryLog(100, 5, true)
	assert.NoError(t, err)
	assert.Equal(t, addedRecs+initialLogTotal, total)
	assert.Equal(t, 0, len(logEntries))

	// Undo the latest version and make sure the corresponding record is correct.
	changes, err := store.UndoChange(parseID(t, lastRec.ID), "user-1")
	assert.NoError(t, err)
	checkLogEntry(t, store, changes)

	changes, err = store.UndoChange(parseID(t, secondToLastRec.ID), "user-1")
	assert.NoError(t, err)
	checkLogEntry(t, store, changes)

	addedRecs += 2
	logEntries, total, err = store.QueryLog(0, 2, true)
	assert.NoError(t, err)
	assert.Equal(t, addedRecs+initialLogTotal, total)
	assert.Equal(t, 0, len(logEntries[1].Details))
	assert.Equal(t, 2, len(logEntries[0].Details))

	foundTestExp, err = store.Get()
	assert.NoError(t, err)

	for testName, digests := range expChange_2 {
		for d := range digests {
			_, ok := foundTestExp[testName][d]
			assert.True(t, ok)
			assert.Equal(t, expChange_1[testName][d].String(), foundTestExp[testName][d].String())
		}
	}

	// Make sure undoing the previous undo causes an error.
	logEntries, _, err = store.QueryLog(0, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(logEntries))
	_, err = store.UndoChange(parseID(t, logEntries[0].ID), "user-1")
	assert.NotNil(t, err)
}

// waitForChan removes 'targetLen' elements from the channel and returns them.
// If the given number of items are not returned within one second the test fails.
func waitForChanLen(t *testing.T, ch chan types.TestNameSlice, targetLen int) []types.TestNameSlice {
	ret := make([]types.TestNameSlice, 0, targetLen)
	assert.NoError(t, testutils.EventuallyConsistent(time.Second, func() error {
		select {
		case ele := <-ch:
			ret = append(ret, ele)
		default:
			break
		}

		if len(ret) != targetLen {
			return testutils.TryAgainErr
		}
		return nil
	}))
	return ret
}

func parseID(t *testing.T, idStr string) int64 {
	ret, err := strconv.ParseInt(idStr, 10, 64)
	assert.NoError(t, err)
	return ret
}

func checkLogEntry(t *testing.T, store expstorage.ExpectationsStore, changes types.Expectations) {
	logEntries, _, err := store.QueryLog(0, 1, true)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(logEntries))

	counter := 0
	for _, digests := range changes {
		counter += len(digests)
	}
	assert.Equal(t, counter, len(logEntries[0].Details))
	for _, d := range logEntries[0].Details {
		_, ok := changes[d.TestName][d.Digest]
		assert.True(t, ok)
		assert.Equal(t, changes[d.TestName][d.Digest].String(), d.Label)
	}
}
