package traceservice

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/net/context"
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
	ts, err := NewTraceServiceServer(FILENAME)
	assert.NoError(t, err)
	defer util.Close(ts)
	defer cleanup()

	now := time.Now()

	first := now.Unix()
	second := now.Add(time.Minute).Unix()

	commitIDs := []*CommitID{
		&CommitID{
			Timestamp: first,
			Id:        "abc123",
			Source:    "master",
		},
		&CommitID{
			Timestamp: second,
			Id:        "xyz789",
			Source:    "master",
		},
	}

	params := &AddParamsRequest{
		Params: map[string]*Params{
			"key:8888:android": &Params{
				Params: map[string]string{
					"config":   "8888",
					"platform": "android",
					"type":     "skp",
				},
			},
			"key:gpu:win8": &Params{
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
	assert.Nil(t, err)

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
		Entries: map[string][]byte{
			"key:gpu:win8":     types.BytesFromFloat64(1.234),
			"key:8888:android": types.BytesFromFloat64(0.01),
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
	assert.Equal(t, 0.01, math.Float64frombits(binary.LittleEndian.Uint64(valuesResp.Values["key:8888:android"])))
	assert.Equal(t, 1.234, math.Float64frombits(binary.LittleEndian.Uint64(valuesResp.Values["key:gpu:win8"])))

	paramsReq := &GetParamsRequest{
		Traceids: []string{"key:8888:android", "key:gpu:win8"},
	}
	paramsResp, err := ts.GetParams(ctx, paramsReq)
	assert.NoError(t, err)
	assert.Equal(t, "8888", paramsResp.Params["key:8888:android"].Params["config"])
	assert.Equal(t, "win8", paramsResp.Params["key:gpu:win8"].Params["platform"])

	// Remove the commit.
	removeRequest := &RemoveRequest{
		Commitid: commitIDs[0],
	}
	_, err = ts.Remove(ctx, removeRequest)
	assert.NoError(t, err)

	listResp, err = ts.List(ctx, listReq)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(listResp.Commitids))
}

func TestAtomize(t *testing.T) {
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
	// Test roundtripping through []byte.
	c := &commitinfo{
		Values: map[uint64][]byte{
			uint64(1):          []byte("foo"),
			uint64(3):          []byte(""),
			uint64(0xffffffff): []byte("last"),
		},
	}
	cp, err := newCommitInfo(c.ToBytes())
	if err != nil {
		t.Fatalf("Failed newCommitInfo: %s", err)
	}
	assert.Equal(t, 3, len(cp.Values))
	assert.Equal(t, "foo", string(cp.Values[uint64(1)]))
	assert.Equal(t, c, cp)

	// Test error handling.
	cnil, err := newCommitInfo(nil)
	if err != nil {
		t.Fatalf("Failed newCommitInfo: %s", err)
	}
	assert.Equal(t, 0, len(cnil.Values))

	b := c.ToBytes()

	// Test inputs that should fail.
	_, err = newCommitInfo(b[1:])
	assert.Error(t, err)

	_, err = newCommitInfo(b[:len(b)-1])
	assert.Error(t, err)
}
