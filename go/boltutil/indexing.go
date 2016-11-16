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
}

func New(config *Config) (*Store, error) {
	return nil, nil
}

type ViewFn func(tx *bolt.Tx) error
type UpdateFn func(tx *bolt.Tx, rec Interface)

func (s *Store) View() (interface{}, error) {
	return nil, nil
}

func (s *Store) Update(data interface{}) error {

	return nil
}

type Interface interface{}

type IndexedDB struct{}
