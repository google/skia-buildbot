// shortcut handles storing and retrieving shortcuts.
package shortcut2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"go.skia.org/infra/perf/go/ds"
)

type Shortcut struct {
	Keys []string `json:"keys" datastore:",noindex"`
}

// Insert adds the shortcut content into the database. The id of the shortcut
// is returned.
func Insert(r io.Reader) (string, error) {
	shortcut := &Shortcut{}
	if err := json.NewDecoder(r).Decode(shortcut); err != nil {
		return "", fmt.Errorf("Unable to read shortcut body: %s", err)
	}
	key := ds.NewKey(ds.SHORTCUT)
	var err error
	key, err = ds.DS.Put(context.TODO(), key, shortcut)
	if err != nil {
		return "", fmt.Errorf("Failed to store shortcut: %s", err)
	}
	return fmt.Sprintf("%d", key.ID), nil
}

// Get retrieves a parsed shortcut for the given id.
func Get(id string) (*Shortcut, error) {
	ret := &Shortcut{}
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Error invalid id: %s", id)
	}
	key := ds.NewKey(ds.SHORTCUT)
	key.ID = i
	if err := ds.DS.Get(context.TODO(), key, ret); err != nil {
		return nil, fmt.Errorf("Error retrieving shortcut from db: %s", err)
	}
	return ret, nil
}

func Write(id string, s *Shortcut) error {
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("Error invalid id: %s", id)
	}
	key := ds.NewKey(ds.SHORTCUT)
	key.ID = i
	_, err = ds.DS.Put(context.Background(), key, s)
	return err
}
