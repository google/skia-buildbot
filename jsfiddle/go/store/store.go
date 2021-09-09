// Stores and retrieves jsfiddles in Google Storage.
package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	JSFIDDLE_STORAGE_BUCKET = "skia-jsfiddle"
)

// Store is used to read and write user code and media to and from Google
// Storage.
type Store struct {
	bucket *storage.BucketHandle
}

// New creates a new Store.
//
// local - True if running locally.
func New(local bool) (*Store, error) {
	ts, err := auth.NewDefaultTokenSource(local, auth.ScopeReadWrite)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up client OAuth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}
	return &Store{
		bucket: storageClient.Bucket(JSFIDDLE_STORAGE_BUCKET),
	}, nil
}

// PutCode writes the code to Google Storage.
// Returns the fiddleHash.
func (s *Store) PutCode(code, fiddleType string) (string, error) {
	hash := computeHash(code)

	path := strings.Join([]string{fiddleType, hash, "draw.js"}, "/")
	w := s.bucket.Object(path).NewWriter(context.Background())
	defer util.Close(w)
	w.ObjectAttrs.ContentEncoding = "text/plain"

	if n, err := w.Write([]byte(code)); err != nil {
		return "", fmt.Errorf("There was a problem storing the code. Uploaded %d bytes: %s", n, err)
	}
	return hash, nil
}

// PutCode writes the code to Google Storage.
// Returns the fiddleHash.
func (s *Store) GetCode(hash, fiddleType string) (string, error) {
	path := strings.Join([]string{fiddleType, hash, "draw.js"}, "/")
	o := s.bucket.Object(path)
	r, err := o.NewReader(context.Background())
	if err != nil {
		return "", fmt.Errorf("Failed to open source file for %s: %s", hash, err)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("Failed to read source file for %s: %s", hash, err)
	}
	return string(b), nil
}

func computeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", sum)
}
