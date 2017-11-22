// Runs "gcloud compute project-info describe --format=json" and parses the output to find the
// 'jwt_service_account' metadata, which is then written to a local file "service-account.json".
package main

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
)

const (
	OUTPUT_FILENAME = "service-account.json"
)

/*
  The output of the project-info looks like:

  {
    "commonInstanceMetadata": {
      "fingerprint": "uc...=",
      "items": [
      {
        "key": "apikey",
        "value": "AI..."
      },
      {
        "key": "appurify_key",
        "value": "094..."
      },

    The following types are for parsing the output.
*/

type ProjectInfo struct {
	CommonInstanceMetadata Metadata `json:"commonInstanceMetadata"`
}

type Metadata struct {
	FingerPrint string  `json:"fingerprint"`
	Items       []*Item `json:"items"`
}

type Item struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func main() {
	output, err := exec.RunSimple(context.Background(), "gcloud --quiet compute project-info describe --format=json --project google.com:skia-buildbots")
	if err != nil {
		sklog.Fatalf("Failed to execute gcloud: %s", err)
	}
	pi := &ProjectInfo{
		CommonInstanceMetadata: Metadata{
			Items: []*Item{},
		},
	}
	if err := json.Unmarshal([]byte(output), pi); err != nil {
		sklog.Fatalf("Failed to parse gcloud output: %s", err)
	}
	items := pi.CommonInstanceMetadata.Items
	body := ""
	for _, item := range items {
		if item.Key == metadata.JWT_SERVICE_ACCOUNT {
			body = item.Value
			break
		}
	}
	if body == "" {
		sklog.Fatalf("Failed to find the JST service account in the metadata.")
	}
	if err := ioutil.WriteFile(OUTPUT_FILENAME, []byte(body), 0600); err != nil {
		sklog.Fatalf("Failed to write %q: %s", OUTPUT_FILENAME, err)
	}
}
