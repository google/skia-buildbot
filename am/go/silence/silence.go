package silence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
)

const (
	SILENCE_PARENT_KEY = "-silence-"

	NUM_RECENTLY_ARCHIVED = 500
)

// Silence is a filter that matches Incidents and is used to silence them.
type Silence struct {
	Key            string              `json:"key" datastore:"-"`
	Active         bool                `json:"active" datastore:"active"`
	User           string              `json:"user" datastore:"user"`
	ParamSet       paramtools.ParamSet `json:"param_set" datastore:"-"`
	ParamSetSerial string              `json:"-" datastore:"param_set_serial,noindex"`
	Created        int64               `json:"created" datastore:"created"`
	Updated        int64               `json:"updated" datastore:"updated"`
	Duration       string              `json:"duration" datastore:"duration"`
	Notes          []note.Note         `json:"notes" datastore:"notes,flatten"`
}

// New creates a new Silence.
//
// user - Email address of the person that created the silence.
func New(user string) *Silence {
	now := time.Now().Unix()
	return &Silence{
		Active:   true,
		User:     user,
		ParamSet: paramtools.ParamSet{},
		Created:  now,
		Updated:  now,
		Duration: "2h",
		Notes:    []note.Note{},
	}
}

// Load converts the JSON paramset back into a paramset.
func (silence *Silence) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(silence, ps); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(silence.ParamSetSerial), &silence.ParamSet); err != nil {
		return err
	}
	return nil
}

// Save serializes the paramset as JSON for storing in the Datastore.
func (silence *Silence) Save() ([]datastore.Property, error) {
	b, err := json.Marshal(silence.ParamSet)
	if err != nil {
		return nil, err
	}
	silence.ParamSetSerial = string(b)
	return datastore.SaveStruct(silence)
}

// Store saves and updates silences in Cloud Datastore.
type Store struct {
	ds *datastore.Client
}

// NewStore creates a new Store from the given Datastore client.
func NewStore(ds *datastore.Client) *Store {
	store := &Store{
		ds: ds,
	}
	// Start a go routine that expires old silences.
	go func(store *Store) {
		for range time.Tick(15 * time.Second) {
			now := time.Now()
			silences, err := store.GetAll()
			if err != nil {
				sklog.Errorf("Silence expirer failed to retrieve silences: %s", err)
			}
			for _, s := range silences {
				d, err := human.ParseDuration(s.Duration)
				if err != nil {
					sklog.Errorf("Silence has invalid duration: %s", err)
					continue
				}
				if time.Unix(s.Created, 0).Add(d).Before(now) {
					if _, err := store.Archive(s.Key); err != nil {
						sklog.Errorf("Failed to archive expired silence: %s", err)
					}
				}
			}
		}
	}(store)
	return store
}

func (s *Store) Put(silence *Silence) (*Silence, error) {
	_, err := human.ParseDuration(silence.Duration)
	if err != nil {
		return nil, fmt.Errorf("Silence has invalid duration: %s", err)
	}

	// Key used if this is a create.
	ancestor := ds.NewKey(ds.SILENCE_ACTIVE_PARENT_AM)
	ancestor.Name = SILENCE_PARENT_KEY
	key := ds.NewKey(ds.SILENCE_AM)
	key.Parent = ancestor

	if silence.Key != "" {
		// This is an update, so use the key provided.
		var err error
		key, err = datastore.DecodeKey(silence.Key)
		if err != nil {
			return nil, err
		}
	}

	silence.Active = true

	var pendingKey *datastore.PendingKey
	commit, err := s.ds.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var err error
		if pendingKey, err = tx.Put(key, silence); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to write Silence: %s", err)
	}

	silence.Key = commit.Key(pendingKey).Encode()
	return silence, nil

}

// _mutate is a helper function for updating Silences inside a transaction.
func (s *Store) _mutate(encodedKey string, mutator func(silence *Silence) error) (*Silence, error) {
	key, err := datastore.DecodeKey(encodedKey)
	if err != nil {
		return nil, err
	}
	var silence Silence
	_, err = s.ds.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		if err := tx.Get(key, &silence); err != nil {
			return err
		}
		if err := mutator(&silence); err != nil {
			return err
		}
		if _, err := tx.Put(key, &silence); err != nil {
			return err
		}
		return nil
	})
	silence.Key = encodedKey
	return &silence, err
}

func (s *Store) Archive(encodedKey string) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		silence.Active = false
		silence.Updated = time.Now().Unix()
		return nil
	})
}

func (s *Store) Reactivate(encodedKey, duration, user string) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		now := time.Now().Unix()
		silence.Active = true
		silence.Created = now
		silence.Updated = now
		silence.Duration = duration
		silence.Notes = append(silence.Notes, note.Note{
			Text:   fmt.Sprintf("Reactivated by %q.", user),
			Author: user,
			TS:     now,
		})

		return nil
	})
}

func (s *Store) Delete(encodedKey string) error {
	key, err := datastore.DecodeKey(encodedKey)
	if err != nil {
		return err
	}
	if err := s.ds.Delete(context.Background(), key); err != nil {
		return fmt.Errorf("Failed to delete Silence: %s", err)
	}

	return nil
}

func (s *Store) AddNote(encodedKey string, note note.Note) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		silence.Updated = time.Now().Unix()
		silence.Notes = append(silence.Notes, note)
		return nil
	})
}

func (s *Store) DeleteNote(encodedKey string, index int) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		if index < 0 || index > len(silence.Notes)-1 {
			return fmt.Errorf("Index for delete out of range.")
		}
		silence.Updated = time.Now().Unix()
		silence.Notes = append(silence.Notes[:index], silence.Notes[index+1:]...)
		return nil
	})
}

// GetAll returns a list of all active Silences.
func (s *Store) GetAll() ([]Silence, error) {
	var active []Silence
	ancestor := ds.NewKey(ds.SILENCE_ACTIVE_PARENT_AM)
	ancestor.Name = SILENCE_PARENT_KEY
	q := ds.NewQuery(ds.SILENCE_AM).Filter("active=", true).Ancestor(ancestor)
	keys, err := s.ds.GetAll(context.Background(), q, &active)
	for i, key := range keys {
		if active[i].Key == "" {
			active[i].Key = key.Encode()
		}
	}
	return active, err
}

// GetRecentlyArchived returns N most recently archived Silences that were
// updated within the specified duration. updatedWithin can be 0 if we want
// all recently archived silences.
func (s *Store) GetRecentlyArchived(updatedWithin time.Duration) ([]Silence, error) {
	var archived []Silence
	ancestor := ds.NewKey(ds.SILENCE_ACTIVE_PARENT_AM)
	ancestor.Name = SILENCE_PARENT_KEY
	modifiedAfter := int64(0)
	if updatedWithin != 0 {
		modifiedAfter = time.Now().Add(-updatedWithin).Unix()
	}
	q := ds.NewQuery(ds.SILENCE_AM).Filter("active=", false).Filter("updated>", modifiedAfter).Ancestor(ancestor).Order("-updated").Limit(NUM_RECENTLY_ARCHIVED)
	keys, err := s.ds.GetAll(context.Background(), q, &archived)
	if err != nil {
		return nil, fmt.Errorf("Failed to make query: %s", err)
	}
	for i, key := range keys {
		if archived[i].Key == "" {
			archived[i].Key = key.Encode()
		}
	}
	return archived, err
}
