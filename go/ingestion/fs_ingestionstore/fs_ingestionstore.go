// This package supplies a Firestore backed implementation of
// ingestion.IngestionStore. See FIRESTORE.md for more.
package fs_ingestionstore

import (
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/skerr"
)

const (
	// These are the collections in Firestore.
	ingestionCollection = "ingestionstore_entries"

	// These are the columns we query by
	fileNameCol = "filename"
	md5Col      = "md5hash"

	maxAttempts = 10

	maxDuration = time.Minute
)

// Store implements the IngestionStore interface backed by Firestore.
type Store struct {
	client *ifirestore.Client
}

// ingestedEntry is the primary document type that keeps track if a
// file with the given md5 hash has been ingested
type ingestedEntry struct {
	FileName string `firestore:"filename"`
	MD5Hash  string `firestore:"md5hash"`
}

func New(client *ifirestore.Client) *Store {
	return &Store{
		client: client,
	}
}

// SetResultFileHash fulfills the IngestionStore interface
func (s *Store) SetResultFileHash(fileName, md5 string) error {
	ir := s.client.Collection(ingestionCollection).NewDoc()
	record := ingestedEntry{
		FileName: fileName,
		MD5Hash:  md5,
	}
	_, err := s.client.Set(ir, record, maxAttempts, maxDuration)
	if err != nil {
		return skerr.Fmt("could not write %s:%s to ingeststore: %s", fileName, md5, err)
	}
	return nil
}

// ContainsResultFileHash fulfills the IngestionStore interface
func (s *Store) ContainsResultFileHash(fileName, md5 string) (bool, error) {
	q := s.client.Collection(ingestionCollection).Where(fileNameCol, "==", fileName)
	q = q.Where(md5Col, "==", md5).Limit(1)
	found := false
	err := s.client.IterDocs("contains", fileName+"|"+md5, q, maxAttempts, maxDuration, func(doc *firestore.DocumentSnapshot) error {
		if doc != nil {
			found = true
		}
		return nil
	})
	if err != nil {
		return false, skerr.Fmt("could not locate %s:%s in firestore: %s", fileName, md5, err)
	}
	return found, nil
}

// Make sure Store fulfills IngestionStore
var _ ingestion.IngestionStore = (*Store)(nil)
