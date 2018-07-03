package incident

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/paramtools"
)

// Note is one note attached to an Incident.
type Note struct {
	Text   string `json:"text" datastore:"text"`
	Author string `json:"author" datastore:"author"`
	TS     uint64 `json:"ts" datastore:"ts"` // Time in seconds since the epoch.
}

// Well known keys for Incident.Params.
const (
	ALERT_NAME  = "alertname"
	CATEGORY    = "category"
	SEVERITY    = "severity"
	ID          = "id"
	ASSIGNED_TO = "assigned_to"
)

// Incident
//
// Will appear in either the list of active or silenced incidents,
// so we don't keep that as part of the state since it is derived info.
type Incident struct {
	// The ID is an md5 hash of all the Params. Stored at key ID+Start under a parent entity keyed at ID.
	//	ID         string            `json:"id"` - ID is stored in Params.
	// AssignedTo string            `json:"assigned_to"` // Email address. - AssignedTo is store in Params.
	Active       bool              `json:"active" datastore:"active"` // Or archived.
	Start        uint64            `json:"start" datastore:"start"`   // Time in seconds since the epoch.
	Finish       uint64            `json:"finish" datastore:"finish"` // Time in seconds since the epoch.
	Params       paramtools.Params `json:"params" datastore:"-"`
	SerialParams []string          `json:"serial_params" datastore:"serial_params,noindex"`
	Notes        []Note            `json:"notes" datastore:"notes,flatten"`
}

func FromAlert(m map[string]string) (*Incident, error) {
	if m[alerts.TYPE] == alerts.TYPE_HEALTHZ {
		return nil, fmt.Errorf("Healthz events should be ignored.")
	}
	id := []string{}
	p := paramtools.Params(m)
	keys := p.Keys()
	sort.Strings(keys)
	for _, key := range keys {
		id = append(id, key, p[key])
	}
	p[ID] = fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(id, ":"))))

	return &Incident{
		Active: true,
		Start:  time.Now().Unix(),
		Params: p,
		Notes:  []Note{},
	}, nil
}

func (in *Incident) Load(ps []datastore.Property) error {
	if err := datastore.LoadStruct(in, ps); err != nil {
		return err
	}
	num := len(in.SerialParams)
	if num%2 == 1 {
		return fmt.Errorf("Params were incorrectly serialized.")
	}
	in.Params = map[string]string{}
	for i := 0; i < num; i += 2 {
		in.Params[in.SerialParams[i]] = in.SerialParams[i+1]
	}
	return nil
}

func (in *Incident) Save() ([]datastore.Property, error) {
	if len(in.SerialParams) == 0 {
		for k, v := range in.Params {
			in.SerialParams = append(in.SerialParams, k, v)
		}
	}
	return datastore.SaveStruct(in)
}
