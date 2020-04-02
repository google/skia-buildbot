package child

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/api/option"
)

const (
	FUCHSIA_SDK_GS_BUCKET            = "fuchsia"
	FUCHSIA_SDK_GS_LATEST_PATH_LINUX = "development/LATEST_LINUX"
	FUCHSIA_SDK_GS_LATEST_PATH_MAC   = "development/LATEST_MAC"
)

// FuchsiaSDKConfig provides configuration for FuchsiaSDKChild.
type FuchsiaSDKConfig struct {
	IncludeMacSDK bool `json:"includeMacSDK"`
}

// See documentation for util.Validator interface.
func (c FuchsiaSDKConfig) Validate() error {
	// Can't validate a lone bool...
	return nil
}

// FuchsiaSDKChild is an implementation of Child which deals with the Fuchsia SDK.
type FuchsiaSDKChild struct {
	gsBucket          string
	gsLatestPathLinux string
	gsLatestPathMac   string
	includeMacSDK     bool
	storageClient     *storage.Client
}

// NewFuchsiaSDK returns a Child implementation which deals with the Fuchsia SDK.
func NewFuchsiaSDK(ctx context.Context, c FuchsiaSDKConfig, client *http.Client) (*FuchsiaSDKChild, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate config: %s", err)
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Failed to create storage client: %s", err)
	}

	rv := &FuchsiaSDKChild{
		gsBucket:          FUCHSIA_SDK_GS_BUCKET,
		gsLatestPathLinux: FUCHSIA_SDK_GS_LATEST_PATH_LINUX,
		gsLatestPathMac:   FUCHSIA_SDK_GS_LATEST_PATH_MAC,
		includeMacSDK:     c.IncludeMacSDK,
		storageClient:     storageClient,
	}
	return rv, nil
}

// See documentation for Child interface.
func (c *FuchsiaSDKChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return fuchsiaSDKVersionToRevision(id), nil
}

// See documentation for Child interface.
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
		tipRev.Dependencies[FUCHSIA_SDK_GS_LATEST_PATH_MAC] = strings.TrimSpace(string(tipRevMacBytes))
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

// fuchsiaSDKVersionToRevision returns a revision.Revision instance based on the
// given version ID.
func fuchsiaSDKVersionToRevision(ver string) *revision.Revision {
	return &revision.Revision{
		Id: ver,
	}
}
