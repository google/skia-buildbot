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
	return nil, nil
}

type ReadFn func(*bolt.Tx, []Interface) error
type WriteFn func(*bolt.Tx, []Interface) error

func (s *Store) Read(idx string, keys []string, readFn ReadFn) error {
	return nil
}

func (s *Store) Write(idx string, keys []string, writeFn WriteFn) error {
	return nil
}

func (s *Store) Create(recs []Interface) error {
	return nil
}

func (s *Store) Delete(keys ...string) error {
	return nil
}

type Interface interface {
	IndexVals(index string) []string
}

type IndexedDB struct{}
