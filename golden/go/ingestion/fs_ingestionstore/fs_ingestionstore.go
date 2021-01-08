// Package fs_ingestionstore supplies a Firestore backed implementation of
// ingestion.IngestionStore. See FIRESTORE.md for more.
package fs_ingestionstore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/ingestion"
)

const (
	// These are the collections in Firestore.
	ingestionCollection = "ingestionstore_entries"

	// These are the fields we query by
	fileHashField = "filehash"

	maxAttempts = 10

	maxDuration = time.Minute

	separator = "|"
)

// Store implements the IngestionStore interface backed by Firestore.
type Store struct {
	client *ifirestore.Client
}

// ingestedEntry is the primary document type that keeps track if a
// file with the given md5 hash has been ingested
type ingestedEntry struct {
	// TODO(kjlubick): This could be simplified by having FileHash be the id - no .Where() needed
	FileHash string `firestore:"filehash"`
}

// combine creates a key for a file/md5 combination.
func combine(fileName, md5 string) string {
	return fileName + separator + md5
}

// New creates a new ingestionstore backed by FireStore
func New(client *ifirestore.Client) *Store {
	return &Store{
		client: client,
	}
}

// SetIngested fulfills the IngestionStore interface
func (s *Store) SetIngested(ctx context.Context, fileName, md5 string, _ time.Time) error {
	defer metrics2.FuncTimer().Stop()
	ir := s.client.Collection(ingestionCollection).NewDoc()
	record := ingestedEntry{
		FileHash: combine(fileName, md5),
	}
	_, err := s.client.Set(ctx, ir, record, maxAttempts, maxDuration)
	if err != nil {
		return skerr.Wrapf(err, "writing %s:%s to ingestionstore", fileName, md5)
	}
	return nil
}

// WasIngested fulfills the IngestionStore interface
func (s *Store) WasIngested(ctx context.Context, fileName, md5 string) (bool, error) {
	defer metrics2.FuncTimer().Stop()
	c := combine(fileName, md5)
	q := s.client.Collection(ingestionCollection).Where(fileHashField, "==", c).Limit(1)
	found := false
	err := s.client.IterDocs(ctx, "contains", c, q, maxAttempts, maxDuration, func(doc *firestore.DocumentSnapshot) error {
		if doc != nil {
			found = true
		}
		return nil
	})
	if err != nil {
		return false, skerr.Wrapf(err, "reading %s:%s in firestore", fileName, md5)
	}
	return found, nil
}

// Make sure Store fulfills IngestionStore
var _ ingestion.IngestionStore = (*Store)(nil)
