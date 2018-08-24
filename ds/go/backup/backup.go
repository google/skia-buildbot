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
		buf := bytes.NewBuffer(b)
		url := fmt.Sprintf(URL, project)
		resp, err := client.Post(url, "application/json", buf)
		if err != nil {
			sklog.Errorf("Request failed: %s-%v: %s", ns, kinds, err)
			success = false
			continue
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		body := string(bodyBytes)
		if err != nil {
			sklog.Errorf("Failed to read response: %s-%v: %s", ns, kinds, err)
			success = false
			continue
		}
		if resp.StatusCode == 200 {
			sklog.Info(body)
		} else if resp.StatusCode >= 500 {
			success = false
			sklog.Error(body)
		} else {
			sklog.Warning(body)
		}
	}
	if success {
		backupSuccess.Reset()
	}
	backupStep.Reset()
	return nil
}
