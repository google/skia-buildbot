package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

// GCSClientOptions is used to define input parameters to the GCSClient.
type GCSClientOptions struct {
	// HashesGSPath is the bucket and path for storing the list of known digests.
	HashesGSPath string

	// BaselineGSPath is the bucket and path for storing the baseline information.
	// This is considered to be a directory and will be used as such.
	BaselineGSPath string
}

// GCSClient provides an abstraction around read/writes to Google storage.
type GCSClient interface {
	// WriteKnownDigests writes the given list of digests to GCS as newline separated strings.
	WriteKnownDigests(digests types.DigestSlice) error

	// WriteBaseline writes the given baseline to GCS. It returns the path of the
	// written file in GCS (prefixed with 'gs://').
	WriteBaseline(b *baseline.Baseline) (string, error)

	// ReadBaseline returns the baseline for the given issue from GCS.
	ReadBaseline(commitHash string, issueID int64) (*baseline.Baseline, error)

	// LoadKnownDigests loads the digests that have previously been written
	// to GS via WriteKnownDigests. The digests should be copied to the
	// provided writer 'w'.
	LoadKnownDigests(w io.Writer) error

	// RemoveForTestingOnly removes the given file. Should only be used for testing.
	RemoveForTestingOnly(targetPath string) error

	// Returns the options that were used to initialize the client
	Options() GCSClientOptions
}

// ClientImpl implements the GCSClient interface.
type ClientImpl struct {
	storageClient *gstorage.Client
	options       GCSClientOptions
}

// NewGCSClient creates a new instance of ClientImpl. The various
// output paths are set in GCSClientOptions.
func NewGCSClient(client *http.Client, options GCSClientOptions) (*ClientImpl, error) {
	storageClient, err := gstorage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &ClientImpl{
		storageClient: storageClient,
		options:       options,
	}, nil
}

// Options implements the GCSClient interface.
func (g *ClientImpl) Options() GCSClientOptions {
	return g.options
}

// WriteKnownDigests fulfills the GCSClient interface.
func (g *ClientImpl) WriteKnownDigests(digests types.DigestSlice) error {
	writeFn := func(w *gstorage.Writer) error {
		for _, digest := range digests {
			if _, err := w.Write([]byte(digest + "\n")); err != nil {
				return fmt.Errorf("Error writing digests: %s", err)
			}
		}
		return nil
	}

	return g.writeToPath(g.options.HashesGSPath, "text/plain", writeFn)
}

// ReadBaseline fulfills the GCSClient interface.
func (g *ClientImpl) WriteBaseline(b *baseline.Baseline) (string, error) {
	writeFn := func(w *gstorage.Writer) error {
		if err := json.NewEncoder(w).Encode(b); err != nil {
			return fmt.Errorf("Error encoding baseline to JSON: %s", err)
		}
		return nil
	}

	// We need a valid end commit or issue.
	if b.EndCommit == nil && b.Issue <= 0 {
		return "", skerr.Fmt("Received empty end commit and no issue. Cannot write baseline")
	}

	hash := ""
	if b.EndCommit != nil {
		hash = b.EndCommit.Hash
	}
	outPath := g.getBaselinePath(hash, b.Issue)
	return "gs://" + outPath, g.writeToPath(outPath, "application/json", writeFn)
}

// ReadBaseline fulfills the GCSClient interface.
func (g *ClientImpl) ReadBaseline(commitHash string, issueID int64) (*baseline.Baseline, error) {
	baselinePath := g.getBaselinePath(commitHash, issueID)
	bucketName, storagePath := gcs.SplitGSPath(baselinePath)

	ctx := context.Background()
	target := g.storageClient.Bucket(bucketName).Object(storagePath)

	_, err := target.Attrs(ctx)
	if err != nil {
		// If the item doesn't exist we return nil
		if err == gstorage.ErrObjectNotExist {
			return nil, nil
		}
		return nil, sklog.FmtErrorf("Error fetching attributes of baseline file: %s", err)
	}

	reader, err := target.NewReader(ctx)
	if err != nil {
		return nil, sklog.FmtErrorf("Error getting reader for baseline file: %s", err)
	}
	defer util.Close(reader)

	ret := &baseline.Baseline{}
	if err := json.NewDecoder(reader).Decode(ret); err != nil {
		return nil, sklog.FmtErrorf("Error decoding baseline file: %s", err)
	}
	return ret, nil
}

// getBaselinePath returns the baseline path in GCS for the given issueID.
// If issueID <= 0 it returns the path for the master baseline.
func (g *ClientImpl) getBaselinePath(commitHash string, issueID int64) string {
	// Change the output file based on whether it's the master branch or a Gerrit issue.
	var outPath string
	if issueID > baseline.MasterBranch {
		outPath = fmt.Sprintf("issue_%d.json", issueID)
	} else if commitHash != "" {
		outPath = fmt.Sprintf("master_%s.json", commitHash)
	} else {
		outPath = "master.json"
	}
	return g.options.BaselineGSPath + "/" + outPath
}

// LoadKnownDigests fulfills the GCSClient interface.
func (g *ClientImpl) LoadKnownDigests(w io.Writer) error {
	bucketName, storagePath := gcs.SplitGSPath(g.options.HashesGSPath)

	ctx := context.Background()
	target := g.storageClient.Bucket(bucketName).Object(storagePath)

	// If the item doesn't exist this will return gstorage.ErrObjectNotExist
	_, err := target.Attrs(ctx)
	if err != nil {
		// We simply assume an empty hashes file if the object was not found.
		if err == gstorage.ErrObjectNotExist {
			return nil
		}
		return err
	}

	// Copy the content to the output writer.
	reader, err := target.NewReader(ctx)
	if err != nil {
		return err
	}
	defer util.Close(reader)

	_, err = io.Copy(w, reader)
	return err
}

// RemoveForTestingOnly fulfills the GCSClient interface.
func (g *ClientImpl) RemoveForTestingOnly(targetPath string) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)
	target := g.storageClient.Bucket(bucketName).Object(storagePath)
	return target.Delete(context.Background())
}

// writeToPath is a generic function that allows to write data to the given
// target path in GCS. The actual writing is done in the passed write function.
func (g *ClientImpl) writeToPath(targetPath, contentType string, wrtFn func(w *gstorage.Writer) error) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)

	// Only write the known digests if a target path was given.
	if (bucketName == "") || (storagePath == "") {
		return nil
	}

	ctx := context.Background()
	target := g.storageClient.Bucket(bucketName).Object(storagePath)
	writer := target.NewWriter(ctx)
	writer.ObjectAttrs.ContentType = contentType
	writer.ObjectAttrs.ACL = []gstorage.ACLRule{{Entity: gstorage.AllUsers, Role: gstorage.RoleReader}}
	defer util.Close(writer)

	// Write the actual data.
	if err := wrtFn(writer); err != nil {
		return err
	}

	return nil
}

// Ensure ClientImpl fulfills the GCSClient interface.
var _ GCSClient = (*ClientImpl)(nil)
