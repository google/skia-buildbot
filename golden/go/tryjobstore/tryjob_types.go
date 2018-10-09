package tryjobstore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/paramtools"
)

// TODO(stephana): Move the UNKNOWN status to the first spot, so that we can
// move to a "higher" status easily.

// States of a tryjob in increasing order.
const (
	TRYJOB_SCHEDULED TryjobStatus = iota
	TRYJOB_RUNNING
	TRYJOB_COMPLETE
	TRYJOB_INGESTED
	TRYJOB_FAILED
	TRYJOB_UNKNOWN
)

// statusStringRepr maps from a TryjobStatus to a string.
var statusStringRepr = []string{
	"scheduled",
	"running",
	"complete",
	"ingested",
	"failed",
	"unknown",
}

// statusStringMap maps from status strings to instances of TryjobStatus
var statusStringMap = map[string]TryjobStatus{}

func init() {
	// Initialize the mapping between TryjobStatus and it's string representation.
	for idx, repr := range statusStringRepr {
		statusStringMap[repr] = TryjobStatus(idx)
	}
}

// TryjobStatus is an enum that captures the status of a tryjob.
type TryjobStatus int

// String returns a tryjob status as a string.
func (t TryjobStatus) String() string {
	return statusStringRepr[t]
}

// tryjobStatusFromString translates from a string representation of TryjobStatus
// to the integer representation.
func tryjobStatusFromString(statusStr string) TryjobStatus {
	if s, ok := statusStringMap[statusStr]; ok {
		return s
	}
	return TRYJOB_UNKNOWN
}

// Serialize TryjobStatus as string to JSON.
func (t TryjobStatus) MarshalJSON() ([]byte, error) {
	return []byte("\"" + t.String() + "\""), nil
}

// Deserialize a TryjobStatus from JSON.
func (t *TryjobStatus) UnmarshalJSON(data []byte) error {
	strStatus := strings.Trim(string(data), "\"")
	*t = tryjobStatusFromString(strStatus)
	return nil
}

// Reuse types from the buildbucket package.
type Parameters = buildbucket.Parameters
type Properties = buildbucket.Properties

// newerInterface is an internal interface that allows to define a temporal
// order for a type.
type newerInterface interface {
	newer(right interface{}) bool
}

// TODO(stephana): Fix the Committed field below to also be spelled correctly
// in the database.

// Issue captures information about a single code review issue.
type Issue struct {
	ID              int64             `json:"id"`
	Subject         string            `json:"subject"           datastore:",noindex"`
	Owner           string            `json:"owner"`
	Updated         time.Time         `json:"updated"`
	URL             string            `json:"url"               datastore:",noindex"`
	Status          string            `json:"status"`
	PatchsetDetails []*PatchsetDetail `json:"patchsets"         datastore:",noindex"`
	Committed       bool              `json:"committed"         datastore:"Commited"`
	QueryPatchsets  []int64           `json:"queryPatchsets"    datastore:"-"`
	CommentAdded    bool              `json:"-"                 datastore:",noindex"`

	clean bool
}

// MarshalJSON implements the Marshaller interface in encoding/json.
func (is *Issue) MarshalJSON() ([]byte, error) {
	// Create a wrapping struct around the Tryjob that produces the correct output.
	temp := struct {
		*wrapIssue
		Updated TimeJsonMs `json:"updated"` // Override the Updated field to produce a timestamp in MS.
	}{
		wrapIssue: (*wrapIssue)(is),
		Updated:   TimeJsonMs(is.Updated),
	}
	return json.Marshal(&temp)
}

// wrapIssue is a dummy type to avoid recursive call to Tryjob.MarshallJSON
type wrapIssue Issue

// HasPatchset returns true if the issue has the given patchset.
func (is *Issue) HasPatchset(patchsetID int64) bool {
	if is == nil {
		return false
	}
	found := is.FindPatchset(patchsetID)
	return found != nil
}

// FindPatchset returns the with the given id or nil if cannot be found
func (is *Issue) FindPatchset(id int64) *PatchsetDetail {
	for _, psd := range is.PatchsetDetails {
		if psd.ID == id {
			return psd
		}
	}
	return nil
}

// UpdatePatchset merges the given patchset information into this issue.
func (is *Issue) UpdatePatchsets(patchsets []*PatchsetDetail) {
	if is.PatchsetDetails == nil {
		is.PatchsetDetails = make([]*PatchsetDetail, 0, len(patchsets))
	}

	//	fmt.Printf("patchsets: %s", spew.Sdump(patchsets))
	for _, psd := range patchsets {
		// Only insert the patchset if it's not already there.
		if found := is.FindPatchset(psd.ID); found == nil {
			is.clean = false
			// insert patchset in the right spot.
			is.PatchsetDetails = append(is.PatchsetDetails, psd)
		}
	}
	if !is.clean {
		sort.Slice(is.PatchsetDetails, func(i, j int) bool { return is.PatchsetDetails[i].ID < is.PatchsetDetails[j].ID })
	}
}

// newer implements newerInterface.
func (is *Issue) newer(right interface{}) bool {
	return is.Updated.After(right.(*Issue).Updated)
}

// PatchsetDetails accumulates information about one patchset and the connected
// tryjobs.
type PatchsetDetail struct {
	ID           int64     `json:"id"`
	Commit       string    `json:"commit"`
	ParentCommit string    `json:"parentCommit"`
	Tryjobs      []*Tryjob `json:"tryjobs"   datastore:"-"`
}

// Tryjob captures information about a tryjob in BuildBucket.
type Tryjob struct {
	Key           *datastore.Key `json:"-" datastore:"__key__"` // Insert the key upon loading
	BuildBucketID int64          `json:"buildBucketID"`
	IssueID       int64          `json:"issueID"`
	PatchsetID    int64          `json:"patchsetID"`
	Builder       string         `json:"builder"`
	Status        TryjobStatus   `json:"status"`
	Updated       time.Time      `json:"-"`
	MasterCommit  string         `json:"masterCommit"`
}

// clone returns a shallow copy of the Tryjob instance
func (t *Tryjob) clone() *Tryjob {
	ret := &Tryjob{}
	*ret = *t
	return ret
}

type TimeJsonMs time.Time

func (j TimeJsonMs) MarshalJSON() ([]byte, error) {
	val := time.Time(j).UnixNano() / int64(time.Millisecond)
	return json.Marshal(val)
}

// String returns a string representation for the Tryjob
func (t *Tryjob) String() string {
	return fmt.Sprintf("%s - %d - %s", t.Builder, t.BuildBucketID, t.Status.String())
}

// newer implements newerInterface.
func (t *Tryjob) newer(r interface{}) bool {
	right := r.(*Tryjob)
	// A tryjob is newer if the status is updated or the BuildBucket record has been
	// updated.
	return t.Updated.Before(right.Updated) || (t.Status > right.Status)
}

// TryjobResult stores results. It is stored in the database as a child of
// a Tryjob entity.
type TryjobResult struct {
	TestName string              `datastore:"TestName,noindex"`
	Digest   string              `datastore:"Digest,noindex"`
	Params   paramtools.ParamSet `datastore:"-"`
}

const (
	tjrParamPrefix = "param."
)

// Save implements the datastore.PropertyLoadSaver interface.
func (t *TryjobResult) Save() ([]datastore.Property, error) {
	props, err := datastore.SaveStruct(t)
	if err != nil {
		return nil, err
	}

	// Make it large enough to hold the struct props and the parameters.
	ret := make([]datastore.Property, len(t.Params), len(props)+len(t.Params))
	idx := 0
	for param, value := range t.Params {
		ret[idx].Name = tjrParamPrefix + param
		ret[idx].Value = strToInterfaceSlice(value)
		ret[idx].NoIndex = true
		idx += 1
	}
	ret = append(ret, props...)
	return ret, nil
}

// Load implements the datastore.PropertyLoadSaver interface.
func (t *TryjobResult) Load(props []datastore.Property) error {
	nonParams := make([]datastore.Property, 0, 2)
	t.Params = make(paramtools.ParamSet, len(props)-2)
	for _, prop := range props {
		if strings.HasPrefix(prop.Name, tjrParamPrefix) {
			t.Params[strings.TrimPrefix(prop.Name, tjrParamPrefix)] = interfaceToStrSlice(prop.Value.([]interface{}))
		} else {
			nonParams = append(nonParams, prop)
		}
	}
	return datastore.LoadStruct(t, nonParams)
}

// strToInterfaceSlice copies a slice of string to a slice of interface{}.
func strToInterfaceSlice(inArr []string) []interface{} {
	ret := make([]interface{}, len(inArr), len(inArr))
	for idx, val := range inArr {
		ret[idx] = val
	}
	return ret
}

// interfaceToStrSlice copies a slice of interface{} to a string slice.
func interfaceToStrSlice(inArr []interface{}) []string {
	ret := make([]string, len(inArr), len(inArr))
	for idx, val := range inArr {
		ret[idx] = val.(string)
	}
	return ret
}

// IssueExpChange is used as the event type when tryjob related information changes
// and an event is sent to notify client.
type IssueExpChange struct {
	IssueID int64 `json:"issueID"`
}
