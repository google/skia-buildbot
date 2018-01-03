// The tsuite package contains datastructures run tests on Firebase Testlab and
// process the results.
package tsuite

import (
	"context"
	"encoding/json"
	"net/http"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

// TODO(stephana): Expand this to read the results and meta data from
// GCS and ingest into Gold or a separate tool.

// FirebaseDevice contains the information and JSON tags for device information
// returned by firebase.
type FirebaseDevice struct {
	Brand        string   `json:"brand"`
	Form         string   `json:"form"`
	ID           string   `json:"id"`
	Manufacturer string   `json:"manufacturer"`
	Name         string   `json:"name"`
	VersionIDs   []string `json:"supportedVersionIds"`
}

// DeviceVersions combines device information from Firebase Testlab with
// a selected list of versions. This is used to define a subset of versions
// used by a devices.
type DeviceVersions struct {
	Device *FirebaseDevice

	// Versions contains the version ids of interest contained in Device.
	Versions []string
}

// TestRunMeta contains the meta data of a complete testrun on firebase.
type TestRunMeta struct {
	ID             string            `json:"id"`
	TS             int64             `json:"timeStamp"`
	Devices        []*DeviceVersions `json:"devices"`
	IgnoredDevices []*DeviceVersions `json:"ignoredDevices"`
	ExitCode       int               `json:"exitCode"`
}

// TODO(stephana): WriteToGCS should probably be converted to accepting an
// instance of Client from the cloud.google.com/go/storage package.
// Add this as the package evolves.

// WriteToGCS writes the meta data as JSON to the given bucket and path in
// GCS. It assumes that the provided client as permissions to write to the
// specified location in GCS.
func (t *TestRunMeta) WriteToGCS(bucket, path string, client *http.Client) error {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	w := storageClient.Bucket(bucket).Object(path).NewWriter(context.Background())
	if err := json.NewEncoder(w).Encode(t); err != nil {
		return err
	}
	defer util.Close(w)

	sklog.Infof("Sucess: Meta data written to %s/%s", bucket, path)
	return nil
}
