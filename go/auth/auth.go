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
	// Supported Cloud storage API OAuth scopes.
	SCOPE_READ_ONLY    = "https://www.googleapis.com/auth/devstorage.read_only"
	SCOPE_READ_WRITE   = "https://www.googleapis.com/auth/devstorage.read_write"
	SCOPE_FULL_CONTROL = "https://www.googleapis.com/auth/devstorage.full_control"
	SCOPE_GCE          = "https://www.googleapis.com/auth/compute"
)

// DefaultOAuthConfig returns the default configuration for oauth.
// If the given path for the cachefile is empty a default value is
// used.
func DefaultOAuthConfig(cacheFilePath string) *oauth.Config {
	return OAuthConfig(cacheFilePath, SCOPE_READ_ONLY)
}

// OAuthConfig returns a configuration for oauth with the specified scope.
// If the given path for the cachefile is empty a default value is used.
func OAuthConfig(cacheFilePath, scope string) *oauth.Config {
	if cacheFilePath == "" {
		cacheFilePath = "google_storage_token.data"
	}
	return &oauth.Config{
		ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
		ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
		Scope:        scope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		TokenCache:   oauth.CacheFile(cacheFilePath),
	}
}

// RunFlowWithTransport runs through a 3LO OAuth 2.0 flow to get credentials for
// Google Storage using the specified HTTP transport.
func RunFlowWithTransport(config *oauth.Config, transport http.RoundTripper) (*http.Client, error) {
	if config == nil {
		config = DefaultOAuthConfig("")
	}
	oauthTransport := &oauth.Transport{
		Config:    config,
		Transport: transport,
	}
	if _, err := config.TokenCache.Token(); err != nil {
		url := config.AuthCodeURL("")
		fmt.Printf(`Your browser has been opened to visit:

  %s

Enter the verification code:`, url)
		webbrowser.Open(url)
		var code string
		fmt.Scan(&code)
		if _, err := oauthTransport.Exchange(code); err != nil {
			return nil, fmt.Errorf("Failed exchange: %s", err)
		}
	}

	return oauthTransport.Client(), nil
}

// runFlow runs through a 3LO OAuth 2.0 flow to get credentials for Google Storage.
// Uses an HTTP transport with a dial timeout. Use RunFlowWithTransport to specify
// your own HTTP transport.
func RunFlow(config *oauth.Config) (*http.Client, error) {
	return RunFlowWithTransport(config, &http.Transport{Dial: util.DialTimeout})
}
