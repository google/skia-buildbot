// gsauth contains utilities for authenticated access to Google Storage.
package gsauth

import (
	"fmt"
	"net/http"

	"code.google.com/p/goauth2/compute/serviceaccount"
	"code.google.com/p/google-api-go-client/compute/v1"
	"code.google.com/p/google-api-go-client/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/util"
)

// NewStoreAndClient returns an authenticated http client and Google Storage service.
//
// You actually need both if you are going to download file contents.
func NewClient(doOAuth bool, oauthCacheFile string) (*http.Client, error) {
	var client *http.Client
	var err error
	if doOAuth {
		config := auth.OAuthConfig(oauthCacheFile, storage.DevstorageFull_controlScope+" "+compute.ComputeReadonlyScope)
		client, err = auth.RunFlow(config)
		if err != nil {
			return nil, fmt.Errorf("Failed to auth: %s", err)
		}
	} else {
		ops := &serviceaccount.Options{
			Transport: &http.Transport{
				Dial: util.DialTimeout,
			},
		}
		client, err = serviceaccount.NewClient(ops)
		if err != nil {
			return nil, fmt.Errorf("Failed to create service account authorized client: %s", err)
		}
	}
	return client, nil
}
