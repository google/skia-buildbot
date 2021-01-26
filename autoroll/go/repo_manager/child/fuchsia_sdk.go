package child

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
	"google.golang.org/api/option"
)

const (
	// TODO(borenet): These should probably all move to the config file.

	// FuchsiaSDKGSBucket is the Google Cloud Storage bucket in which the
	// Fuchsia SDK tarballs are stored.
	FuchsiaSDKGSBucket = "fuchsia"

	// FuchsiaSDKGSLatestPathLinux is the path in GCS of the file containing the
	// ID of the latest Linux Fuchsia SDK.
	FuchsiaSDKGSLatestPathLinux = "development/LATEST_LINUX"

	// FuchsiaSDKGSLatestPathMac is the path in GCS of the file containing the
	// ID of the latest Mac Fuchsia SDK.
	FuchsiaSDKGSLatestPathMac = "development/LATEST_MAC"

	// FuchsiaSDKGSTarballPathLinux is a template for the path in GCS of the
	// Linux Fuchsia SDK tarball itself, keyed by version ID.
	FuchsiaSDKGSTarballPathLinux = "development/%s/sdk/linux-amd64/gn.tar.gz"
)

// FuchsiaSDKChild is an implementation of Child which deals with the Fuchsia
// SDK.
type FuchsiaSDKChild struct {
	gsBucket          string
	gsLatestPathLinux string
	gsLatestPathMac   string
	includeMacSDK     bool
	storageClient     *storage.Client
}

// NewFuchsiaSDK returns a Child implementation which deals with the Fuchsia
// SDK.
func NewFuchsiaSDK(ctx context.Context, c *config.FuchsiaSDKChildConfig, client *http.Client) (*FuchsiaSDKChild, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate config: %s", err)
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Failed to create storage client: %s", err)
	}

	rv := &FuchsiaSDKChild{
		gsBucket:          FuchsiaSDKGSBucket,
		gsLatestPathLinux: FuchsiaSDKGSLatestPathLinux,
		gsLatestPathMac:   FuchsiaSDKGSLatestPathMac,
		includeMacSDK:     c.IncludeMacSdk,
		storageClient:     storageClient,
	}
	return rv, nil
}

// GetRevision implements Child.
func (c *FuchsiaSDKChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return fuchsiaSDKVersionToRevision(id), nil
}

// Update implements Child.
func (c *FuchsiaSDKChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	// Get latest SDK version.
	tipRevBytes, err := gcs.FileContentsFromGCS(c.storageClient, c.gsBucket, c.gsLatestPathLinux)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to read latest SDK version (linux)")
	}
	tipRev, err := c.GetRevision(ctx, strings.TrimSpace(string(tipRevBytes)))
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if c.includeMacSDK {
		tipRevMacBytes, err := gcs.FileContentsFromGCS(c.storageClient, c.gsBucket, c.gsLatestPathMac)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to read latest SDK version (mac)")
		}
		// TODO(borenet): Is there a better dep ID than the GCS path?
		tipRev.Dependencies = map[string]string{
			FuchsiaSDKGSLatestPathMac: strings.TrimSpace(string(tipRevMacBytes)),
		}
	}

	// We cannot compute notRolledRevs correctly because there are things
	// other than SDKs in the GCS dir, and because they are content-
	// addressed, we can't tell which ones are relevant to us, so we only
	// include the latest and don't bother loading the list of versions
	// from GCS.
	notRolledRevs := []*revision.Revision{}
	if tipRev.Id != lastRollRev.Id {
		notRolledRevs = append(notRolledRevs, tipRev)
	}
	return tipRev, notRolledRevs, nil
}

// VFS implements the Child interface.
func (c *FuchsiaSDKChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	fs, err := vfs.TempDir(ctx, "", "")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	gcsPath := fmt.Sprintf(FuchsiaSDKGSTarballPathLinux, rev.Id)
	if err := gcs.DownloadAndExtractTarGz(ctx, c.storageClient, FuchsiaSDKGSBucket, gcsPath, fs.Dir()); err != nil {
		return nil, skerr.Wrap(err)
	}
	return fs, nil
}

// fuchsiaSDKVersionToRevision returns a revision.Revision instance based on the
// given version ID.
func fuchsiaSDKVersionToRevision(ver string) *revision.Revision {
	return &revision.Revision{
		Id: ver,
	}
}

// fuchsiaSDKChild implements Child.
var _ Child = &FuchsiaSDKChild{}
