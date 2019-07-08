package ds_tryjobstore

import "go.skia.org/infra/golden/go/tryjobstore"

// newerInterface is an internal interface that allows to define a temporal
// order for a type.
type newerInterface interface {
	newer(right interface{}) bool
}

type newerIssue tryjobstore.Issue

// newer implements newerInterface.
func (is *newerIssue) newer(r interface{}) bool {
	right := r.(*newerIssue)
	return is.Updated.After(right.Updated)
}

type newerTryjob tryjobstore.Tryjob

// newer implements newerInterface.
func (t *newerTryjob) newer(r interface{}) bool {
	right := r.(*newerTryjob)
	// A tryjob is newer if the status is updated or the BuildBucket record has been
	// updated.
	return t.Updated.Before(right.Updated) || (t.Status > right.Status)
}
