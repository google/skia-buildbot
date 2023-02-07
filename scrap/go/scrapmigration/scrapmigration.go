package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/scrap/go/scrap"
	"google.golang.org/api/iterator"
)

const (
	bucketName   = "skia-public-scrap-exchange"
	skslPrefix   = "scraps/sksl"
	maxScrapSize = 128 * 1024
)

// Return the full name of a SkSL scrap object given its |id| in string form.
func getSkSLIdObjName(id string) string {
	return fmt.Sprintf("%s/%s", skslPrefix, id)
}

// changeDistResourcePath will change a source path if it starts with "/dist/"
// to start with "/img/" and return a boolean to indicate if it was changed.
func changeDistResourcePath(p string) (bool, string) {
	if !strings.HasPrefix(p, "/dist/") {
		return false, p
	}
	return true, "/img/" + strings.TrimPrefix(p, "/dist/")
}

// readScrap reads a scrap, identified by |name|.
// |name| is the full name of the object - e.g. "scraps/sksl/<hash>".
func readScrap(ctx context.Context, bucket *storage.BucketHandle, name string) (scrap.ScrapBody, error) {
	obj := bucket.Object(name).ReadCompressed( /*compressed=*/ true)
	obr, err := obj.NewReader(ctx)
	if err != nil {
		return scrap.ScrapBody{}, skerr.Wrapf(err, "Obj %q", name)
	}
	defer obr.Close()

	gzr, err := gzip.NewReader(obr)
	if err != nil {
		return scrap.ScrapBody{}, skerr.Wrap(err)
	}
	defer gzr.Close()

	decoder := json.NewDecoder(gzr)
	decoder.UseNumber()
	var v scrap.ScrapBody
	err = decoder.Decode(&v)
	if err != nil {
		return scrap.ScrapBody{}, skerr.Wrap(err)
	}
	return v, nil
}

// updateScrap will write the scrap to cloud storage for an existing object.
// |name| is the full name of the object - e.g. "scraps/sksl/<hash>". This
// function does not support named scraps - e.g. "@scraps/sksl/@<some name>"
// scrap.CreateScrap generates the ID of the newly created scrap which is the
// SHA256 hash of the JSON encoded scrap.ScrapBody. This function *does not*
// change the scrap ID.
func updateScrap(ctx context.Context, bucket *storage.BucketHandle, name string, body scrap.ScrapBody) error {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return skerr.Wrapf(err, "Failed to JSON encode scrap.")
	}
	if b.Len() > maxScrapSize {
		return scrap.ErrInvalidScrapSize
	}
	encodedBody := b.Bytes()

	wc := bucket.Object(name).NewWriter(ctx)
	wc.ContentType = "application/json"
	wc.ContentEncoding = "gzip"

	zw := gzip.NewWriter(wc)
	_, err := zw.Write(encodedBody)
	if err != nil {
		return skerr.Wrapf(err, "Failed to write JSON body.")
	}
	if err = zw.Close(); err != nil {
		return skerr.Wrapf(err, "Failed to close gzip writer.")
	}
	if err = wc.Close(); err != nil {
		return skerr.Wrapf(err, "Failed to close object writer.")
	}
	return nil
}

// switchSkSLScrapDistToImg will change a SkSL scrap object's ImageURL
// /dist to /img. |name| refers to the full object name e.g. "scraps/sksl/<id>".
// Will return a boolean value indicating whether the object was updated in
// the GCS bucket.
func switchSkSLScrapDistToImg(ctx context.Context, bucket *storage.BucketHandle, name string) (bool, error) {
	body, err := readScrap(ctx, bucket, name)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if body.Type != scrap.SKSL {
		return false, skerr.Fmt("Object %s is in the %s with incorrect type", name, skslPrefix)
	}
	if body.SKSLMetaData == nil {
		// Some objects have no metadata value - not an error.
		return false, nil
	}
	changed, newImageURL := changeDistResourcePath(body.SKSLMetaData.ImageURL)
	if !changed {
		return false, nil
	}
	body.SKSLMetaData.ImageURL = newImageURL
	err = updateScrap(ctx, bucket, name, body)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	return true, nil
}

// switchSingleSkSLScrapDistToImg will change the |ImageURL| in a single
// SkSL object, identified by |id|, to refer to the /img/ dir instead of
// the /dist/ dir.
func switchSingleSkSLScrapDistToImg(bucketName, id string) (bool, error) {
	ctx := context.Background()
	gcsc, err := storage.NewClient(ctx)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	defer gcsc.Close()
	bucket := gcsc.Bucket(bucketName)
	return switchSkSLScrapDistToImg(ctx, bucket, getSkSLIdObjName(id))
}

// switchDefaultSkSLScrapDistToImg will ensure the defult scrap, i.e.
// `gs://skia-public-scrap-exchange/names/sksl/@default` has an ImageURL
// field that references /img instead of /dist.
func switchDefaultSkSLScrapDistToImg() error {
	// gsutil cat gs://skia-public-scrap-exchange/names/sksl/@default
	const defaultSkSLScrapID = "f1be7449cdd7b15ab39efa681f5191bbaa55b1c58f582cb16c55186dd95a24e0"
	changed, err := switchSingleSkSLScrapDistToImg(bucketName, defaultSkSLScrapID)
	if err != nil {
		return skerr.Wrap(err)
	}
	if changed {
		fmt.Println("The default SkSL object was updated")
	} else {
		fmt.Println("The default SkSL object was NOT updated")
	}
	return nil
}

// Update all SkSL scraps so that any ImageURL values refer to an image
// in /img/ instead of /dist/. This function will return on the first
// failure.
func switchAllSkSLScrapDistToImg() (numChanged, totalCount int, err error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, 0, skerr.Wrap(err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	it := bucket.Objects(ctx, &storage.Query{
		Prefix: skslPrefix,
	})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		totalCount++
		if err != nil {
			return 0, 0, skerr.Wrapf(err, "Bucket: %q, prefix: %q", bucketName, skslPrefix)
		}
		changed, err := switchSkSLScrapDistToImg(ctx, bucket, attrs.Name)
		if err != nil {
			return 0, 0, skerr.Wrap(err)
		}
		if changed {
			numChanged++
		}
	}
	return numChanged, totalCount, nil

}

func main() {
	err := switchDefaultSkSLScrapDistToImg()
	if err != nil {
		log.Fatalf("Error updating default object: %q", err)
	}

	numChanged, count, err := switchAllSkSLScrapDistToImg()
	if err != nil {
		log.Fatalf("Error updating all SkSL objects: %q", err)
	}
	fmt.Printf("Successfully updated %d objects out of %d SkSL scraps", numChanged, count)
}
