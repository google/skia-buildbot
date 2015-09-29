package sharedb

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_out=plugins=grpc:. sharedb.proto

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// New returns a ShareDBClient that is connected to the given server address.
// The returned client can then be used to make RPC calls.
func New(serverAddress string) (ShareDBClient, error) {
	conn, err := grpc.Dial(serverAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return NewShareDBClient(conn), nil
}

// rpcServer implements the ShareDBServer define in the sharedb.proto file.
// This implementation is based on BoltDB. It stores key-value pairs that
// are addressable via: database/bucket/key.
// Each database maps to a file on the filesystem. The keys within a bucket
// are unique.
type rpcServer struct {
	dataDir   string
	databases map[string]*bolt.DB
	dbsMutex  sync.Mutex
}

// NewServer returns a instance that implements the ShareDBServer interface that
// was generated via the sharedb.proto file.
// It can then be used to run an RPC server. See tests for details.
func NewServer(dataDir string) ShareDBServer {
	ret := &rpcServer{
		dataDir:   fileutil.Must(fileutil.EnsureDirExists(dataDir)),
		databases: map[string]*bolt.DB{},
	}
	return ret
}

// Get returns the value for the given key identified by the tripple
// (database, bucket, key) in the provided KeyRequest instance.
func (r *rpcServer) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	db, err := r.getDB(req.Database, false)
	if err != nil {
		return nil, err
	}

	// If there is no database we are done.
	if db == nil {
		return nil, nil
	}

	result := GetResponse{}
	return &result, db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Bucket))
		if bucket == nil {
			return nil
		}

		val := bucket.Get([]byte(req.Key))
		result.Value = make([]byte, len(val))
		copy(result.Value, val)
		return nil
	})
}

// Put stores the value provided by the tuple (database, bucket, key, value) in
// the provided instance of KeyValueRequest.
// If the write succeeded, the return value AckReply.Ok will be true.
func (r *rpcServer) Put(ctx context.Context, req *PutRequest) (*PutResponse, error) {
	db, err := r.getDB(req.Database, true)
	if err != nil {
		return &PutResponse{false}, err
	}

	err = db.Batch(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(req.Bucket))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(req.Key), req.Value)
	})
	return &PutResponse{err == nil}, err
}

// Delete removes the key identified by (database, bucket, key) in KeyRequest.
// If the write succeeded, the return value AckReply.Ok will be true.
func (r *rpcServer) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	result := &DeleteResponse{}
	db, err := r.getDB(req.Database, false)
	if err != nil {
		return result, err
	}

	// If there is no database we are done.
	if db == nil {
		result.Ok = true
		return result, nil
	}

	err = db.Batch(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Bucket))
		if bucket == nil {
			return nil
		}

		return bucket.Delete([]byte(req.Key))
	})
	result.Ok = (err == nil)
	return result, err
}

// Databases returns the list of all databases currently managed by this server.
func (r *rpcServer) Databases(ctx context.Context, req *DatabasesRequest) (*DatabasesResponse, error) {
	result := &DatabasesResponse{}
	fileInfos, err := ioutil.ReadDir(r.dataDir)
	if err != nil {
		return result, err
	}

	dbs := make([]string, 0, len(fileInfos))
	for _, fi := range fileInfos {
		name := fi.Name()
		if strings.HasSuffix(name, ".db") {
			dbs = append(dbs, strings.TrimSuffix(name, ".db"))
		}
	}
	result.Values = dbs
	return result, nil
}

// Buckets returns the list of all buckets in the specified database.
func (r *rpcServer) Buckets(ctx context.Context, req *BucketsRequest) (*BucketsResponse, error) {
	result := &BucketsResponse{}
	db, err := r.getDB(req.Database, false)
	if err != nil {
		return result, err
	}

	result.Values = []string{}
	if db == nil {
		return result, nil
	}

	return result, db.View(func(tx *bolt.Tx) error {
		cursor := tx.Cursor()
		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			result.Values = append(result.Values, string(k))
		}
		return nil
	})
}

// Keys returns the list of all keys in the specified bucket.
func (r *rpcServer) Keys(ctx context.Context, req *KeysRequest) (*KeysResponse, error) {
	result := &KeysResponse{}
	db, err := r.getDB(req.Database, false)
	if err != nil {
		return result, err
	}

	result.Values = []string{}
	if db == nil {
		return result, nil
	}

	// continueScan is set depending if we have a prefix scan or a
	// minPrefix - maxPrefix range scan.
	continueScan := func(k []byte) bool { return true }
	minKey := []byte(req.Prefix)
	switch {
	case req.MaxPrefix != "":
		continueScan = func(k []byte) bool { return bytes.Compare(k, []byte(req.MaxPrefix)) <= 0 }
		fallthrough
	case req.MinPrefix != "":
		minKey = []byte(req.MinPrefix)
	case req.Prefix != "":
		continueScan = func(k []byte) bool { return bytes.HasPrefix(k, []byte(req.Prefix)) }
	}

	return result, db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Bucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for k, _ := cursor.Seek(minKey); (k != nil) && continueScan(k); k, _ = cursor.Next() {
			result.Values = append(result.Values, string(k))
		}
		return nil
	})
}

// getDB returns a BoltDB if the instance exists in the internal map of
// databases or on disk. Otherwise it will create the database on disk if
// the 'create' parameter is true. If the database does not exist and create
// is false, it will return (nil, nil).
func (r *rpcServer) getDB(database string, create bool) (*bolt.DB, error) {
	r.dbsMutex.Lock()
	defer r.dbsMutex.Unlock()

	db, ok := r.databases[database]
	if ok {
		return db, nil
	}

	// Check if the database exists on disk.
	databaseFile := filepath.Join(r.dataDir, database+".db")
	if !create && !fileutil.FileExists(databaseFile) {
		return nil, nil
	}

	db, err := bolt.Open(databaseFile, 0644, nil)
	if err != nil {
		return nil, err
	}
	r.databases[database] = db
	return db, nil
}
