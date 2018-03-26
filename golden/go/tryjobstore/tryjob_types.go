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

// TryjobStatus is an enum that captures the status of a tryjob.
type TryjobStatus int

// String returns a tryjob status as a string.
func (t TryjobStatus) String() string {
	return statusStringRepr[t]
}

// Serialize TryjobStatus as string to JSON.
// Note: We only output JSON so we omit the UnmarshalJSON function.
func (t TryjobStatus) MarshalJSON() ([]byte, error) {
	return []byte("\"" + t.String() + "\""), nil
}

// Reuse types from the buildbucket package.
type Parameters = buildbucket.Parameters
type Properties = buildbucket.Properties

// newerInterface is an internal interface that allows to define a temporal
// order for a type.
type newerInterface interface {
	newer(right interface{}) bool
}

// Issue captures information about a single code review issue.
type Issue struct {
	ID              int64             `json:"id"`
	Subject         string            `json:"subject"`
	Owner           string            `json:"owner"`
	Updated         time.Time         `json:"updated"`
	URL             string            `json:"url"`
	Status          string            `json:"status"`
	PatchsetDetails []*PatchsetDetail `json:"patchsets"`
	Commited        bool              `json:"commited"`
	QueryPatchsets  []int64           `json:"queryPatchsets"    datastore:"-"`

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
	found := is.findPatchset(patchsetID)
	return found != nil
}

// findPatchset returns the patchset for the issue.
func (is *Issue) findPatchset(id int64) *PatchsetDetail {
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
		if found := is.findPatchset(psd.ID); found == nil {
			is.clean = false
			// insert patchset in the right spot.
			is.PatchsetDetails = append(is.PatchsetDetails, psd)
		}
	}
	if !is.clean {
		sort.Slice(is.PatchsetDetails, func(i, j int) bool { return is.PatchsetDetails[i].ID < is.PatchsetDetails[j].ID })
	}
}

// newer implments newerInterface.
func (is *Issue) newer(right interface{}) bool {
	return is.Updated.After(right.(*Issue).Updated)
}

// PatchsetDetails accumulates information about one patchset and the connected
// tryjobs.
type PatchsetDetail struct {
	ID      int64     `json:"id"`
	Tryjobs []*Tryjob `json:"tryjobs"   datastore:"-"`
}

// Tryjob captures information about a tryjob in BuildBucket.
type Tryjob struct {
	BuildBucketID int64        `json:"buildBucketID"`
	IssueID       int64        `json:"issueID"`
	PatchsetID    int64        `json:"patchsetID"`
	Builder       string       `json:"builder"`
	Status        TryjobStatus `json:"status"`
	Updated       time.Time    `json:"-"`
	MasterCommit  string       `json:"masterCommit"`
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

// newer implments newerInterface.
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

// ExpChange is used to store an expectation change in the database. Each
// expecation change is an atomic change to expectations for an issue.
// The actualy expecations are captured in instances of TestDigestExp.
type ExpChange struct {
	ChangeID     *datastore.Key `datastore:"__key__"`
	IssueID      int64
	UserID       string
	TimeStamp    int64
	Count        int64
	UndoChangeID int64
	OK           bool
}

// TestDigestExp is used to store expectations for an issue in the database.
// Each entity is a child of instance of ExpChange. It captures the expectation
// of one Test/Digest pair.
type TestDigestExp struct {
	Name   string
	Digest string
	Label  string
}

// IssueExpChange is used as the event type when tryjob related information changes
// and an event is sent to notify client.
type IssueExpChange struct {
	IssueID int64 `json:"issueID"`
}
