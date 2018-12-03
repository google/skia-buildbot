package incident

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Well known keys for Incident.Params.
const (
	ALERT_NAME  = "alertname"
	CATEGORY    = "category"
	SEVERITY    = "severity"
	ID          = "id"
	ASSIGNED_TO = "assigned_to"
)

const (
	TX_RETRIES                   = 5
	NUM_RECENTLY_RESOLVED        = 20
	NUM_RECENTLY_RESOLVED_FOR_ID = 20
)

// Incident - An alert that is being acted on.
//
// Each alert has an ID which is the same each time that exact alert is fired.
// Not to be confused with the Key which is the datastore key for a single
// incident of an alert firing. There will be many Incidents in the datastore
// with the same ID, but at most one will be Active.
type Incident struct {
	Key          string            `json:"key" datastore:"key"`             // Key is the web-safe serialized Datastore key for the incident.
	ID           string            `json:"id" datastore:"id"`               // Also appears in Params.
	Active       bool              `json:"active" datastore:"active"`       // Or archived.
	Start        int64             `json:"start" datastore:"start"`         // Time in seconds since the epoch.
	LastSeen     int64             `json:"last_seen" datastore:"last_seen"` // Time in seconds since the epoch.
	Params       map[string]string `json:"params" datastore:"-"`            // Params
	ParamsSerial string            `json:"-" datastore:"params_serial"`     // Params serialized as JSON for easy storing in the datastore.
	Notes        []note.Note       `json:"notes" datastore:"notes,flatten"`
}

// Load converts the JSON params back into a map[string]string.
func (in *Incident) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(in, ps); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(in.ParamsSerial), &in.Params); err != nil {
		return err
	}
	return nil
}

// Save serializes the params as JSON.
func (in *Incident) Save() ([]datastore.Property, error) {
	b, err := json.Marshal(in.Params)
	if err != nil {
		return nil, err
	}
	in.ParamsSerial = string(b)
	return datastore.SaveStruct(in)
}

// IsSilence returns if any of the given silences apply to this incident.
func (in *Incident) IsSilenced(silences []silence.Silence) bool {
	ps := paramtools.ParamSet{}
	for k, v := range in.Params {
		ps[k] = []string{v}
	}

	for _, s := range silences {
		if !s.Active {
			continue
		}
		if s.ParamSet.Matches(ps) {
			return true
		}
	}
	return false
}

// Store and retrieve Incidents from Cloud Datastore.
type Store struct {
	ignoredAttr []string // key-value pairs to ignore when computing IDs, such as kubernetes_pod_name, instance, and pod_template_hash.
	ds          *datastore.Client
}

// NewStore creates a new Store.
//
// ds - Datastore client.
// ignoredAttr - A list of keys to ignore when calculating an Incidents ID.
func NewStore(ds *datastore.Client, ignoredAttr []string) *Store {
	ignored := []string{}
	ignored = append(ignored, ignoredAttr...)
	ignored = append(ignored, alerts.STATE, ID, ASSIGNED_TO)
	return &Store{
		ignoredAttr: ignored,
		ds:          ds,
	}
}

// idForAlert calculates the ID for an Incident, which is the md5 sum of all
// the sorted non-ignored keys and values.
func (s *Store) idForAlert(m map[string]string) (string, error) {
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
		if util.In(key, s.ignoredAttr) {
			continue
		}
		h.Write([]byte(key))
		h.Write([]byte(m[key]))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// inFromAlert creates an Incident from an alert.
func (s *Store) inFromAlert(m map[string]string, id string) *Incident {
	m[ID] = id
	now := time.Now().Unix()
	return &Incident{
		Active:   true,
		ID:       id,
		Start:    now,
		LastSeen: now,
		Params:   m,
		Notes:    []note.Note{},
	}
}

// AlertArrival turns alerts into Incidents, or archives Incidents if
// the arriving state is resolved.
//
// Note that it is possible for the returned incident to be nil even if the
// returned error is non-nil. An example of when this could happen: If we
// receive an alert for an incident that is no longer active.
func (s *Store) AlertArrival(m map[string]string) (*Incident, error) {
	// If there is a matching active alert then just update its LastUpdated
	// value, otherwise create a new Incident and store it.
	id, err := s.idForAlert(m)
	if err != nil {
		return nil, err
	}
	ancestor := ds.NewKey(ds.INCIDENT_ACTIVE_PARENT_AM)
	ancestor.Name = id
	key := ds.NewKey(ds.INCIDENT_AM)
	key.Parent = ancestor

	alertState, ok := m[alerts.STATE]
	if !ok {
		alertState = alerts.STATE_ACTIVE
	}

	ctx := context.Background()
	var active []*Incident
	for i := 0; i < TX_RETRIES; i++ {
		// Inside a transaction.
		var tx *datastore.Transaction
		tx, err = s.ds.NewTransaction(ctx)
		if err != nil {
			return nil, fmt.Errorf("Could not create transaction: %s", err)
		}
		// Find all active Incidents with the given ID.
		q := ds.NewQuery(ds.INCIDENT_AM).Ancestor(ancestor).Filter("active=", true).Transaction(tx)

		var keys []*datastore.Key
		active = []*Incident{}
		keys, err = s.ds.GetAll(ctx, q, &active)
		if err != nil {
			sklog.Errorf("Failed to retrieve: %s", err)
			break
		}
		// Either create a new Incident or update an existing Incident.
		if len(active) == 0 {
			if alertState == alerts.STATE_RESOLVED {
				sklog.Warningf("Received alert for incident that isn't active: %s", id)
				return nil, nil
			}
			sklog.Infof("New: %s", id)
			in := s.inFromAlert(m, id)
			active = append(active, in)
		} else {
			key = keys[0]
			active[0].LastSeen = time.Now().Unix()
			active[0].Key = key.Encode()
		}
		// Write to the Datastore and keep track of the Incident key.
		active[0].Active = alertState != alerts.STATE_RESOLVED
		var pending *datastore.PendingKey
		pending, err = tx.Put(key, active[0])
		if err != nil {
			break
		}
		var commit *datastore.Commit
		commit, err = tx.Commit()
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

// _mutateIncident utility function to update an Incident in a transaction.
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

func (s *Store) AddNote(encodedKey string, note note.Note) (*Incident, error) {
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

func (s *Store) Assign(encodedKey string, user string) (*Incident, error) {
	return s._mutateIncident(encodedKey, func(in *Incident) error {
		in.Params[ASSIGNED_TO] = user
		return nil
	})
}

func (s *Store) Archive(encodedKey string) (*Incident, error) {
	return s._mutateIncident(encodedKey, func(in *Incident) error {
		in.Active = false
		return nil
	})
}

// GetAll returns a list of all active Incidents.
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

// GetRecentlyResolved returns the N most recently archived Incidents.
func (s *Store) GetRecentlyResolved() ([]Incident, error) {
	var resolved []Incident
	q := ds.NewQuery(ds.INCIDENT_AM).Filter("active=", false).Order("-last_seen").Limit(NUM_RECENTLY_RESOLVED)
	keys, err := s.ds.GetAll(context.Background(), q, &resolved)
	for i, key := range keys {
		if resolved[i].Key == "" {
			resolved[i].Key = key.Encode()
		}
	}
	return resolved, err
}

// GetRecentlyResolvedForID returns a list of the N most recent archived Incidents
// that don't match the given key.
func (s *Store) GetRecentlyResolvedForID(id, excludeKey string) ([]Incident, error) {
	ancestor := ds.NewKey(ds.INCIDENT_ACTIVE_PARENT_AM)
	ancestor.Name = id
	var resolved []Incident
	q := ds.NewQuery(ds.INCIDENT_AM).Ancestor(ancestor).Filter("active=", false).Order("-last_seen").Limit(NUM_RECENTLY_RESOLVED_FOR_ID)
	keys, err := s.ds.GetAll(context.Background(), q, &resolved)
	toDelete := -1
	for i, key := range keys {
		if resolved[i].Key == "" {
			resolved[i].Key = key.Encode()
		}
		if resolved[i].Key == excludeKey {
			toDelete = i
		}
	}
	if toDelete != -1 {
		resolved = append(resolved[:toDelete], resolved[toDelete+1:]...)
	}
	return resolved, err
}

// GetRecentlyResolvedInRange returns the most recently archived Incidents in the given range.
//
// d - The range in human units, e.g. "1w".
func (s *Store) GetRecentlyResolvedInRange(d string) ([]Incident, error) {
	duration, err := human.ParseDuration(d)
	if err != nil {
		return nil, fmt.Errorf("Invalid range: %s", err)
	}
	ts := time.Now().Add(-1 * duration).Unix()
	var resolved []Incident
	q := ds.NewQuery(ds.INCIDENT_AM).Filter("last_seen>", ts)
	keys, err := s.ds.GetAll(context.Background(), q, &resolved)
	for i, key := range keys {
		if resolved[i].Key == "" {
			resolved[i].Key = key.Encode()
		}
	}
	return resolved, err
}

// GetRecentlyResolvedInRangeWithID returns the most recently archived Incidents in the given range.
//
// d - The range in human units, e.g. "1w".
// id - The id of the incidents to return.
func (s *Store) GetRecentlyResolvedInRangeWithID(d, id string) ([]Incident, error) {
	duration, err := human.ParseDuration(d)
	if err != nil {
		return nil, fmt.Errorf("Invalid range: %s", err)
	}
	ts := time.Now().Add(-1 * duration).Unix()

	ancestor := ds.NewKey(ds.INCIDENT_ACTIVE_PARENT_AM)
	ancestor.Name = id
	var resolved []Incident
	q := ds.NewQuery(ds.INCIDENT_AM).Filter("last_seen>", ts).Ancestor(ancestor).Order("-last_seen")
	keys, err := s.ds.GetAll(context.Background(), q, &resolved)
	for i, key := range keys {
		if resolved[i].Key == "" {
			resolved[i].Key = key.Encode()
		}
	}
	return resolved, err
}
