// Trigger backups of Cloud Datastore entities to Cloud Storage using the
// datastore v1beta1 API.
//
// See http://go/datastore-backup-example for an example in the APIs Explorer.
//
// At the time of this writing there is a cloud golang client, but it is ugly,
// filled with default named types such as
// GoogleDatastoreAdminV1beta1ExportEntitiesRequest, which I presume will
// change before leaving beta. We can update to the cloud golang client once it
// leaves beta.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	URL = "https://datastore.googleapis.com/v1beta1/projects/google.com:skia-buildbots:export"
)

// flags
var (
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
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

func step(client *http.Client) error {
	//
	// Configure what gets backed up here by adding to ds.KindsToBackup.
	//
	success := true
	for ns, kinds := range ds.KindsToBackup {
		req := Request{
			OutputUrlPrefix: "gs://skia-backups/ds/" + time.Now().Format("2006/01/02/15/"),
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
		resp, err := client.Post(URL, "application/json", buf)
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

func main() {
	common.InitWithMust(
		"datastore_backup",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	client, err := auth.NewDefaultJWTServiceAccountClient(datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	if err := step(client); err != nil {
		sklog.Errorf("Failed to do first backup step: %s", err)
	}
	for _ = range time.Tick(24 * time.Hour) {
		if err := step(client); err != nil {
			sklog.Errorf("Failed to backup: %s", err)
		}
	}
}
