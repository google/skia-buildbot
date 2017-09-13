// shortcut handles storing and retrieving shortcuts.
package shortcut2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/ds"
)

type Shortcut struct {
	Keys []string `json:"keys" datastore:",noindex"`
}

var (
	useCloudDatastore bool
)

// Init initializes shortcut2.
//
// useDS - Is true if shortcuts should be store in Google Cloud Datastore, otherwise they're stored in Cloud SQL.
func Init(useDS bool) {
	useCloudDatastore = useDS
}

// Insert adds the shortcut content into the database. The id of the shortcut
// is returned.
func Insert(r io.Reader) (string, error) {
	if useCloudDatastore {
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
	} else {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return "", fmt.Errorf("Unable to read shortcut body: %s", err)
		}
		result, err := db.DB.Exec(`INSERT INTO shortcuts (traces) VALUES (?)`, string(b))
		if err != nil {
			return "", fmt.Errorf("Error while inserting shortcut: %v", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return "", fmt.Errorf("Error retrieving ID of new shortcut: %v", err)
		}
		return fmt.Sprintf("%d", id), nil
	}
}

// Get retrieves a parsed shortcut for the given id.
func Get(id string) (*Shortcut, error) {
	ret := &Shortcut{}
	if useCloudDatastore {
		i, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error invalid id: %s", id)
		}
		key := ds.NewKey(ds.SHORTCUT)
		key.ID = i
		if err := ds.DS.Get(context.TODO(), key, ret); err != nil {
			return nil, fmt.Errorf("Error retrieving shortcut from db: %s", err)
		}
	} else {
		var s string
		if err := db.DB.QueryRow(`SELECT traces FROM shortcuts WHERE id =?`, id).Scan(&s); err != nil {
			return nil, fmt.Errorf("Error retrieving shortcut from db: %s", err)
		}
		if err := json.Unmarshal([]byte(s), ret); err != nil {
			return nil, fmt.Errorf("Error decoding shortcut: %s", err)
		}
	}
	return ret, nil
}

func Write(id string, s *Shortcut) error {
	if !useCloudDatastore {
		return fmt.Errorf("Write is only usable on Cloud Datastore.")
	}
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("Error invalid id: %s", id)
	}
	key := ds.NewKey(ds.SHORTCUT)
	key.ID = i
	_, err = ds.DS.Put(context.Background(), key, s)
	return err
}

func List() (map[string]*Shortcut, error) {
	if useCloudDatastore {
		return nil, fmt.Errorf("List is only usable on MySQL.")
	}
	ret := map[string]*Shortcut{}
	var id string
	var s string
	rows, err := db.DB.Query(`SELECT id, traces FROM shortcuts`)
	defer func() {
		err := rows.Close()
		sklog.Errorf("MySQL error from iterating rows: %s %s", err, rows.Err())
	}()
	if err != nil {
		return nil, fmt.Errorf("Error retrieving shortcut from db: %s", err)
	}

	for rows.Next() {
		if err := rows.Scan(&id, &s); err != nil {
			return nil, fmt.Errorf("Failed to read: %s", err)
		}
		shortcut := &Shortcut{}
		if err := json.Unmarshal([]byte(s), shortcut); err != nil {
			sklog.Warningf("Error decoding shortcut %s: %s", id, err)
			continue
		}
		ret[id] = shortcut
	}

	return ret, nil
}
