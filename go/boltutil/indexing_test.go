package boltutil

import (
	"testing"

	"github.com/boltdb/bolt"
	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/util"
)

type SomeStruct struct{}

var codec util.LRUCache = nil

func TestDB(t *testing.T) {
	config := Config{
		Directory: "./testdata",
		Name:      "./test-store",
		Indices:   []string{"index1", "index2", "index3"},
	}

	store, err := NewStore(&config)
	assert.NoError(t, err)

	key1 := "key_1"
	key2 := "key_2"
	example1 := SomeStruct{}
	example2 := SomeStruct{}

	// index1 := "index1"
	// index2 := "index2"
	mainIndex := ""
	keys := []string{key1, key2}

	err = store.Read(mainIndex, keys, func(tx *bolt.Tx, recs []Interface) error {
		return nil
	})

	err = store.Write(mainIndex, keys, func(tx *bolt.Tx, recs []Interface) error {
		return nil
	})

	recs := []Interface{example1, example2}

	err = store.Create(recs)
	err = store.Delete(mainIndex, key1, key2)

	// db.View
	// db.Update
	//
}
