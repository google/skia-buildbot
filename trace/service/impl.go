package traceservice

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_out=plugins=grpc:. traceservice.proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/groupcache/lru"
	"github.com/golang/protobuf/proto"
	"github.com/skia-dev/glog"
	"golang.org/x/net/context"
)

const (
	COMMIT_BUCKET_NAME  = "commits"
	TRACE_BUCKET_NAME   = "traces"
	TRACEID_BUCKET_NAME = "traceids"
	LARGEST_TRACEID_KEY = "the largest trace64id"

	// How many items to keep in the in-memory LRU cache.
	MAX_INT64_ID_CACHED = 1024 * 1024
)

// bytesFromUint64 converts a uint64 to a []byte.
func bytesFromUint64(u uint64) []byte {
	ret := make([]byte, 8, 8)
	binary.LittleEndian.PutUint64(ret, u)
	return ret
}

// CommitIDToByes serializes the CommitID to a []byte in the same format that CommitIDFromBytes reads.
//
// The []byte is constructed so that serialized CommitIDs are comparable via bytes.Compare
// with earlier commits coming before later commits.
func CommitIDToBytes(c *CommitID) ([]byte, error) {
	if strings.Contains(c.Id, "!") || strings.Contains(c.Source, "!") {
		return nil, fmt.Errorf("Invalid CommitID: Must not contain '!': %#v", *c)
	}
	return []byte(fmt.Sprintf("%s!%s!%s", time.Unix(c.Timestamp, 0).Format(time.RFC3339), c.Id, c.Source)), nil
}

// CommitIDFromBytes creates a CommitID from a []byte, usually produced from a
// previously serialized CommitID. See ToBytes.
func CommitIDFromBytes(b []byte) (*CommitID, error) {
	s := string(b)
	parts := strings.SplitN(s, "!", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("Invalid CommitID format %s", s)
	}
	t, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return nil, fmt.Errorf("Invalid CommitID time format %s: %s", s, err)
	}
	return &CommitID{
		Timestamp: t.Unix(),
		Id:        parts[1],
		Source:    parts[2],
	}, nil
}

// TraceServiceImpl implements TraceServiceServer.
type TraceServiceImpl struct {
	// db is the BoltDB datastore we actually store the data in.
	db *bolt.DB

	// cache is an in-memory LRU cache for traceids and trace64ids.
	cache *lru.Cache

	// mutex controls access to cache.
	mutex sync.Mutex
}

// NewTraceServiceServer creates a new DB that stores the data in BoltDB format at
// the given filename location.
func NewTraceServiceServer(filename string) (*TraceServiceImpl, error) {
	d, err := bolt.Open(filename, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("Failed to open BoltDB at %s: %s", filename, err)
	}
	createBuckets := func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(COMMIT_BUCKET_NAME))
		if err != nil {
			return fmt.Errorf("Failed to create bucket %s: %s", COMMIT_BUCKET_NAME, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(TRACE_BUCKET_NAME))
		if err != nil {
			return fmt.Errorf("Failed to create bucket %s: %s", TRACE_BUCKET_NAME, err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(TRACEID_BUCKET_NAME))
		if err != nil {
			return fmt.Errorf("Failed to create bucket %s: %s", TRACEID_BUCKET_NAME, err)
		}
		return nil
	}
	if err := d.Update(createBuckets); err != nil {
		return nil, fmt.Errorf("Failed to create buckets: %s", err)
	}
	return &TraceServiceImpl{
		db:    d,
		cache: lru.New(MAX_INT64_ID_CACHED),
	}, nil
}

// atomize looks up the 64bit id for the given strings.
//
// If not found create a new mapping uint64->id and store in the datastore.
// Also stores the reverse mapping in the same bucket.
func (ts *TraceServiceImpl) atomize(ids []string) (map[string]uint64, error) {
	ret := map[string]uint64{}

	// First look up everything in the LRU cache.
	notcached := []string{}
	for _, id := range ids {
		ts.mutex.Lock()
		cached, ok := ts.cache.Get(id)
		ts.mutex.Unlock()
		if ok {
			ret[id] = cached.(uint64)
		} else {
			notcached = append(notcached, id)
		}
	}

	// If we've found all our answers in the LRU cache then we're done.
	if len(ret) == len(ids) {
		return ret, nil
	}

	// Next look into the datastore and see if we can find the answers there.
	notstored := []string{}
	get := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACEID_BUCKET_NAME))
		for _, id := range notcached {
			// Find the value in the datastore.
			if bid64 := t.Get([]byte(id)); bid64 == nil {
				notstored = append(notstored, id)
			} else {
				// If found update the return value and the cache.
				id64 := binary.LittleEndian.Uint64(bid64)
				ts.mutex.Lock()
				ts.cache.Add(id, id64)
				ts.cache.Add(id64, id)
				ts.mutex.Unlock()
				ret[id] = id64
			}
		}
		return nil
	}

	if err := ts.db.View(get); err != nil {
		return nil, fmt.Errorf("Error while reading trace ids: %s", err)
	}

	if len(ret) == len(ids) {
		return ret, nil
	}

	// If we still have ids that we haven't matching trace64ids for then we need
	// to create trace64ids for them and store them in BoltDB and in the LRU
	// cache.
	add := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACEID_BUCKET_NAME))
		// Find the current largest trace64id.
		var largest uint64 = 0
		if blargest := t.Get([]byte(LARGEST_TRACEID_KEY)); blargest != nil {
			largest = binary.LittleEndian.Uint64(blargest)
		}

		// Generate a new id for each traceid and store the results.
		for i, id := range notstored {
			value := largest + uint64(i) + 1
			bvalue := make([]byte, 8, 8)
			binary.LittleEndian.PutUint64(bvalue, value)
			if err := t.Put([]byte(id), bvalue); err != nil {
				return fmt.Errorf("Failed to write atomized value for %s: %s", id, err)
			}
			if err := t.Put(bvalue, []byte(id)); err != nil {
				return fmt.Errorf("Failed to write atomized reverse lookup value for %s: %s", id, err)
			}
			ts.mutex.Lock()
			ts.cache.Add(id, value)
			ts.cache.Add(value, id)
			ts.mutex.Unlock()
			ret[id] = value
		}

		largest = largest + uint64(len(notstored))

		// Write the new value for LARGEST_TRACEID_KEY.
		blargest := make([]byte, 8, 8)
		binary.LittleEndian.PutUint64(blargest, largest)
		if err := t.Put([]byte(LARGEST_TRACEID_KEY), blargest); err != nil {
			return fmt.Errorf("Failed to write an updated largest trace64id value: %s", err)
		}

		return nil
	}

	if err := ts.db.Update(add); err != nil {
		return nil, fmt.Errorf("Error while writing new trace ids: %s", err)
	}

	if len(ret) == len(ids) {
		return ret, nil
	} else {
		return nil, fmt.Errorf("Failed to add traceid ids: mismatched number of ids.")
	}
}

// commitinfo is the value stored in the commit bucket.
type commitinfo struct {
	Values map[uint64][]byte
}

// newCommitInfo returns a commitinfo with data deserialized from the byte slice.
func newCommitInfo(volatile []byte) (*commitinfo, error) {
	ret := &commitinfo{
		Values: map[uint64][]byte{},
	}
	if len(volatile) == 0 {
		return ret, nil
	}

	b := make([]byte, len(volatile), len(volatile))
	n := copy(b, volatile)
	if n != len(volatile) {
		return nil, fmt.Errorf("Failed to copy all the bytes.")
	}
	// The byte slice is structured as a repeating set of:
	//
	// [uint64 (8 bytes)][length of the value (1 byte)][value (0-256 bytes, as determined by previous byte)]
	//
	for len(b) > 0 {
		if len(b) < 9 {
			return nil, fmt.Errorf("Failed to decode, not enough bytes left: %#v", b)
		}
		key := binary.LittleEndian.Uint64(b[0:8])
		length := b[8]
		b = b[9:]
		if len(b) < int(length) {
			return nil, fmt.Errorf("Failed to decode, not enough bytes left for %d: Want %d Got %d", key, length, len(b))
		}
		ret.Values[key] = b[0:length]
		b = b[length:]
	}
	return ret, nil
}

// ToBytes serializes the data in the commitinfo into a byte slice. The format is ingestable by FromBytes.
//
// The byte slice is structured as a repeating set of three things serialized as bytes.
//
//   1. uint64
//   2. length of the value
//   3. the actual value.
//
// So in the byte slice this would look like:
//
//   [uint64 (8 bytes)][length of the value (1 byte)][value (0-255 bytes, as determined by previous byte)]
//
func (c *commitinfo) ToBytes() []byte {
	size := 0
	for _, v := range c.Values {
		size += 9 + len(v)
	}
	buf := make([]byte, 0, size)
	for k, v := range c.Values {
		buf = append(buf, bytesFromUint64(k)...)
		buf = append(buf, byte(len(v)))
		buf = append(buf, v...)
	}
	return buf
}

func (ts *TraceServiceImpl) MissingParams(ctx context.Context, in *MissingParamsRequest) (*MissingParamsResponse, error) {
	resp := &MissingParamsResponse{
		Traceids: []string{},
	}

	// Populate the response with traceids we can't find in the bucket.
	get := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACE_BUCKET_NAME))
		for _, traceid := range in.Traceids {
			if b := t.Get([]byte(traceid)); b == nil {
				resp.Traceids = append(resp.Traceids, traceid)
			}
		}
		return nil
	}
	if err := ts.db.View(get); err != nil {
		return nil, fmt.Errorf("Failed to add values to tracedb: %s", err)
	}
	return resp, nil
}

func (ts *TraceServiceImpl) AddParams(ctx context.Context, in *AddParamsRequest) (*EmptyResponse, error) {
	// Serialize the Params for each trace as a proto and collect the traceids.
	// We do this outside the add func so there's less work taking place in the
	// Update transaction.
	params := map[string][]byte{}
	var err error
	for key, value := range in.Params {
		ti := &StoredEntry{
			Params: value,
		}
		params[key], err = proto.Marshal(ti)
		if err != nil {
			return nil, fmt.Errorf("Failed to serialize the Params: %s", err)
		}
	}

	// Add the Params for each traceid to the bucket.
	add := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACE_BUCKET_NAME))
		for traceid, _ := range in.Params {
			if err := t.Put([]byte(traceid), params[traceid]); err != nil {
				return fmt.Errorf("Failed to write the trace info for %s: %s", traceid, err)
			}
		}
		return nil
	}
	if err := ts.db.Update(add); err != nil {
		return nil, fmt.Errorf("Failed to add values to tracedb: %s", err)
	}
	return &EmptyResponse{}, nil
}

func (ts *TraceServiceImpl) Add(ctx context.Context, in *AddRequest) (*EmptyResponse, error) {
	glog.Info("Add() begin.")
	if in == nil {
		return nil, fmt.Errorf("Received nil request.")
	}
	if in.Commitid == nil {
		return nil, fmt.Errorf("Received nil CommitID")
	}
	if in.Entries == nil {
		return nil, fmt.Errorf("Received nil Entries")
	}

	// Get the trace64ids for each traceid.
	keys := []string{}
	for key, _ := range in.Entries {
		keys = append(keys, key)
	}

	trace64ids, err := ts.atomize(keys)
	glog.Infof("atomized %d keys", len(trace64ids))
	if err != nil {
		return nil, fmt.Errorf("Failed to create short trace ids: %s", err)
	}

	add := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME))

		// Write the commitinfo.
		key, err := CommitIDToBytes(in.Commitid)
		if err != nil {
			return err
		}

		// First load the existing info.
		data, err := newCommitInfo(c.Get(key))
		if err != nil {
			return fmt.Errorf("Unable to decode stored values: %s", err)
		}

		// Add our new data points.
		for key, entry := range in.Entries {
			data.Values[trace64ids[key]] = entry
		}

		// Write to the datastore.
		if err := c.Put(key, data.ToBytes()); err != nil {
			return fmt.Errorf("Failed to write the trace info for %s: %s", key, err)
		}
		return nil
	}

	if err := ts.db.Update(add); err != nil {
		return nil, fmt.Errorf("Failed to add values to tracedb: %s", err)
	}
	return &EmptyResponse{}, nil
}

func (ts *TraceServiceImpl) Remove(ctx context.Context, in *RemoveRequest) (*EmptyResponse, error) {
	if in == nil {
		return nil, fmt.Errorf("Received nil request.")
	}
	remove := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME))
		key, err := CommitIDToBytes(in.Commitid)
		if err != nil {
			return err
		}
		return c.Delete(key)
	}
	if err := ts.db.Update(remove); err != nil {
		return nil, fmt.Errorf("Failed to remove values from tracedb: %s", err)
	}
	ret := &EmptyResponse{}
	return ret, nil
}

func (ts *TraceServiceImpl) List(ctx context.Context, listRequest *ListRequest) (*ListResponse, error) {
	if listRequest == nil {
		return nil, fmt.Errorf("Received nil request.")
	}
	// Convert the begin and end timestamps into RFC3339 strings we can use for prefix matching.
	begin := []byte(time.Unix(listRequest.Begin, 0).Format(time.RFC3339))
	end := []byte(time.Unix(listRequest.End, 0).Format(time.RFC3339))

	commitIDs := []*CommitID{}

	// Do a prefix scan and record all the CommitIDs that match.
	scan := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME)).Cursor()
		for k, _ := c.Seek(begin); k != nil && bytes.Compare(k, end) <= 0; k, _ = c.Next() {
			cid, err := CommitIDFromBytes(k)
			if err != nil {
				return fmt.Errorf("scan: Failed to deserialize a commit id: %s", err)
			}
			commitIDs = append(commitIDs, cid)
		}
		return nil
	}

	if err := ts.db.View(scan); err != nil {
		return nil, fmt.Errorf("Failed to scan for commits: %s", err)
	}

	ret := &ListResponse{
		Commitids: commitIDs,
	}
	return ret, nil
}

func (ts *TraceServiceImpl) GetValues(ctx context.Context, getValuesRequest *GetValuesRequest) (*GetValuesResponse, error) {
	if getValuesRequest == nil {
		return nil, fmt.Errorf("Received nil request.")
	}

	ret := &GetValuesResponse{
		Values: map[string][]byte{},
	}

	// Load the values from the datastore.
	load := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME))
		tid := tx.Bucket([]byte(TRACEID_BUCKET_NAME))

		key, err := CommitIDToBytes(getValuesRequest.Commitid)
		if err != nil {
			return err
		}
		// Load the raw data and convert it into a commitInfo.
		data, err := newCommitInfo(c.Get(key))
		if err != nil {
			return fmt.Errorf("Unable to decode stored values: %s", err)
		}
		// Pull data out of commitInfo and put into the GetValuesResponse.
		for id64, value := range data.Values {
			// Look up the traceid from the trace64id, first from the in-memory
			// cache, and then from within the BoltDB if not found.
			ts.mutex.Lock()
			cached, ok := ts.cache.Get(id64)
			ts.mutex.Unlock()
			if ok {
				ret.Values[cached.(string)] = value
			} else {
				if b := tid.Get(bytesFromUint64(id64)); b == nil {
					return fmt.Errorf("Failed to get traceid for trace64id %d", id64)
				} else {
					ret.Values[string(b)] = value
				}
			}
		}
		return nil
	}
	if err := ts.db.View(load); err != nil {
		return nil, fmt.Errorf("Failed to load data for commitid: %#v, %s", *(getValuesRequest.Commitid), err)
	}

	return ret, nil
}

func (ts *TraceServiceImpl) GetParams(ctx context.Context, getParamsRequest *GetParamsRequest) (*GetParamsResponse, error) {
	if getParamsRequest == nil {
		return nil, fmt.Errorf("Received nil request.")
	}

	ret := &GetParamsResponse{
		Params: map[string]*Params{},
	}
	load := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACE_BUCKET_NAME))
		for _, traceid := range getParamsRequest.Traceids {
			entry := &StoredEntry{}
			if err := proto.Unmarshal(t.Get([]byte(traceid)), entry); err != nil {
				return fmt.Errorf("Failed to unmarshal StoredEntry proto for %s: %s", traceid, err)
			}
			ret.Params[traceid] = entry.Params
		}

		return nil
	}
	if err := ts.db.View(load); err != nil {
		return nil, fmt.Errorf("GetParams: Failed to load data: %s", err)
	}

	return ret, nil
}

// Close closes the underlying datastore and it not part of the TraceServiceServer interface.
func (ts *TraceServiceImpl) Close() error {
	return ts.db.Close()
}
