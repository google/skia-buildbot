package auth

import (
	"fmt"
	"net/http"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/oxtoacart/webbrowser"
	"skia.googlesource.com/buildbot.git/go/util"
)

const (
	// TIMEOUT is the http timeout when making Google Storage requests.
	TIMEOUT = time.Duration(time.Minute)
)

// DefaultOAuthConfig returns the default configuration for oauth.
// If the given path for the cachefile is empty a default value is
// used.
func DefaultOAuthConfig(cacheFilePath string) *oauth.Config {
	if cacheFilePath == "" {
		cacheFilePath = "google_storage_token.data"
	}

	return &oauth.Config{
		ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
		ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
		Scope:        "https://www.googleapis.com/auth/devstorage.read_only",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		TokenCache:   oauth.CacheFile(cacheFilePath),
	}
}

// runFlow runs through a 3LO OAuth 2.0 flow to get credentials for Google Storage.
func RunFlow(config *oauth.Config) (*http.Client, error) {
	if config == nil {
		config = DefaultOAuthConfig("")
	}
	transport := &oauth.Transport{
		Config: config,
		Transport: &http.Transport{
			Dial: util.DialTimeout,
		},
	}
	if _, err := config.TokenCache.Token(); err != nil {
		url := config.AuthCodeURL("")
		fmt.Printf(`Your browser has been opened to visit:

  %s

Enter the verification code:`, url)
		webbrowser.Open(url)
		var code string
		fmt.Scan(&code)
		if _, err := transport.Exchange(code); err != nil {
			return nil, err
		}
	}

	return transport.Client(), nil
}
