// Copyright 2018 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package silence

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	SILENCE_PARENT_KEY = "-silence-"

	NUM_RECENTLY_ARCHIVED = 500

	// RECENTLY_EXPIRED_DURATION is the default duration within which archived silences
	// are considered recently expired.
	RECENTLY_EXPIRED_DURATION = 2 * time.Hour
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

// ValidateRegexes returns an error if the silence is not valid.
func (silence *Silence) ValidateRegexes() error {
	for _, vals := range silence.ParamSet {
		for _, v := range vals {
			if _, err := regexp.Compile(fmt.Sprintf(`^%s$`, v)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Store saves and updates silences in Cloud Datastore.
type Store interface {
	// Put writes a Silence to the store, creating a new Silence if its Key is
	// empty or updating the existing Silence, and marks it active.
	Put(silence *Silence) (*Silence, error)

	// Archive marks the Silence with the given encoded Datastore key as inactive.
	Archive(encodedKey string) (*Silence, error)

	// Reactivate marks the Silence with the given encoded Datastore key as
	// active with a new duration and records the user who reactivated it.
	Reactivate(encodedKey, duration, user string) (*Silence, error)

	// Delete removes the Silence with the given encoded Datastore key from the store.
	Delete(encodedKey string) error

	// AddNote adds a note to the Silence with the given encoded Datastore key.
	AddNote(encodedKey string, note note.Note) (*Silence, error)

	// DeleteNote deletes the note at the given index from the Silence with the
	// given encoded Datastore key.
	DeleteNote(encodedKey string, index int) (*Silence, error)

	// GetAll returns a list of all active Silences.
	GetAll() ([]Silence, error)

	// GetRecentlyArchived returns N most recently archived Silences that were
	// updated within the specified duration. updatedWithin can be 0 if we want
	// all recently archived silences.
	GetRecentlyArchived(updatedWithin time.Duration) ([]Silence, error)
}

// StoreImpl implements Store using Cloud Datastore.
type StoreImpl struct {
	ds *datastore.Client
}

// NewStore creates a new StoreImpl from the given Datastore client.
func NewStore(ds *datastore.Client) *StoreImpl {
	return &StoreImpl{
		ds: ds,
	}
}

// StartExpirer starts a goroutine that periodically archives expired silences
// until the given context is canceled.
func (s *StoreImpl) StartExpirer(ctx context.Context) {
	go util.RepeatCtx(ctx, 15*time.Second, func(ctx context.Context) {
		now := time.Now()
		silences, err := s.GetAll()
		if err != nil {
			sklog.Errorf("Silence expirer failed to retrieve silences: %s", err)
		}
		for _, silence := range silences {
			d, err := human.ParseDuration(silence.Duration)
			if err != nil {
				sklog.Errorf("Silence has invalid duration: %s", err)
				continue
			}
			if time.Unix(silence.Created, 0).Add(d).Before(now) {
				if _, err := s.Archive(silence.Key); err != nil {
					sklog.Errorf("Failed to archive expired silence: %s", err)
				}
			}
		}
	})
}

func (s *StoreImpl) Put(silence *Silence) (*Silence, error) {
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
func (s *StoreImpl) _mutate(encodedKey string, mutator func(silence *Silence) error) (*Silence, error) {
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

func (s *StoreImpl) Archive(encodedKey string) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		silence.Active = false
		silence.Updated = time.Now().Unix()
		return nil
	})
}

func (s *StoreImpl) Reactivate(encodedKey, duration, user string) (*Silence, error) {
	_, err := human.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("Silence has invalid duration: %s", err)
	}

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

func (s *StoreImpl) Delete(encodedKey string) error {
	key, err := datastore.DecodeKey(encodedKey)
	if err != nil {
		return err
	}
	if err := s.ds.Delete(context.Background(), key); err != nil {
		return fmt.Errorf("Failed to delete Silence: %s", err)
	}

	return nil
}

func (s *StoreImpl) AddNote(encodedKey string, note note.Note) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		silence.Updated = time.Now().Unix()
		silence.Notes = append(silence.Notes, note)
		return nil
	})
}

func (s *StoreImpl) DeleteNote(encodedKey string, index int) (*Silence, error) {
	return s._mutate(encodedKey, func(silence *Silence) error {
		if index < 0 || index > len(silence.Notes)-1 {
			return fmt.Errorf("Index for delete out of range.")
		}
		silence.Updated = time.Now().Unix()
		silence.Notes = append(silence.Notes[:index], silence.Notes[index+1:]...)
		return nil
	})
}

func (s *StoreImpl) GetAll() ([]Silence, error) {
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

func (s *StoreImpl) GetRecentlyArchived(updatedWithin time.Duration) ([]Silence, error) {
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

var _ Store = (*StoreImpl)(nil)
