package silence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/paramtools"
)

const (
	SILENCE_PARENT_KEY = "-silence-"
)

type Silence struct {
	Key            string              `json:"key" datastore:"-"`
	Active         bool                `json:"active" datastore:"active"`
	User           string              `json:"user" datastore:"user"`
	ParamSet       paramtools.ParamSet `json:"param_set" datastore:"-"`
	ParamSetSerial string              `json:"-" datastore:"param_set_serial"`
	Created        int64               `json:"created" datastore:"created"`
	Duration       time.Duration       `json:"duration" datastore:"duration"`
	Notes          []note.Note         `json:"notes" datastore:"notes,flatten"`
}

func (silence *Silence) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(silence, ps); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(silence.ParamSetSerial), &silence.ParamSet); err != nil {
		return err
	}
	return nil
}

func (silence *Silence) Save() ([]datastore.Property, error) {
	b, err := json.Marshal(silence.ParamSet)
	if err != nil {
		return nil, err
	}
	silence.ParamSetSerial = string(b)
	return datastore.SaveStruct(silence)
}

type Store struct {
	ds *datastore.Client
}

func NewStore(ds *datastore.Client) *Store {
	return &Store{
		ds: ds,
	}
}

func (s *Store) Create(silence *Silence) (*Silence, error) {
	ancestor := ds.NewKey(ds.SILENCE_ACTIVE_PARENT_AM)
	ancestor.Name = SILENCE_PARENT_KEY
	key := ds.NewKey(ds.SILENCE_AM)
	key.Parent = ancestor

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

func (s *Store) GetRecentlyArchived() ([]Silence, error) {
	var archived []Silence
	ancestor := ds.NewKey(ds.SILENCE_ACTIVE_PARENT_AM)
	ancestor.Name = SILENCE_PARENT_KEY
	q := ds.NewQuery(ds.SILENCE_AM).Filter("active=", false).Ancestor(ancestor).Order("-created").Limit(20)
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
