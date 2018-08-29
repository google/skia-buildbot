// Common code for backing up datastore entities.
//
// At the time of this writing there is a cloud golang client, but it is ugly,
// filled with default named types such as
// GoogleDatastoreAdminV1beta1ExportEntitiesRequest, which I presume will
// change before leaving beta. We can update to the cloud golang client once it
// leaves beta.
package backup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	URL = "https://datastore.googleapis.com/v1beta1/projects/%s:export"
)

var (
	backupStep    = metrics2.NewLiveness("backup_step")
	backupSuccess = metrics2.NewLiveness("backup_success")
)

type EntityFilter struct {
	Kinds        []ds.Kind `json:"kinds"`
	NamespaceIds []string  `json:"namespaceIds"`
}

type Request struct {
	OutputUrlPrefix string       `json:"outputUrlPrefix"`
	EntityFilter    EntityFilter `json:"entityFilter"`
}

// singleRequest makes a single http POST request to 'url' with a body of
// 'buf'.
//
// Returns the http.Response and a bool that is true if the request should be
// retried because there are already the maximum number of exports running.
func singleRequest(client *http.Client, url string, buf *bytes.Buffer) (*http.Response, error, bool) {
	shouldRetry := false
	resp, err := client.Post(url, "application/json", buf)
	if resp != nil {
		if resp.StatusCode == 429 {
			sklog.Infof("Got 429 RESOURCE_EXHAUSTED, waiting to retry operation.")
			shouldRetry = true
			util.Close(resp.Body)
		}
	}
	return resp, err, shouldRetry
}

// Step runs a single backup of all the entities listed in ds.KindsToBackup
// for the given project, data is written to the given GCS bucker.
func Step(client *http.Client, project, bucket string) error {
	//
	// Configure what gets backed up here by adding to ds.KindsToBackup.
	//
	success := true
	for ns, kinds := range ds.KindsToBackup {
		req := Request{
			OutputUrlPrefix: fmt.Sprintf("gs://%s/ds/", bucket) + time.Now().Format("2006/01/02/15/04"),
			EntityFilter: EntityFilter{
				Kinds:        kinds,
				NamespaceIds: []string{ns},
			},
		}
		b, err := json.Marshal(req)
		if err != nil {
			sklog.Errorf("Failed to encode request: %s-%v: %s", ns, kinds, err)
			success = false
			continue
		}
		url := fmt.Sprintf(URL, project)
		var resp *http.Response

		shouldRetry := true
		for shouldRetry { // Could retry forever, but then backupSuccess will never be Reset() and that will trigger an alert.
			resp, err, shouldRetry = singleRequest(client, url, bytes.NewBuffer(b))
			if shouldRetry {
				time.Sleep(10 * time.Minute)
			}
		}
		if err != nil {
			sklog.Errorf("Request failed: %s-%v: %s", ns, kinds, err)
			success = false
			continue
		} else {
			sklog.Infof("Successfully started backup: %s-%v", ns, kinds)
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			sklog.Errorf("Failed to read response: %s-%v: %s", ns, kinds, err)
			success = false
			continue
		}
		if resp.StatusCode >= 500 {
			success = false

			// Emit the reponse into the structured logs, but make sure the JSON is
			// only a singe line by decoding and re-encoding as JSON using
			// json.Marshal().
			var parsed interface{}
			err := json.Unmarshal(bodyBytes, &parsed)
			if err != nil {
				sklog.Errorf("Response was invalid JSON: %s", err)
				continue
			}
			singleLine, err := json.Marshal(parsed)
			if err != nil {
				sklog.Errorf("Unable to convert response to JSON: %s", err)
				continue
			}
			fmt.Print(string(singleLine))
			continue
		}
		sklog.Infof("Started backup of %s-%v", ns, kinds)
	}
	if success {
		backupSuccess.Reset()
	}
	backupStep.Reset()
	return nil
}
