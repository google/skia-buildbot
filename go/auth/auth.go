package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/oxtoacart/webbrowser"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// TODO(stephana): Remove the goauth2 dependency and convert everything to
// using the newer golang.org/x/oauth2 package.

const (
	// TIMEOUT is the http timeout when making Google Storage requests.
	TIMEOUT = time.Duration(time.Minute)
	// Supported Cloud storage API OAuth scopes.
	SCOPE_READ_ONLY    = "https://www.googleapis.com/auth/devstorage.read_only"
	SCOPE_READ_WRITE   = "https://www.googleapis.com/auth/devstorage.read_write"
	SCOPE_FULL_CONTROL = "https://www.googleapis.com/auth/devstorage.full_control"
	SCOPE_GCE          = "https://www.googleapis.com/auth/compute"
)

// GCEServiceAccountClient creates an oauth client that is uses the auth token
// attached to an instance in GCE. This requires that the necessary scopes are
// attached to the instance upon creation.
// See details here: https://cloud.google.com/compute/docs/authentication
// If transport is nil, the default transport will be used.
func GCEServiceAccountClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{
			Source: google.ComputeTokenSource(""),
			Base:   transport,
		},
	}
}

// InstalledAppClient creates an oauth authenticated client for an installed
// app based on the credentials from the developer console from an
// account labeled 'Client ID for native application'.
// cacheFilePath is the path to where the token should be cached,
// configFilePath is the path to the config file downloaded from GCE and
// scopes is the list of desired oauth2 scopes.
// If transport is nil, the default transport will be used.
// If the cache file does not contain a token the oauth2 flow will be trigger.
func InstalledAppClient(cacheFilePath, configFilePath string, transport http.RoundTripper, scopes ...string) (*http.Client, error) {
	jsonKey, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	config, err := google.ConfigFromJSON(jsonKey, scopes...)
	if err != nil {
		return nil, err
	}

	tokenSource, err := newCachingTokenSource(cacheFilePath, oauth2.NoContext, config)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
			Base:   transport,
		},
	}

	return client, nil
}

// cachingTokenSource implments the oauth2.TokenSource interface and
// caches the oauth token in a file.
type cachingTokenSource struct {
	cacheFilePath string
	tokenSource   oauth2.TokenSource
	lastToken     *oauth2.Token
}

// newCachingTokenSource creates a new instance of CachingTokenSource that
// caches the token in cacheFilePath. ctx and config are used to create and
// retrieve the token in the first place.
// If no token is available it will run though the oauth flow for an
// installed app.
func newCachingTokenSource(cacheFilePath string, ctx context.Context, config *oauth2.Config) (oauth2.TokenSource, error) {
	var tok *oauth2.Token = nil
	var err error

	if cacheFilePath == "" {
		glog.Warningf("cacheFilePath is empty. Not caching auth token.")
	} else if _, err = os.Stat(cacheFilePath); err == nil {
		// If the file exists. Load from disk.
		f, err := os.Open(cacheFilePath)
		if err != nil {
			return nil, err
		}
		tok = &oauth2.Token{}
		if err = json.NewDecoder(f).Decode(tok); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// If there was no token, we run through the flow.
	if tok == nil {
		// Run through the flow.
		url := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
		fmt.Printf("Your browser has been opened to visit:\n\n%s\n\nEnter the verification code:", url)

		if err = webbrowser.Open(url); err != nil {
			return nil, fmt.Errorf("Failed to open web browser: %v", err)
		}

		var code string
		if _, err = fmt.Scan(&code); err != nil {
			return nil, err
		}
		tok, err = config.Exchange(ctx, code)
		if err != nil {
			return nil, err
		}

		if err = saveToken(cacheFilePath, tok); err != nil {
			return nil, err
		}
		glog.Infof("token: %v", tok)
	}

	// We have a token at this point.
	tokenSource := config.TokenSource(ctx, tok)
	return &cachingTokenSource{
		cacheFilePath: cacheFilePath,
		tokenSource:   tokenSource,
		lastToken:     tok,
	}, nil
}

// Token is part of implementing the oauth2.TokenSource interface.
func (c *cachingTokenSource) Token() (*oauth2.Token, error) {
	newToken, err := c.tokenSource.Token()
	if err != nil {
		return nil, err
	}

	if newToken.AccessToken != c.lastToken.AccessToken {
		// Write the token to file.
		if err := saveToken(c.cacheFilePath, newToken); err != nil {
			return nil, err
		}
	}

	c.lastToken = newToken
	return newToken, nil
}

func saveToken(cacheFilePath string, tok *oauth2.Token) error {
	if cacheFilePath == "" {
		return nil
	}

	if tok != nil {
		f, err := os.Create(cacheFilePath)
		if err != nil {
			return err
		}
		defer util.Close(f)

		if err := json.NewEncoder(f).Encode(tok); err != nil {
			return err
		}
	}
	return nil
}

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
		if err := webbrowser.Open(url); err != nil {
			return nil, fmt.Errorf("Failed to open web browser: %v", err)
		}
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
