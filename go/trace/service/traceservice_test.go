package traceservice

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/trace/db/perftypes"
	"go.skia.org/infra/go/util"
)

const (
	FILENAME = "/tmp/tracestore_test.db"
)

func cleanup() {
	if err := os.Remove(FILENAME); err != nil {
		fmt.Printf("Failed to clean up %s: %s", FILENAME, err)
	}
}
func TestCommitID(t *testing.T) {
	unittest.SmallTest(t)
	// Test that CommitIDs round trip through byte slices.
	now := time.Unix(time.Now().Unix(), 0)
	c := &CommitID{
		Timestamp: now.Unix(),
		Id:        "abc1234",
		Source:    "master",
	}
	b, err := CommitIDToBytes(c)
	if err != nil {
		t.Fatalf("Failed to convert CommitID to []byte: %s", err)
	}
	cp, err := CommitIDFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, c, cp)

	// Handle error conditions such as empty byte slices.
	_, err = CommitIDFromBytes([]byte{})
	assert.Error(t, err)

	b, err = CommitIDToBytes(c)
	if err != nil {
		t.Fatalf("Failed to convert CommitID to []byte: %s", err)
	}
	short1 := b[:8]
	_, err = CommitIDFromBytes(short1)
	assert.Error(t, err, "Input []byte has malformed time.")

	_, err = CommitIDFromBytes([]byte("fred!barney"))
	assert.Error(t, err, "Input []byte value too few seperators.")

	bad_cid := &CommitID{
		Timestamp: now.Unix(),
		Id:        "abc!1234",
		Source:    "master",
	}
	_, err = CommitIDToBytes(bad_cid)
	assert.NotNil(t, err)
}

func TestImpl(t *testing.T) {
	unittest.MediumTest(t)
	ts, err := NewTraceServiceServer(FILENAME)
	assert.NoError(t, err)
	defer util.Close(ts)
	defer cleanup()

	now := time.Unix(100, 0)

	first := now.Unix()
	second := now.Add(time.Minute).Unix()

	commitIDs := []*CommitID{
		{
			Timestamp: first,
			Id:        "abc123",
			Source:    "master",
		},
		{
			Timestamp: second,
			Id:        "xyz789",
			Source:    "master",
		},
	}

	params := &AddParamsRequest{
		Params: []*ParamsPair{
			{
				Key: "key:8888:android",
				Params: map[string]string{
					"config":   "8888",
					"platform": "android",
					"type":     "skp",
				},
			},
			{
				Key: "key:gpu:win8",
				Params: map[string]string{
					"config":   "gpu",
					"platform": "win8",
					"type":     "skp",
				},
			},
		},
	}

	ctx := context.Background()

	// First confirm that Ping() works.
	_, err = ts.Ping(ctx, &Empty{})
	assert.NoError(t, err)

	// Confirm that these traceids don't have Params stored in the db yet.
	missingRequest := &MissingParamsRequest{
		Traceids: []string{"key:8888:android", "key:gpu:win8"},
	}
	missingResp, err := ts.MissingParams(ctx, missingRequest)
	assert.NoError(t, err)
	assert.Equal(t, missingResp.Traceids, missingRequest.Traceids)

	// Now add the Params for them.
	_, err = ts.AddParams(ctx, params)
	assert.NoError(t, err)

	// Confirm the missing list is now empty.
	nowMissing, err := ts.MissingParams(ctx, missingRequest)
	assert.Equal(t, []string{}, nowMissing.Traceids)

	addReq := &AddRequest{
		Commitid: commitIDs[0],
		Values: []*ValuePair{
			{
				Key:   "key:gpu:win8",
				Value: perftypes.BytesFromFloat64(1.234),
			},
			{
				Key:   "key:8888:android",
				Value: perftypes.BytesFromFloat64(0.01),
			},
		},
	}

	// Add a commit.
	_, err = ts.Add(ctx, addReq)
	assert.NoError(t, err)

	// List, GetValues, and GetParams for the added commit.
	listReq := &ListRequest{
		Begin: first,
		End:   second,
	}
	listResp, err := ts.List(ctx, listReq)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(listResp.Commitids))
	assert.Equal(t, "abc123", listResp.Commitids[0].Id)

	valuesReq := &GetValuesRequest{
		Commitid: commitIDs[0],
	}
	valuesResp, err := ts.GetValues(ctx, valuesReq)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(valuesResp.Values))
	expected := map[string]float64{
		"key:gpu:win8":     1.234,
		"key:8888:android": 0.01,
	}
	for _, v := range valuesResp.Values {
		assert.Equal(t, expected[v.Key], math.Float64frombits(binary.LittleEndian.Uint64(v.Value)))
	}
	assert.NotEqual(t, "", valuesResp.Md5)

	paramsReq := &GetParamsRequest{
		Traceids: []string{"key:8888:android", "key:gpu:win8"},
	}
	paramsResp, err := ts.GetParams(ctx, paramsReq)
	assert.NoError(t, err)
	assert.Equal(t, "8888", paramsResp.Params[0].Params["config"])
	assert.Equal(t, "win8", paramsResp.Params[1].Params["platform"])

	// Request the raw data for the commit.
	rawRequest := &GetValuesRequest{
		Commitid: commitIDs[0],
	}
	rawResp, err := ts.GetValuesRaw(ctx, rawRequest)
	assert.NoError(t, err)
	assert.Equal(t, 34, len(rawResp.Value))
	assert.Equal(t, valuesResp.Md5, rawResp.Md5, "Should get the same md5 regardless of how you request the data.")
	// Confirm that we can decode the info on the client side.
	ci, err := NewCommitInfo(rawResp.Value)
	assert.NoError(t, err)

	// The keys are trace64ids, so test that we can convert them to traceids,
	// i.e. from uint64's to strings.
	keys64 := []uint64{}
	for k := range ci.Values {
		keys64 = append(keys64, k)
	}
	assert.Equal(t, 2, len(keys64))
	traceidsRequest := &GetTraceIDsRequest{
		Id: keys64,
	}
	traceids, err := ts.GetTraceIDs(ctx, traceidsRequest)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(traceids.Ids))
	assert.True(t, util.In(traceids.Ids[0].Id, paramsReq.Traceids))
	assert.True(t, util.In(traceids.Ids[1].Id, paramsReq.Traceids))
}

func TestAtomize(t *testing.T) {
	unittest.MediumTest(t)
	ts, err := NewTraceServiceServer(FILENAME)
	assert.NoError(t, err)
	defer util.Close(ts)
	defer cleanup()

	ids, err := ts.atomize([]string{"foo"})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), ids["foo"])

	ids, err = ts.atomize([]string{"foo"})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), ids["foo"])

	ids, err = ts.atomize([]string{"bar"})
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), ids["bar"])

	ids, err = ts.atomize([]string{"foo"})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), ids["foo"])

}

func TestCommitInfo(t *testing.T) {
	unittest.SmallTest(t)
	// Test roundtripping through []byte.
	c := &CommitInfo{
		Values: map[uint64][]byte{
			uint64(1):          []byte("foo"),
			uint64(3):          []byte(""),
			uint64(0xffffffff): []byte("last"),
		},
	}
	cp, err := NewCommitInfo(c.ToBytes())
	if err != nil {
		t.Fatalf("Failed NewCommitInfo: %s", err)
	}
	assert.Equal(t, 3, len(cp.Values))
	assert.Equal(t, "foo", string(cp.Values[uint64(1)]))
	assert.Equal(t, c, cp)

	// Test error handling.
	cnil, err := NewCommitInfo(nil)
	if err != nil {
		t.Fatalf("Failed NewCommitInfo: %s", err)
	}
	assert.Equal(t, 0, len(cnil.Values))

	b := c.ToBytes()

	// Test inputs that should fail.
	_, err = NewCommitInfo(b[1:])
	assert.Error(t, err)

	_, err = NewCommitInfo(b[:len(b)-1])
	assert.Error(t, err)
}
