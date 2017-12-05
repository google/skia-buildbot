package tryjobstore

import (
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/paramtools"
)

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
	ID      int64     `json:"id"`
	Subject string    `json:"subject"`
	Owner   string    `json:"owner"`
	Updated time.Time `json:"updated"`
	URL     string    `json:"url"`
	Status  string    `json:"status"`
}

// IssueDetails extends Issue with information about Patchsets, which in turn
// contain information about tryjobs for patchsets.
type IssueDetails struct {
	*Issue
	PatchsetDetails []*PatchsetDetail `json:"-"`
	clean           bool
}

// HasPatchset returns true if the issue has the given patchset.
func (is *IssueDetails) HasPatchset(patchsetID int64) bool {
	if is == nil {
		return false
	}
	found, _ := is.findPatchset(patchsetID)
	return found != nil
}

// findPatchset returns the patchset for the issue.
func (is *IssueDetails) findPatchset(id int64) (*PatchsetDetail, int) {
	foundIdx := sort.Search(len(is.PatchsetDetails), func(i int) bool { return is.PatchsetDetails[i].ID >= id })
	if (foundIdx == len(is.PatchsetDetails)) || (is.PatchsetDetails[foundIdx].ID > id) {
		return nil, foundIdx
	}
	return is.PatchsetDetails[foundIdx], foundIdx
}

// UpdatePatchset merges the given patchset information into this issue.
func (is *IssueDetails) UpdatePatchsets(patchsets []*PatchsetDetail) {
	if is.PatchsetDetails == nil {
		is.PatchsetDetails = make([]*PatchsetDetail, 0, len(patchsets))
	}

	//	fmt.Printf("patchsets: %s", spew.Sdump(patchsets))
	for _, psd := range patchsets {
		// Only insert the patchset if it's not already there.
		if found, idx := is.findPatchset(psd.ID); found == nil {
			is.clean = false
			// insert patchset in the right spot.
			is.PatchsetDetails = append(is.PatchsetDetails[:idx], append([]*PatchsetDetail{psd}, is.PatchsetDetails[idx:]...)...)
		}
	}
}

// newer implments newerInterface.
func (is *IssueDetails) newer(right interface{}) bool {
	return is.Updated.Before(right.(*IssueDetails).Updated)
}

// PatchsetDetails accumulates information about one patchset and the connected
// tryjobs.
type PatchsetDetail struct {
	ID       int64     `json:"id"`
	Tryjobs  []*Tryjob `json:"tryjobs"   datastore:"-"`
	JobTotal int64     `json:"jobTotal"  datastore:"-"`
	JobDone  int64     `json:"jobDone"   datastore:"-"`
}

// Tryjob captures information about a tryjob in BuildBucket.
type Tryjob struct {
	BuildBucketID int64        `json:"buildBucketID"`
	IssueID       int64        `json:"issueID"`
	PatchsetID    int64        `json:"patchsetID"`
	Builder       string       `json:"builder"`
	Status        TryjobStatus `json:"status"`
}

// String returns a string representation for the Tryjob
func (t *Tryjob) String() string {
	return fmt.Sprintf("%s - %d - %s", t.Builder, t.BuildBucketID, t.Status.String())
}

// newer implments newerInterface.
func (t *Tryjob) newer(r interface{}) bool {
	right := r.(*Tryjob)
	return (t.Builder < right.Builder) ||
		(t.BuildBucketID < right.BuildBucketID) ||
		(t.Status < right.Status)
}

// TryjobResult stores results. It is stored in the database as a child of
// a Tryjob entity.
type TryjobResult struct {
	Digest string // Key
	Params paramtools.ParamSet
}

// Save implements the datastore.PropertyLoadSaver interface.
func (t *TryjobResult) Save() ([]datastore.Property, error) {
	ret := make([]datastore.Property, len(t.Params), len(t.Params))
	idx := 0
	for param, value := range t.Params {
		ret[idx].Name = param
		ret[idx].Value = strToInterfaceSlice(value)
		idx += 1
	}
	return ret, nil
}

// Load implements the datastore.PropertyLoadSaver interface.
func (t *TryjobResult) Load(props []datastore.Property) error {
	t.Params = make(paramtools.ParamSet, len(props))
	for _, prop := range props {
		t.Params[prop.Name] = interfaceToStrSlice(prop.Value.([]interface{}))
	}
	return nil
}

// LoadKey implements the datastore.KeyLoader interface.
func (t *TryjobResult) LoadKey(k *datastore.Key) error {
	t.Digest = k.Name
	return nil
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
