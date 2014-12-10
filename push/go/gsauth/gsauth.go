// gsauth contains utilities for authenticated access to Google Storage.
package gsauth

import (
	"fmt"
	"net/http"

	"code.google.com/p/goauth2/compute/serviceaccount"

	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/util"
)

// NewStoreAndClient returns an authenticated http client and Google Storage service.
//
// You actually need both if you are going to download file contents.
func NewClient(doOAuth bool, oauthCacheFile string) (*http.Client, error) {
	var client *http.Client
	var err error
	if doOAuth {
		config := auth.OAuthConfig(oauthCacheFile, auth.SCOPE_FULL_CONTROL)
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
