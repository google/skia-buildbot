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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	defaultJwtFilename          = "service-account.json"
	defaultClientSecretFilename = "client_secret.json"
	defaultTokenStoreFilename   = "google_storage_token.data"
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

// Token implements oauth2.TokenSource by returning a local user's token via gcloud.
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
	type tokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresInSec int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	// In June of 2020  "gcloud auth print-access-token --format=json" changed
	// its output from a TokenResponse to just emitting {"token": "...."}. We
	// need to support both formats during the transition. Once everyone is on a
	// new version of gcloud then the res struct can be simplified to struct {
	// Token string `json:"token"`}.
	var res struct {
		TokenResponse tokenResponse `json:"token_response"`
		Token         string        `json:"token"`
	}
	if err := json.NewDecoder(&buf).Decode(&res); err != nil {
		return nil, fmt.Errorf("Invalid token JSON from gcloud: %v", err)
	}

	if res.TokenResponse.ExpiresInSec == 0 && res.TokenResponse.AccessToken == "" && res.Token == "" {
		return nil, fmt.Errorf("Incomplete token received from gcloud")
	}
	if res.TokenResponse.AccessToken != "" {
		return &oauth2.Token{
			AccessToken: res.TokenResponse.AccessToken,
			TokenType:   res.TokenResponse.TokenType,
			Expiry:      time.Now().Add(time.Duration(res.TokenResponse.ExpiresInSec) * time.Second),
		}, nil
	} else {
		return &oauth2.Token{
			AccessToken: res.Token,
			TokenType:   "Bearer",
			// The value for Expiry is just a guess, but it doesn't really
			// matter since AFAICT ReuseTokenSource just keeps using a token
			// until it fails and never checks the Expiry.
			Expiry: time.Now().Add(time.Hour),
		}, nil
	}
}

// NewTokenSourceFromIdAndSecret creates a new OAuth 2.0 token source with all the defaults for the
// given scopes, and the given token store filename.
func NewTokenSourceFromIdAndSecret(clientId, clientSecret, oauthCacheFile string, scopes ...string) (oauth2.TokenSource, error) {
	config := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}
	return newLegacyTokenSourceFromConfig(true, config, oauthCacheFile)
}

// newLegacyTokenSourceFromConfig creates an new OAuth 2.0 token source for the given config.
//
// If local is true then a 3-legged flow is initiated, otherwise the GCE Service Account is used if
// running in GCE, and the Skolo access token provider is used if running in Skolo.
func newLegacyTokenSourceFromConfig(local bool, config *oauth2.Config, oauthCacheFile string) (oauth2.TokenSource, error) {
	if oauthCacheFile == "" {
		oauthCacheFile = defaultTokenStoreFilename
	}

	if local {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httputils.DefaultClientConfig().Client())
		return newCachingTokenSource(oauthCacheFile, ctx, config)
	}
	// Are we running on GCE?
	if cloud_metadata.OnGCE() {
		// Use compute engine service account.
		return google.ComputeTokenSource(""), nil
	}
	// Create and use a token provider for skolo service account access tokens.
	return newSkoloTokenSource(), nil
}

const (
	ScopeReadOnly        = storage.DevstorageReadOnlyScope
	ScopeReadWrite       = storage.DevstorageReadWriteScope
	ScopeFullControl     = storage.DevstorageFullControlScope
	ScopeCompute         = compute.ComputeScope
	ScopeGerrit          = "https://www.googleapis.com/auth/gerritcodereview"
	ScopePubsub          = pubsub.ScopePubSub
	ScopeUserinfoEmail   = "https://www.googleapis.com/auth/userinfo.email"
	ScopeUserinfoProfile = "https://www.googleapis.com/auth/userinfo.profile"
	ScopeAllCloudAPIs    = iam.CloudPlatformScope
)

// skoloTokenSource implements the oauth2.TokenSource interface using tokens
// from the skolo metadata server.
type skoloTokenSource struct {
	client *http.Client
}

func newSkoloTokenSource() oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, &skoloTokenSource{
		client: httputils.DefaultClientConfig().With2xxOnly().Client(),
	})
}

func (s *skoloTokenSource) Token() (*oauth2.Token, error) {
	resp, err := s.client.Get(metadata.TOKEN_URL)
	if err != nil {
		sklog.Errorf("Failed to retrieve token:  %s", err)
		return nil, fmt.Errorf("Failed to retrieve token: %s", err)
	}
	defer util.Close(resp.Body)
	type tokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresInSec int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	var res tokenResponse
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

// cachingTokenSource implements the oauth2.TokenSource interface and
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
		filename = defaultJwtFilename
	}
	var body []byte
	jwt, err := metadata.ProjectGet(metadataname)
	if err != nil {
		body, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, skerr.Fmt("Couldn't find JWT via metadata %q or in a local file %q.", metadataname, filename)
		}
		sklog.Infof("Read from file %s", filename)
	} else {
		body = []byte(jwt)
	}
	// TODO(dogben): Ok to add metrics?
	tokenClient := httputils.DefaultClientConfig().Client()
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient)
	jwtConfig, err := google.JWTConfigFromJSON(body, scopes...)
	if err != nil {
		sklog.Errorf("Invalid JWT/JSON for token source: %s", body)
		return nil, skerr.Wrapf(err, "failed to load JWT from JSON. See logs for full detail")
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
