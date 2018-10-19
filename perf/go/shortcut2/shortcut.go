// shortcut handles storing and retrieving shortcuts.
package shortcut2

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/ds"
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
	return InsertShortcut(shortcut)
}

// Insert adds the shortcut content into the database. The id of the shortcut
// is returned.
func InsertShortcut(shortcut *Shortcut) (string, error) {
	sort.Strings(shortcut.Keys)
	h := md5.New()
	for _, s := range shortcut.Keys {
		_, _ = io.WriteString(h, s)
	}

	key := ds.NewKey(ds.SHORTCUT)
	key.Name = fmt.Sprintf("X%x", h.Sum(nil))
	var err error
	key, err = ds.DS.Put(context.TODO(), key, shortcut)
	if err != nil {
		return "", fmt.Errorf("Failed to store shortcut: %s", err)
	}
	return key.Name, nil
}

// Get retrieves a parsed shortcut for the given id.
func Get(id string) (*Shortcut, error) {
	ret := &Shortcut{}

	key := ds.NewKey(ds.SHORTCUT)
	if strings.HasPrefix(id, "X") {
		key.Name = id
	} else {
		i, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error invalid id: %s", id)
		}
		key.ID = i
	}
	if err := ds.DS.Get(context.TODO(), key, ret); err != nil {
		return nil, fmt.Errorf("Error retrieving shortcut from db: %s", err)
	}
	return ret, nil
}
