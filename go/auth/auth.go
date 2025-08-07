package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	defaultJwtFilename          = "service-account.json"
	defaultClientSecretFilename = "client_secret.json"
	defaultTokenStoreFilename   = "google_storage_token.data"
)

type gcloudTokenSource struct {
	projectId string
}

// NewGCloudTokenSource creates an oauth2.TokenSource that returns tokens from
// the locally authorized gcloud command line tool, i.e. it gets them from
// running:
//
//	gcloud auth print-access-token
//
// projectId - The name of the GCP project, e.g. 'skia-public'. If empty, "", then
//
//	the default project id for gcloud is used.
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

// getJWT attempts to retrieve JWT JSON Service Account data from the following
// sources, in order:
//
// * GCE metadata
// * Local file
// * GCP secrets
func getJWT(ctx context.Context, metadataName, fileName, secretProject, secretName string) ([]byte, error) {
	if metadataName == "" {
		metadataName = metadata.JWT_SERVICE_ACCOUNT
	}
	if fileName == "" {
		fileName = defaultJwtFilename
	}
	jwt, err := metadata.ProjectGet(metadataName)
	if err == nil {
		sklog.Infof("Read JWT from metadata %s", metadataName)
		return []byte(jwt), nil
	}
	body, err := os.ReadFile(fileName)
	if err == nil {
		sklog.Infof("Read JWT from file %s", fileName)
		return body, nil
	}
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed creating secret client after failing to retrieve JWT via metadata %q and file %q", metadataName, fileName)
	}
	s, err := secretClient.Get(ctx, secretProject, secretName, secret.VersionLatest)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed retrieving secret %q from project %q after failing to retrieve JWT via metadata %q and file %q", secretName, secretProject, metadataName, fileName)
	}
	return []byte(s), nil
}

// NewJWTServiceAccountTokenSource creates a new oauth2.TokenSource that
// is loaded first by attempting to load JWT JSON Service Account data from GCE
// Project Level metadata, and if that fails falls back to loading the data
// from a local file, followed by GCP secrets if the local file fails.
//
//	metadataname - The name of the GCE project level metadata key that holds the JWT JSON. If empty a default is used.
//	filename - The name of the local file that holds the JWT JSON. If empty a default is used.
//	secretProject - The GCP project containing the GCP secret which holds the JWT JSON.
//	secretName - The name of the GCP secret which holds the JWT JSON.
func NewJWTServiceAccountTokenSource(ctx context.Context, metadataname, filename, secretProject, secretName string, scopes ...string) (oauth2.TokenSource, error) {
	body, err := getJWT(ctx, metadataname, filename, secretProject, secretName)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(dogben): Ok to add metrics?
	tokenClient := httputils.DefaultClientConfig().Client()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, tokenClient)
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
func NewDefaultJWTServiceAccountTokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	return NewJWTServiceAccountTokenSource(ctx, "", "", "", "", scopes...)
}

// keyFormat is used to extract some information from a JSON encoded service
// account key for the sake of logging only.
type keyFormat struct {
	ClientEmail  string `json:"client_email"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	TokenURL     string `json:"token_uri"`
	ProjectID    string `json:"project_id"`
	ClientSecret string `json:"client_secret"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

// NewTokenSourceFromKeyString creates a TokenSource from the given
// 'keyAsBase64String' for the given 'scopes'.
//
// The value of 'keyAsBase64String' is a JSON service account key encoded in
// base64.
//
// This function can be used with public variables declared in a module and the
// value of the Key can be changed via -ldflags to pass an -X flag to the
// linker, for example
//
//	go build \
//	-ldflags="-X 'main.Key=${SERVICE_ACCOUNT_KEY_IN_BASE64}' " \
//	./go/foo
func NewTokenSourceFromKeyString(ctx context.Context, local bool, keyAsBase64String string, scopes ...string) (oauth2.TokenSource, error) {
	if local {
		return google.DefaultTokenSource(ctx, scopes...)
	}

	decodedKey, err := base64.StdEncoding.DecodeString(keyAsBase64String)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to base64 decode Key: %q", keyAsBase64String)
	}

	// Unmarshal Key so that we can log some of its values.
	var key keyFormat
	if err := json.Unmarshal([]byte(decodedKey), &key); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse Key as JSON")
	}
	sklog.Infof("client_email: %s", key.ClientEmail)
	sklog.Infof("client_id: %s", key.ClientID)
	sklog.Infof("private_key_id: %s", key.PrivateKeyID)
	sklog.Infof("project_id: %s", key.ProjectID)

	cred, err := google.CredentialsFromJSON(ctx, []byte(decodedKey), scopes...)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create token source")
	}
	return cred.TokenSource, nil
}

func PeriodicallyLogTokenExpiration(ctx context.Context, freq time.Duration, ts oauth2.TokenSource) {
	util.RepeatCtx(ctx, freq, func(ctx context.Context) {
		tok, err := ts.Token()
		if err != nil {
			sklog.Errorf("[PeriodicallyLogTokenExpiration] Failed to obtain auth token: %s", err)
		} else {
			sklog.Infof("[PeriodicallyLogTokenExpiration] Token expires at %s", tok.Expiry)
		}
	})
}
