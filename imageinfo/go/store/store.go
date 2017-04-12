// store contains a wrapper around Google Storage for imageinfo uploaded images.
package store

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	IMAGEINFO_STORAGE_BUCKET = "skia-imageinfo"

	// UPLOADS_DIR is the directory under IMAGEINFO_STORAGE_BUCKET where the uploaded
	// images are stored.
	UPLOADS_DIR = "uploads"

	LRU_CACHE_SIZE = 10000

	// USER_METADATA is the key used to store the user who uploaded the image.
	USER_METADATA = "user"
)

// Store is used to read and write uploaded images to and from Google Storage.
type Store struct {
	bucket *storage.BucketHandle

	// cache is an in-memory cache of PNGs, where the keys are <hash>.
	cache *lru.Cache
}

// New creates a new Store.
func New() (*Store, error) {
	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up client OAuth: %s", err)
	}
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}
	return &Store{
		bucket: storageClient.Bucket(IMAGEINFO_STORAGE_BUCKET),
		cache:  lru.New(LRU_CACHE_SIZE),
	}, nil
}

// Put writes a new image to Google Storage.
//
//  b    - The bytes of the image file.
//  hash - The md5 hash of the image contents, used as the name of the file.
//  contentType - The content type of the image.
//  user - The id of the user that uploaded the file.
func (s *Store) Put(b []byte, hash, contentType, user string) error {
	path := strings.Join([]string{UPLOADS_DIR, hash}, "/")
	w := s.bucket.Object(path).NewWriter(context.Background())
	w.ObjectAttrs.ContentEncoding = contentType
	w.ObjectAttrs.Metadata = map[string]string{
		USER_METADATA: user,
	}
	defer util.Close(w)
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("Failed to save image: %s", err)
	}
	s.cache.Add(hash, b)
	return nil
}

// Get returns the bytes and the content type of the image identified by hash.
//
// If there is no image for the given hash then Get passes through the
// storage.ErrObjectNotExist as the returned error.
func (s *Store) Get(hash string) ([]byte, string, error) {
	if c, ok := s.cache.Get(hash); ok {
		if b, ok := c.([]byte); ok {
			sklog.Infof("Cache hit: %s", hash)
			return b, "", nil
		}
	}
	o := s.bucket.Object(fmt.Sprintf("%s/%s", UPLOADS_DIR, hash))
	r, err := o.NewReader(context.Background())
	if err != nil {
		return nil, "", fmt.Errorf("Failed to open image file: %s", err)
	}
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	if err == storage.ErrObjectNotExist {
		return nil, "", err
	} else if err != nil {
		return nil, "", fmt.Errorf("Failed to read image file: %s", err)
	}
	s.cache.Add(hash, b)
	return b, r.ContentType(), nil
}
