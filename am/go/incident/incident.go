package incident

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
)

// Note is one note attached to an Incident.
type Note struct {
	Text   string `json:"text" datastore:"text"`
	Author string `json:"author" datastore:"author"`
	TS     int64  `json:"ts" datastore:"ts"` // Time in seconds since the epoch.
}

// Well known keys for Incident.Params.
const (
	ALERT_NAME  = "alertname"
	CATEGORY    = "category"
	SEVERITY    = "severity"
	ID          = "id"
	ASSIGNED_TO = "assigned_to"
)

const (
	TX_RETRIES = 5
)

// Incident
//
// Silenced is a derived state of an incident, so we don't store it in Incident.
// Will appear in either the list of active or silenced incidents,
// so we don't keep that as part of the state since it is derived info.
type Incident struct {
	Key          string            `json:"key" datastore:"key"`             // Key is the web-safe serialized Datastore key for the incident.
	ID           string            `json:"id" datastore:"id"`               // Also appears in Params.
	Active       bool              `json:"active" datastore:"active"`       // Or archived.
	Start        int64             `json:"start" datastore:"start"`         // Time in seconds since the epoch.
	LastSeen     int64             `json:"last_seen" datastore:"last_seen"` // Time in seconds since the epoch.
	Params       map[string]string `json:"params" datastore:"-"`            // Params
	ParamsSerial string            `json:"-" datastore:"params_serial"`
	Notes        []Note            `json:"notes" datastore:"notes,flatten"`
}

func (in *Incident) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(in, ps); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(in.ParamsSerial), &in.Params); err != nil {
		return err
	}
	return nil
}

func (in *Incident) Save() ([]datastore.Property, error) {
	b, err := json.Marshal(in.Params)
	if err != nil {
		return nil, err
	}
	in.ParamsSerial = string(b)
	return datastore.SaveStruct(in)
}

type Store struct {
	ignoredAttr []string // key-value pairs to ignore when computing IDs, such as kubernetes_pod_name, instance, and pod_template_hash.
	ds          *datastore.Client
}

func NewStore(ds *datastore.Client, ignoredAttr []string) *Store {
	return &Store{
		ignoredAttr: ignoredAttr,
		ds:          ds,
	}
}

func idForAlert(m map[string]string) (string, error) {
	if m[ID] != "" {
		return m[ID], nil
	}
	if m[alerts.TYPE] == alerts.TYPE_HEALTHZ {
		return "", fmt.Errorf("Healthz events should be ignored.")
	}
	keys := paramtools.Params(m).Keys()
	sort.Strings(keys)
	h := md5.New()
	for _, key := range keys {
		if key == ID {
			continue
		}
		h.Write([]byte(key))
		h.Write([]byte(m[key]))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (s *Store) inFromAlert(m map[string]string, id string) (*Incident, error) {
	m[ID] = id
	now := time.Now().Unix()
	return &Incident{
		Active:   true,
		ID:       id,
		Start:    now,
		LastSeen: now,
		Params:   m,
		Notes:    []Note{},
	}, nil
}

func (s *Store) AlertArrival(m map[string]string) (*Incident, error) {
	// If there is a matching active alert then just update its LastUpdated
	// value, otherwise create a new Incident and store it.
	id, err := idForAlert(m)
	if err != nil {
		return nil, err
	}
	ancestor := ds.NewKey(ds.INCIDENT_ACTIVE_PARENT_AM)
	ancestor.Name = id
	key := ds.NewKey(ds.INCIDENT_AM)
	key.Parent = ancestor

	ctx := context.Background()
	var active []*Incident
	for i := 0; i < TX_RETRIES; i++ {
		tx, err := s.ds.NewTransaction(ctx)
		if err != nil {
			return nil, fmt.Errorf("Could not create transaction: %s", err)
		}
		q := ds.NewQuery(ds.INCIDENT_AM).Ancestor(ancestor).Filter("active=", true).Transaction(tx)

		keys, err := s.ds.GetAll(ctx, q, &active)
		if err != nil {
			sklog.Errorf("Failed to retrieve: %s", err)
			break
		}
		if len(active) == 0 {
			sklog.Infof("New: %s", id)
			in, err := s.inFromAlert(m, id)
			if err != nil {
				break
			}
			active = append(active, in)
		} else {
			key = keys[0]
			active[0].LastSeen = time.Now().Unix()
			active[0].Key = key.Encode()
		}
		pending, err := tx.Put(key, active[0])
		if err != nil {
			break
		}
		commit, err := tx.Commit()
		if err == datastore.ErrConcurrentTransaction {
			continue
		}
		active[0].Key = commit.Key(pending).Encode()
		break
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to save incoming alert %v: %s", m, err)
	}

	return active[0], nil
}

func (s *Store) _mutateIncident(encodedKey string, mutator func(in *Incident) error) (*Incident, error) {
	key, err := datastore.DecodeKey(encodedKey)
	if err != nil {
		return nil, err
	}
	var in Incident
	_, err = s.ds.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		if err := tx.Get(key, &in); err != nil {
			return err
		}
		if err := mutator(&in); err != nil {
			return err
		}
		if _, err := tx.Put(key, &in); err != nil {
			return err
		}
		return nil
	})
	in.Key = encodedKey
	return &in, err
}

func (s *Store) AddNote(encodedKey string, note Note) (*Incident, error) {
	return s._mutateIncident(encodedKey, func(in *Incident) error {
		in.Notes = append(in.Notes, note)
		return nil
	})
}

func (s *Store) DeleteNote(encodedKey string, index int) (*Incident, error) {
	return s._mutateIncident(encodedKey, func(in *Incident) error {
		if index < 0 || index > len(in.Notes)-1 {
			return fmt.Errorf("Index for delete out of range.")
		}
		in.Notes = append(in.Notes[:index], in.Notes[index+1:]...)
		return nil
	})
}

func (s *Store) Archive(encodedKey string) (*Incident, error) {
	return s._mutateIncident(encodedKey, func(in *Incident) error {
		in.Active = false
		return nil
	})
}

func (s *Store) Assign(encodedKey string, user string) (*Incident, error) {
	return s._mutateIncident(encodedKey, func(in *Incident) error {
		in.Params[ASSIGNED_TO] = user
		return nil
	})
}

// Returns a list of all active Incidents.
func (s *Store) GetAll() ([]Incident, error) {
	var active []Incident
	q := ds.NewQuery(ds.INCIDENT_AM).Filter("active=", true)
	keys, err := s.ds.GetAll(context.Background(), q, &active)
	for i, key := range keys {
		if active[i].Key == "" {
			active[i].Key = key.Encode()
		}
	}
	return active, err
}
