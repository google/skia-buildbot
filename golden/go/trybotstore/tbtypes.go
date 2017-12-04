package trybotstore

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

var statusStringRepr = []string{
	"scheduled",
	"running",
	"complete",
	"ingested",
	"failed",
	"unknown",
}

type TryjobStatus int

func (t TryjobStatus) String() string {
	return statusStringRepr[t]
}

type Parameters = buildbucket.Parameters
type Properties = buildbucket.Properties

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

// IssueDetails extends issue with the commit ideas for the issue.
type IssueDetails struct {
	*Issue
	PatchsetDetails []*PatchsetDetail `json:"-"`
	clean           bool
}

func (is *IssueDetails) HasPatchset(patchsetID int64) bool {
	if is == nil {
		return false
	}
	found, _ := is.findPatchset(patchsetID)
	return found != nil
}

func (is *IssueDetails) findPatchset(id int64) (*PatchsetDetail, int) {
	foundIdx := sort.Search(len(is.PatchsetDetails), func(i int) bool { return is.PatchsetDetails[i].ID >= id })
	if (foundIdx == len(is.PatchsetDetails)) || (is.PatchsetDetails[foundIdx].ID > id) {
		return nil, foundIdx
	}
	return is.PatchsetDetails[foundIdx], foundIdx
}

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

func (is *IssueDetails) newer(right interface{}) bool {
	return is.Updated.Before(right.(*IssueDetails).Updated)
}

func (is *IssueDetails) equals(right *IssueDetails) bool {
	if *is.Issue != *right.Issue {
		return false
	}

	if len(is.PatchsetDetails) != len(right.PatchsetDetails) {
		return false
	}

	for _, psLeft := range is.PatchsetDetails {
		if psLeft.ID != right.ID {
			return false
		}
	}

	return true
}

type PatchsetDetail struct {
	ID       int64     `json:"id"`
	Tryjobs  []*Tryjob `json:"tryjobs"   datastore:"-"`
	JobTotal int64     `json:"jobTotal"  datastore:"-"`
	JobDone  int64     `json:"jobDone"   datastore:"-"`
}

type Tryjob struct {
	BuildBucketID int64        `json:"buildBucketID"`
	IssueID       int64        `json:"issueID"`
	PatchsetID    int64        `json:"patchsetID"`
	Builder       string       `json:"builder"`
	Status        TryjobStatus `json:"status"`
}

func (t *Tryjob) String() string {
	return fmt.Sprintf("%s - %d - %s", t.Builder, t.BuildBucketID, t.Status.String())
}

// func (t *Tryjob) equalx(right *Tryjob) bool {
// 	// fmt.Printf("LEFT : %s", t.String())
// 	// fmt.Printf("RIGHT: %s", t.String())
// 	return *t == *right
// }

// func (t *Tryjob) lessx(right *Tryjob) bool {
// 	return (t.Builder < right.Builder) ||
// 		(t.Buildnumber < right.Buildnumber) ||
// 		(t.Status < right.Status)
// }

func (t *Tryjob) newer(r interface{}) bool {
	right := r.(*Tryjob)
	return (t.Builder < right.Builder) ||
		(t.BuildBucketID < right.BuildBucketID) ||
		(t.Status < right.Status)
}

// func (i *IssueDetails) updatePatchsetDetailsx(newDetails map[int64]*PatchsetDetail) {
// 	// if i.PatchsetDetails == nil {
// 	// 	i.PatchsetDetails = newDetails
// 	// 	return
// 	// }

// 	// // Copy the patchsets over that are not already in the current patchset details.
// 	// for id, detail := range newDetails {
// 	// 	if _, ok := i.PatchsetDetails[id]; !ok {
// 	// 		i.PatchsetDetails[id] = detail
// 	// 	}
// 	// }
// }

// func (i *IssueDetails) addTryjobx(patchsetID int64, tryjob *Tryjob) error {
// 	// detail, ok := i.PatchsetDetails[patchsetID]
// 	// if !ok {
// 	// 	return fmt.Errorf("Unable to find patchset %d in issue %d", patchsetID, i.ID)
// 	// }

// 	// sklog.Infof("Adding: %s", tryjob.String())

// 	// done := false
// 	// for idx, current := range detail.Tryjobs {
// 	// 	// We found it and they are the same.
// 	// 	if tryjob.equal(tryjob) {
// 	// 		return nil
// 	// 	}

// 	// 	if tryjob.less(current) {
// 	// 		// Insert at position idx.
// 	// 		detail.Tryjobs = append(detail.Tryjobs[:idx], append([]*Tryjob{tryjob}, detail.Tryjobs[idx:]...)...)
// 	// 		done = true
// 	// 		break
// 	// 	}
// 	// }

// 	// if !done {
// 	// 	detail.Tryjobs = append(detail.Tryjobs, tryjob)
// 	// }

// 	// return nil
// 	return nil
// }

type TryjobResult struct {
	Digest string // Key
	Params paramtools.ParamSet
}

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

func (t *TryjobResult) Load(props []datastore.Property) error {
	t.Params = make(paramtools.ParamSet, len(props))
	for _, prop := range props {
		t.Params[prop.Name] = interfaceToStrSlice(prop.Value.([]interface{}))
	}
	return nil
}

func (t *TryjobResult) LoadKey(k *datastore.Key) error {
	t.Digest = k.Name
	return nil
}

func strToInterfaceSlice(inArr []string) []interface{} {
	ret := make([]interface{}, len(inArr), len(inArr))
	for idx, val := range inArr {
		ret[idx] = val
	}
	return ret
}

func interfaceToStrSlice(inArr []interface{}) []string {
	ret := make([]string, len(inArr), len(inArr))
	for idx, val := range inArr {
		ret[idx] = val.(string)
	}
	return ret
}

type ExpChange struct {
	ChangeID     *datastore.Key `datastore:"__key__"`
	IssueID      int64
	UserID       string
	TimeStamp    int64
	Count        int64
	UndoChangeID int64
}

type TestDigestExp struct {
	Name   string
	Digest string
	Label  string
}
