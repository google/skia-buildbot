package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	cloud_metadata "cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	oauth2_api "google.golang.org/api/oauth2/v2"
	storage "google.golang.org/api/storage/v1"
)

const (
	DEFAULT_JWT_FILENAME           = "service-account.json"
	DEFAULT_CLIENT_SECRET_FILENAME = "client_secret.json"
	DEFAULT_TOKEN_STORE_FILENAME   = "google_storage_token.data"
)

// NewDefaultTokenSource creates a new OAuth 2.0 token source. If local is true
// then it uses the credentials it gets from running:
//
//    gcloud auth print-access-token
//
// otherwise the GCE Service Account is used if running in GCE, and the Skolo
// access token provider is used if running in Skolo.
//
// Note: The default project for gcloud is used, and can be changed by running
//
//    $ gcloud config set project [project name]
//
// local  - If true then use the gcloud command line tool.
// scopes - The scopes requested.
//
// When run on GCE the scopes are ignored in favor of the scopes
// set on the instance, see:
//
//    https://cloud.google.com/sdk/gcloud/reference/compute/instances/set-service-account
//
func NewDefaultTokenSource(local bool, scopes ...string) (oauth2.TokenSource, error) {
	if local {
		return NewGCloudTokenSource(""), nil
	} else {
		return google.DefaultTokenSource(context.Background(), scopes...)
	}
}

// ClientFromTokenSource creates an http.Client with a BackOff transport and a
// request timeout.
func ClientFromTokenSource(tok oauth2.TokenSource) *http.Client {
	return httputils.AddMetricsToClient(&http.Client{
		Transport: &oauth2.Transport{
			Source: tok,
			Base:   httputils.NewBackOffTransport(),
		},
		Timeout: httputils.REQUEST_TIMEOUT,
	})
}

// TimeoutClientFromTokenSource creates an http.Client with a Timeout transport and a
// request timeout.
func TimeoutClientFromTokenSource(tok oauth2.TokenSource) *http.Client {
	return httputils.AddMetricsToClient(&http.Client{
		Transport: &oauth2.Transport{
			Source: tok,
			Base:   &http.Transport{Dial: httputils.DialTimeout},
		},
		Timeout: httputils.REQUEST_TIMEOUT,
	})
}

// NewDefaultClient creates a new OAuth 2.0 authorized client with all the
// defaults for the given scopes. If local is true then a 3-legged flow is
// initiated, otherwise the GCE Service Account is used if running in GCE, and
// the Skolo access token provider is used if running in Skolo.
//
// The default OAuth config filename is "client_secret.json".
// The default OAuth token store filename is "google_storage_token.data".
func NewDefaultClient(local bool, scopes ...string) (*http.Client, error) {
	return NewClient(local, "", scopes...)
}

// NewClient creates a new OAuth 2.0 authorized client with all the defaults
// for the given scopes, and the given token store filename. If local is true
// then a 3-legged flow is initiated, otherwise the GCE Service Account is
// used.
//
// The default OAuth config filename is "client_secret.json".
func NewClient(local bool, oauthCacheFile string, scopes ...string) (*http.Client, error) {
	return NewClientWithTransport(local, oauthCacheFile, "", nil, scopes...)
}

type gcloudTokenSource struct {
	projectId string
}

// NewGCloudTokenSource creates an oauth2.TokenSource that returns tokens from
// the locally authorized gcloud command line tool, i.e. it gets them from
// running:
//
//    gcloud auth print-access-token
//
// projectId - The name of the GCP project, e.g. 'skia-public'. If empty, "", then
//    the default project id for gcloud is used.
func NewGCloudTokenSource(projectId string) oauth2.TokenSource {
	ts := &gcloudTokenSource{
		projectId: projectId,
	}
	return oauth2.ReuseTokenSource(nil, ts)
}

func (g *gcloudTokenSource) Token() (*oauth2.Token, error) {
	buf := bytes.Buffer{}
	errBuf := bytes.Buffer{}
	args := []string{"auth", "print-access-token", "--format=json"}
	if g.projectId != "" {
		args = append(args, fmt.Sprintf("--project=%s", g.projectId))
	}
	gcloudCmd := &exec.Command{
		Name:        "gcloud",
		Args:        args,
		InheritPath: true,
		InheritEnv:  true,
		Stdout:      &buf,
		Stderr:      &errBuf,
	}
	if err := exec.Run(context.Background(), gcloudCmd); err != nil {
		return nil, fmt.Errorf("Failed fetching access token: %s - %s", err, errBuf.String())
	}
	type TokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresInSec int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	var res struct {
		TokenResponse TokenResponse `json:"token_response"`
	}
	if err := json.NewDecoder(&buf).Decode(&res); err != nil {
		return nil, fmt.Errorf("Invalid token JSON from metadata: %v", err)
	}
	if res.TokenResponse.ExpiresInSec == 0 || res.TokenResponse.AccessToken == "" {
		return nil, fmt.Errorf("Incomplete token received from metadata")
	}
	return &oauth2.Token{
		AccessToken: res.TokenResponse.AccessToken,
		TokenType:   res.TokenResponse.TokenType,
		Expiry:      time.Now().Add(time.Duration(res.TokenResponse.ExpiresInSec) * time.Second),
	}, nil
}

// NewClientFromIdAndSecret creates a new OAuth 2.0 authorized client with all the defaults
// for the given scopes, and the given token store filename.
func NewClientFromIdAndSecret(clientId, clientSecret, oauthCacheFile string, scopes ...string) (*http.Client, error) {
	config := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}
	return NewClientFromConfigAndTransport(true, config, oauthCacheFile, nil)
}

// NewClientWithTransport creates a new OAuth 2.0 authorized client. If local
// is true then a 3-legged flow is initiated, otherwise the GCE Service Account
// is used.
//
// The OAuth tokens will be stored in oauthCacheFile.
// The OAuth config will come from oauthConfigFile.
// The transport will be used. If nil then httputils.NewBackOffTransport() is used.
func NewClientWithTransport(local bool, oauthCacheFile string, oauthConfigFile string, transport http.RoundTripper, scopes ...string) (*http.Client, error) {
	// If this is running locally we need to load the oauth configuration.
	var config *oauth2.Config = nil
	if local {
		if oauthConfigFile == "" {
			oauthConfigFile = DEFAULT_CLIENT_SECRET_FILENAME
		}
		body, err := ioutil.ReadFile(oauthConfigFile)
		if err != nil {
			return nil, err
		}
		config, err = google.ConfigFromJSON(body, scopes...)
		if err != nil {
			return nil, err
		}
	}

	return NewClientFromConfigAndTransport(local, config, oauthCacheFile, transport)
}

// NewClientFromConfigAndTransport creates an new OAuth 2.0 authorized client
// for the given config and transport.
//
// If the transport is nil then httputils.NewBackOffTransport() is used.
// If local is true then a 3-legged flow is initiated, otherwise the GCE
// Service Account is used.
func NewClientFromConfigAndTransport(local bool, config *oauth2.Config, oauthCacheFile string, transport http.RoundTripper) (*http.Client, error) {
	if oauthCacheFile == "" {
		oauthCacheFile = DEFAULT_TOKEN_STORE_FILENAME
	}
	if transport == nil {
		transport = httputils.NewBackOffTransport()
	}

	var client *http.Client
	if local {
		tokenClient := &http.Client{
			Transport: transport,
			Timeout:   httputils.REQUEST_TIMEOUT,
		}
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient)
		tokenSource, err := newCachingTokenSource(oauthCacheFile, ctx, config)
		if err != nil {
			return nil, fmt.Errorf("NewClientFromConfigAndTransport: Unable to create token source: %s", err)
		}
		client = httputils.AddMetricsToClient(&http.Client{
			Transport: &oauth2.Transport{
				Source: tokenSource,
				Base:   transport,
			},
			Timeout: httputils.REQUEST_TIMEOUT,
		})
	} else {
		// Are we running on GCE?
		if cloud_metadata.OnGCE() {
			// Use compute engine service account.
			client = GCEServiceAccountClient(transport)
		} else {
			// Create and use a token provider for skolo service account access tokens.
			client = SkoloServiceAccountClient(transport)
		}
	}

	return client, nil
}

const (
	// Supported Cloud storage API OAuth scopes.
	SCOPE_READ_ONLY         = storage.DevstorageReadOnlyScope
	SCOPE_READ_WRITE        = storage.DevstorageReadWriteScope
	SCOPE_FULL_CONTROL      = storage.DevstorageFullControlScope
	SCOPE_COMPUTE_READ_ONLY = compute.ComputeReadonlyScope
	SCOPE_GCE               = compute.ComputeScope
	SCOPE_GERRIT            = "https://www.googleapis.com/auth/gerritcodereview"
	SCOPE_PLUS_ME           = "https://www.googleapis.com/auth/plus.me"
	SCOPE_PUBSUB            = pubsub.ScopePubSub
	SCOPE_USERINFO_EMAIL    = "https://www.googleapis.com/auth/userinfo.email"
	SCOPE_USERINFO_PROFILE  = "https://www.googleapis.com/auth/userinfo.profile"
)

// GCEServiceAccountClient creates an oauth client that is uses the auth token
// attached to an instance in GCE. This requires that the necessary scopes are
// attached to the instance upon creation.  See details here:
// https://cloud.google.com/compute/docs/authentication If transport is nil,
// the default transport will be used.
func GCEServiceAccountClient(transport http.RoundTripper) *http.Client {
	return httputils.AddMetricsToClient(&http.Client{
		Transport: &oauth2.Transport{
			Source: google.ComputeTokenSource(""),
			Base:   transport,
		},
		Timeout: httputils.REQUEST_TIMEOUT,
	})
}

// skoloTokenSource implements the oauth2.TokenSource interface using tokens
// from the skolo metadata server.
type skoloTokenSource struct {
	client *http.Client
}

func newSkoloTokenSource() oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, &skoloTokenSource{
		client: httputils.NewBackOffClient(),
	})
}

func (s *skoloTokenSource) Token() (*oauth2.Token, error) {
	resp, err := s.client.Get(metadata.TOKEN_URL)
	if err != nil {
		sklog.Errorf("Failed to retrieve token:  %s", err)
		return nil, fmt.Errorf("Failed to retrieve token: %s", err)
	}
	defer util.Close(resp.Body)
	type TokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresInSec int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	var res TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("Invalid token JSON from metadata: %v", err)
	}
	if res.ExpiresInSec == 0 || res.AccessToken == "" {
		return nil, fmt.Errorf("Incomplete token received from metadata")
	}
	return &oauth2.Token{
		AccessToken: res.AccessToken,
		TokenType:   res.TokenType,
		Expiry:      time.Now().Add(time.Duration(res.ExpiresInSec) * time.Second),
	}, nil
}

// SkoloServiceAccountClient creates an oauth client that uses the auth token
// provided by the skolo metadata server.
func SkoloServiceAccountClient(transport http.RoundTripper) *http.Client {
	return httputils.AddMetricsToClient(&http.Client{
		Transport: &oauth2.Transport{
			Source: newSkoloTokenSource(),
			Base:   transport,
		},
		Timeout: httputils.REQUEST_TIMEOUT,
	})
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
// retrieve the token in the first place.  If no token is available it will run
// though the oauth flow for an installed app.
func newCachingTokenSource(cacheFilePath string, ctx context.Context, config *oauth2.Config) (oauth2.TokenSource, error) {
	var tok *oauth2.Token = nil
	var err error

	if cacheFilePath == "" {
		sklog.Warningf("cacheFilePath is empty. Not caching auth token.")
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
		sklog.Infof("Token saved to %s", cacheFilePath)
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

// NewDefaultServiceAccountClient Looks for the JWT JSON in metadata, falls
// back to a local file names "service-account.json" if metadata isn't
// available.
func NewDefaultJWTServiceAccountClient(scopes ...string) (*http.Client, error) {
	return NewJWTServiceAccountClient("", "", nil, scopes...)
}

// NewJWTServiceAccountClient creates a new http.Client that is loaded by first
// attempting to load the JWT JSON Service Account data from GCE Project Level
// metadata, and if that fails falls back to loading the data from a local
// file.
//
//   metadataname - The name of the GCE project level metadata key that holds the JWT JSON. If empty a default is used.
//   filename - The name of the local file that holds the JWT JSON. If empty a default is used.
//   transport - A transport. If nil then a default is used.
func NewJWTServiceAccountClient(metadataname, filename string, transport http.RoundTripper, scopes ...string) (*http.Client, error) {
	if metadataname == "" {
		metadataname = metadata.JWT_SERVICE_ACCOUNT
	}
	if filename == "" {
		filename = DEFAULT_JWT_FILENAME
	}
	if transport == nil {
		transport = httputils.NewBackOffTransport()
	}
	tokenStore, err := NewJWTServiceAccountTokenSource(metadataname, filename, scopes...)
	if err != nil {
		return nil, err
	}
	return httputils.AddMetricsToClient(&http.Client{
		Transport: &oauth2.Transport{
			Source: tokenStore,
			Base:   transport,
		},
		Timeout: httputils.REQUEST_TIMEOUT,
	}), nil
}

// NewJWTServiceAccountTokenSource creates a new oauth2.TokenSource that
// is loaded first by attempting to load JWT JSON Service Account data from GCE
// Project Level metadata, and if that fails falls back to loading the data
// from a local file.
//
//   metadataname - The name of the GCE project level metadata key that holds the JWT JSON. If empty a default is used.
//   filename - The name of the local file that holds the JWT JSON. If empty a default is used.
func NewJWTServiceAccountTokenSource(metadataname, filename string, scopes ...string) (oauth2.TokenSource, error) {
	if metadataname == "" {
		metadataname = metadata.JWT_SERVICE_ACCOUNT
	}
	if filename == "" {
		filename = DEFAULT_JWT_FILENAME
	}
	var body []byte
	jwt, err := metadata.ProjectGet(metadataname)
	if err != nil {
		body, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("Couldn't find JWT via metadata or in a local file.")
		}
	} else {
		body = []byte(jwt)
	}
	tokenClient := &http.Client{
		Transport: httputils.NewBackOffTransport(),
		Timeout:   httputils.REQUEST_TIMEOUT,
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient)
	jwtConfig, err := google.JWTConfigFromJSON(body, scopes...)
	if err != nil {
		return nil, err
	}
	return jwtConfig.TokenSource(ctx), nil
}

// NewDefaultJWTServiceAccountTokenSource creates a new oauth2.TokenSource that
// is loaded first by attempting to load JWT JSON Service Account data from GCE
// Project Level metadata, and if that fails falls back to loading the data
// from a local file.
func NewDefaultJWTServiceAccountTokenSource(scopes ...string) (oauth2.TokenSource, error) {
	return NewJWTServiceAccountTokenSource("", "", scopes...)
}

// ValidateBearerToken takes an OAuth 2.0 Bearer token (e.g. The third part of
// Authorization: Bearer ya29.Elj...
// and polls a Google HTTP endpoint to see if is valid. This is fine in low-volumne
// situations, but another solution may be needed if this goes higher than a few QPS.
func ValidateBearerToken(token string) (*oauth2_api.Tokeninfo, error) {
	c, err := oauth2_api.New(httputils.NewTimeoutClient())
	if err != nil {
		return nil, fmt.Errorf("could not make oauth2 api client: %s", err)
	}
	return c.Tokeninfo().AccessToken(token).Do()
}
