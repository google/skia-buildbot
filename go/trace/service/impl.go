package traceservice

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_out=plugins=grpc:. traceservice.proto

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/groupcache/lru"
	"github.com/golang/protobuf/proto"
	"go.skia.org/infra/go/metrics2"
)

const (
	COMMIT_BUCKET_NAME  = "commits"
	TRACE_BUCKET_NAME   = "traces"
	TRACEID_BUCKET_NAME = "traceids"
	LARGEST_TRACEID_KEY = "the largest trace64id"

	// How many items to keep in the in-memory LRU cache.
	MAX_INT64_ID_CACHED = 1024 * 1024
)

var (
	tags               = map[string]string{"module": "tracedb"}
	missingParamsCalls = metrics2.GetCounter("missing_params_calls", tags)
	addParamsCalls     = metrics2.GetCounter("add_params_calls", tags)
	addCalls           = metrics2.GetCounter("add_calls", tags)
	addCount           = metrics2.GetCounter("added_count", tags)
	removeCalls        = metrics2.GetCounter("remove_calls", tags)
	listCalls          = metrics2.GetCounter("list_calls", tags)
	getParamsCalls     = metrics2.GetCounter("get_params_calls", tags)
	getValuesCalls     = metrics2.GetCounter("get_values_calls", tags)
	getValuesRawCalls  = metrics2.GetCounter("get_values_raw_calls", tags)
	getTraceIDsCalls   = metrics2.GetCounter("get_traceids_calls", tags)
	pingCalls          = metrics2.GetCounter("ping_calls", tags)
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

	// cache is an in-memory LRU cache for traceids <-> trace64ids and commitid -> md5.
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

// addMD5 adds the md5 of the raw bytes for the given key, which should
// be a CommitID as a byte slice.
//
// The md5 is stored as a hex formatted string.
//
// This func doesn't lock the cache, which should be done by the caller.
func (ts *TraceServiceImpl) addMD5(key, raw []byte) string {
	hash := fmt.Sprintf("%x", md5.Sum(raw))
	ts.cache.Add(string(key), hash)
	return hash
}

// getMD5 retrieves the md5 of the raw bytes for the given key, which should
// be a CommitID as a byte slice. If the md5 isn't in the cache then it is
// calculated and added to the cache.
//
// The md5 is stored as a hex formatted string.
//
func (ts *TraceServiceImpl) getMD5(key, raw []byte) string {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if hash, ok := ts.cache.Get(string(key)); !ok {
		return ts.addMD5(key, raw)
	} else {
		return hash.(string)
	}
}

// readMD5 retrieves the md5 of the raw bytes for the given key, which should
// be a CommitID as a byte slice. If the md5 isn't in the cache then "" is returned.
//
// The md5 is stored as a hex formatted string.
//
func (ts *TraceServiceImpl) readMD5(key []byte) string {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if hash, ok := ts.cache.Get(string(key)); !ok {
		return ""
	} else {
		return hash.(string)
	}
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

// CommitInfo is the value stored in the commit bucket.
type CommitInfo struct {
	Values map[uint64][]byte
}

// NewCommitInfo returns a CommitInfo with data deserialized from the byte slice.
//
// The byte slice is in the format returned from GetValuesRaw.
func NewCommitInfo(volatile []byte) (*CommitInfo, error) {
	ret := &CommitInfo{
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

// ToBytes serializes the data in the CommitInfo into a byte slice. The format is ingestable by FromBytes.
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
func (c *CommitInfo) ToBytes() []byte {
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
	missingParamsCalls.Inc(1)
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

func (ts *TraceServiceImpl) AddParams(ctx context.Context, in *AddParamsRequest) (*Empty, error) {
	addParamsCalls.Inc(1)
	// Serialize the Params for each trace as a proto and collect the traceids.
	// We do this outside the add func so there's less work taking place in the
	// Update transaction.
	params := map[string][]byte{}
	var err error
	for _, p := range in.Params {
		ti := &StoredEntry{
			Params: &Params{
				Params: p.Params,
			},
		}
		params[p.Key], err = proto.Marshal(ti)
		if err != nil {
			return nil, fmt.Errorf("Failed to serialize the Params: %s", err)
		}
	}

	// Add the Params for each traceid to the bucket.
	add := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACE_BUCKET_NAME))
		for _, p := range in.Params {
			if err := t.Put([]byte(p.Key), params[p.Key]); err != nil {
				return fmt.Errorf("Failed to write the trace info for %s: %s", p.Key, err)
			}
		}
		return nil
	}
	if err := ts.db.Update(add); err != nil {
		return nil, fmt.Errorf("Failed to add values to tracedb: %s", err)
	}
	return &Empty{}, nil
}

func (ts *TraceServiceImpl) Add(ctx context.Context, in *AddRequest) (*Empty, error) {
	addCalls.Inc(1)
	if in == nil {
		return nil, fmt.Errorf("Received nil request.")
	}
	if in.Commitid == nil {
		return nil, fmt.Errorf("Received nil CommitID")
	}
	if in.Values == nil {
		return nil, fmt.Errorf("Received nil Values")
	}
	addCount.Inc(int64(len(in.Values)))

	// Get the trace64ids for each traceid.
	keys := []string{}
	for _, entry := range in.Values {
		keys = append(keys, entry.Key)
	}

	trace64ids, err := ts.atomize(keys)
	if err != nil {
		return nil, fmt.Errorf("Failed to create short trace ids: %s", err)
	}

	add := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME))

		// Write the CommitInfo.
		key, err := CommitIDToBytes(in.Commitid)
		if err != nil {
			return err
		}

		// First load the existing info.
		data, err := NewCommitInfo(c.Get(key))
		if err != nil {
			return fmt.Errorf("Unable to decode stored values: %s", err)
		}

		// Add our new data points.
		for _, entry := range in.Values {
			data.Values[trace64ids[entry.Key]] = entry.Value
		}

		// Convert the CommitInfo back into bytes.
		b := data.ToBytes()

		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		// Update the md5 for the CommitID.
		_ = ts.addMD5(key, b)

		// Write to the datastore.
		if err := c.Put(key, b); err != nil {
			return fmt.Errorf("Failed to write the trace info for %s: %s", key, err)
		}
		return nil
	}

	if err := ts.db.Update(add); err != nil {
		return nil, fmt.Errorf("Failed to add values to tracedb: %s", err)
	}
	return &Empty{}, nil
}

func (ts *TraceServiceImpl) List(ctx context.Context, listRequest *ListRequest) (*ListResponse, error) {
	listCalls.Inc(1)
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
	getValuesCalls.Inc(1)
	if getValuesRequest == nil {
		return nil, fmt.Errorf("Received nil request.")
	}

	ret := &GetValuesResponse{
		Values: []*ValuePair{},
	}

	// Load the values from the datastore.
	load := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME))
		tid := tx.Bucket([]byte(TRACEID_BUCKET_NAME))

		key, err := CommitIDToBytes(getValuesRequest.Commitid)
		if err != nil {
			return err
		}

		// Load the raw data.
		b := c.Get(key)

		// Get the MD5 hash for the commit id.
		ret.Md5 = ts.getMD5(key, b)

		// Convert into a CommitInfo.
		data, err := NewCommitInfo(b)
		if err != nil {
			return fmt.Errorf("Unable to decode stored values: %s", err)
		}
		// Pull data out of CommitInfo and put into the GetValuesResponse.
		for id64, value := range data.Values {
			// Look up the traceid from the trace64id, first from the in-memory
			// cache, and then from within the BoltDB if not found.

			if b := tid.Get(bytesFromUint64(id64)); b == nil {
				return fmt.Errorf("Failed to get traceid for trace64id %d", id64)
			} else {
				ret.Values = append(ret.Values, &ValuePair{
					Key:   string(b),
					Value: value,
				})
			}
		}

		return nil
	}
	if err := ts.db.View(load); err != nil {
		return nil, fmt.Errorf("Failed to load data for commitid: %#v, %s", *(getValuesRequest.Commitid), err)
	}

	return ret, nil
}

func (ts *TraceServiceImpl) GetValuesRaw(ctx context.Context, getValuesRequest *GetValuesRequest) (*GetValuesRawResponse, error) {
	getValuesRawCalls.Inc(1)
	if getValuesRequest == nil {
		return nil, fmt.Errorf("Received nil request.")
	}

	ret := &GetValuesRawResponse{
		Value: []byte{},
	}

	// Load the values from the datastore.
	load := func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(COMMIT_BUCKET_NAME))
		key, err := CommitIDToBytes(getValuesRequest.Commitid)
		if err != nil {
			return err
		}
		ret.Value = c.Get(key)
		ret.Md5 = ts.getMD5(key, ret.Value)
		return nil
	}
	if err := ts.db.View(load); err != nil {
		return nil, fmt.Errorf("Failed to load data for commitid: %#v, %s", *(getValuesRequest.Commitid), err)
	}

	return ret, nil
}

func (ts *TraceServiceImpl) GetTraceIDs(ctx context.Context, getTraceIDsRequest *GetTraceIDsRequest) (*GetTraceIDsResponse, error) {
	getTraceIDsCalls.Inc(1)
	ret := &GetTraceIDsResponse{
		Ids: []*TraceIDPair{},
	}
	load := func(tx *bolt.Tx) error {
		tid := tx.Bucket([]byte(TRACEID_BUCKET_NAME))
		for _, trace64id := range getTraceIDsRequest.Id {
			if b := tid.Get(bytesFromUint64(trace64id)); b == nil {
				return fmt.Errorf("Failed to get traceid for trace64id %d", trace64id)
			} else {
				ret.Ids = append(ret.Ids, &TraceIDPair{
					Id:   string(b),
					Id64: trace64id,
				})
			}
		}
		return nil
	}
	if err := ts.db.View(load); err != nil {
		return nil, fmt.Errorf("Failed to load traceids: %s", err)
	}

	return ret, nil
}

func (ts *TraceServiceImpl) GetParams(ctx context.Context, getParamsRequest *GetParamsRequest) (*GetParamsResponse, error) {
	getParamsCalls.Inc(1)
	if getParamsRequest == nil {
		return nil, fmt.Errorf("Received nil request.")
	}

	ret := &GetParamsResponse{
		Params: []*ParamsPair{},
	}
	load := func(tx *bolt.Tx) error {
		t := tx.Bucket([]byte(TRACE_BUCKET_NAME))
		for _, traceid := range getParamsRequest.Traceids {
			entry := &StoredEntry{}
			if err := proto.Unmarshal(t.Get([]byte(traceid)), entry); err != nil {
				return fmt.Errorf("Failed to unmarshal StoredEntry proto for %s: %s", traceid, err)
			}

			// TODO(stephana): Find a way to ensure that no inconsitent data are
			// written to the database which cause entry.Params to be nil.
			// Returning an error below will cause certain higher level operations
			// to fail, but it will not crash the server.
			if entry.Params == nil {
				return fmt.Errorf("Got empty params for %s", traceid)
			}

			ret.Params = append(ret.Params, &ParamsPair{
				Key:    traceid,
				Params: entry.Params.Params,
			})
		}

		return nil
	}
	if err := ts.db.View(load); err != nil {
		return nil, fmt.Errorf("GetParams: Failed to load data: %s", err)
	}

	return ret, nil
}

func (ts *TraceServiceImpl) Ping(ctx context.Context, empty *Empty) (*Empty, error) {
	pingCalls.Inc(1)

	return &Empty{}, nil
}

// Close closes the underlying datastore. Not part of the TraceServiceServer interface.
func (ts *TraceServiceImpl) Close() error {
	return ts.db.Close()
}
