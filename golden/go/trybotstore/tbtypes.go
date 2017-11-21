package trybotstore

import (
	"fmt"
	"sort"
	"time"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/buildbucket"
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

// Issue captures information about a single code review issue.
type Issue struct {
	ID      int64     `json:"id"`
	Subject string    `json:"subject"`
	Owner   string    `json:"owner"`
	Updated time.Time `json:"updated"`
	URL     string    `json:"url"`
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

	fmt.Printf("patchsets: %s", spew.Sdump(patchsets))
	for _, psd := range patchsets {
		// Only insert the patchset if it's not already there.
		if found, idx := is.findPatchset(psd.ID); found == nil {
			// insert patchset in the right spot.
			is.PatchsetDetails = append(is.PatchsetDetails[:idx], append([]*PatchsetDetail{psd}, is.PatchsetDetails[idx:]...)...)
		}
	}
}

type PatchsetDetail struct {
	ID       int64     `json:"id"`
	Tryjobs  []*Tryjob `json:"tryjobs"`
	JobTotal int64     `json:"jobTotal"`
	JobDone  int64     `json:"jobDone"`
}

type Tryjob struct {
	Builder     string       `json:"builder"`
	Buildnumber int64        `json:"buildnumber"`
	Status      TryjobStatus `json:"status"`
}

func (t *Tryjob) String() string {
	return fmt.Sprintf("%s - %d - %s", t.Builder, t.Buildnumber, t.Status.String())
}

func (t *Tryjob) equal(right *Tryjob) bool {
	fmt.Printf("LEFT : %s", t.String())
	fmt.Printf("RIGHT: %s", t.String())
	return *t == *right
}

func (t *Tryjob) less(right *Tryjob) bool {
	return (t.Builder < right.Builder) ||
		(t.Buildnumber < right.Buildnumber) ||
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
