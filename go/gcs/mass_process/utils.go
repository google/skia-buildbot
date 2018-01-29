package mass_process

/*
   Utilities for processing many objects in Google Storage.
*/

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/util"
)

const (
	// Number of concurrent goroutines to use for processing objects in GS.
	// Consider that this number should not only be a function of number of
	// cores but also network bandwidth and QPS limits to GS.
	NUM_WORKERS = 20
)

// Replace the inPrefix with the outPrefix in the given input path to produce an
// output path.
func GetOutputPath(path, inPrefix, outPrefix string) string {
	return outPrefix + strings.TrimPrefix(path, inPrefix)
}

// Read the given object from Google Storage.
func ReadObj(o *storage.ObjectHandle) ([]byte, error) {
	r, err := o.NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer util.Close(r)
	return ioutil.ReadAll(r)
}

// Write the given content to the given object in Google Storage.
func WriteObj(o *storage.ObjectHandle, content []byte) (err error) {
	w := o.NewWriter(context.Background())
	w.ObjectAttrs.ContentEncoding = "gzip"
	if err := util.WithGzipWriter(w, func(w io.Writer) error {
		_, err := w.Write(content)
		return err
	}); err != nil {
		_ = w.CloseWithError(err) // Always returns nil, according to docs.
		return err
	}
	return w.Close()
}

// Search for objects with the given prefix. Object paths are passed onto the
// given channel, which is closed once all objects have been processed.
func SearchObj(b *storage.BucketHandle, prefix string, results chan<- string, max int) error {
	q := &storage.Query{
		Prefix: prefix,
	}
	it := b.Objects(context.Background(), q)
	n := 0
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			close(results)
			return err
		}
		results <- obj.Name

		n++
		if max > 0 && n >= max {
			break
		}
	}
	close(results)
	return nil
}

// Process all objects with the given prefix.
func ProcessMany(b *storage.BucketHandle, prefix string, fn func(string) error, max int) error {
	queue := make(chan string)
	var wg sync.WaitGroup
	var mtx sync.Mutex
	failed := map[string]error{}
	for i := 0; i < NUM_WORKERS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range queue {
				if err := fn(path); err != nil {
					mtx.Lock()
					failed[path] = err
					mtx.Unlock()
				}
			}
		}()
	}

	if err := SearchObj(b, prefix, queue, max); err != nil {
		return err
	}
	wg.Wait()
	if len(failed) > 0 {
		errMsg := fmt.Sprintf("Failed to delete %d objects:\n", len(failed))
		for k, v := range failed {
			errMsg += fmt.Sprintf("  %s: %s", k, v)
		}
		return errors.New(errMsg)
	}
	return nil
}

// Delete all objects with the given prefix.
func Delete(b *storage.BucketHandle, prefix string) error {
	return ProcessMany(b, prefix, func(path string) error {
		return b.Object(path).Delete(context.Background())
	}, -1)
}
