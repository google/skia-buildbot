package boltutil

import (
	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/util"
)

type Config struct {
	Directory string
	Name      string
	Indices   []string
	Codec     util.LRUCache
}

type Store struct {
	*bolt.DB
}

func NewStore(config *Config) (*Store, error) {
	// baseDir, err := fileutil.EnsureDirExists(baseDir)
	// if err != nil {
	//   return nil, err
	// }
	//
	// dbName := path.Join(baseDir, "ignorestore.db")
	// db, err := bolt.Open(dbName, 0600, nil)
	// if err != nil {
	//   return nil, err
	// }
	// 	ret := &boltIssueStore{
	// 		db: db,
	// 	}
	//
	// 	return ret, ret.initBuckets()

	return nil, nil
}

type ReadFn func(*bolt.Tx, []Interface) error
type WriteFn func(*bolt.Tx, []Interface) error
type CreateFn func(*bolt.Tx, []Interface) error

func (s *Store) Read(idx string, keys []string, readFn ReadFn) error {
	return nil
}

func (s *Store) Write(idx string, keys []string, writeFn WriteFn) error {
	return nil
}

func (s *Store) Create(recs []Interface, createFn CreateFn) error {
	return nil
}

func (s *Store) Delete(keys []string) error {
	return nil
}

type Interface interface{}

type IndexedDB struct{}
