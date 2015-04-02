// gsauth contains utilities for authenticated access to Google Storage.
package gsauth

import (
	"net/http"

	"code.google.com/p/google-api-go-client/compute/v1"
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/util"
)

// NewStoreAndClient returns an authenticated http client and Google Storage service.
//
// You actually need both if you are going to download file contents.
func NewClient(doOAuth bool, oauthCacheFile string) (*http.Client, error) {
	var client *http.Client
	var err error

	transport := &http.Transport{
		Dial: util.DialTimeout,
	}

	if doOAuth {
		// Use a local client secret file to load data.
		client, err = auth.InstalledAppClient(oauthCacheFile, "client_secret.json",
			transport,
			storage.DevstorageFull_controlScope,
			compute.ComputeReadonlyScope)
		if err != nil {
			glog.Fatalf("Unable to create installed app oauth client:%s", err)
		}
	} else {
		// Use compute engine service account.
		client = auth.GCEServiceAccountClient(transport)
	}

	return client, nil
}
