package goldclient

import (
	"context"
	"net/http"
	"path/filepath"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
)

const (
	// authFile is the file in the work directory where the auth options are cached.
	authFile = "auth_opt.json"
)

// The AuthOpt interface adds a layer of abstraction around getting authenticated
// clients that can make certain requests over the wire.
// This being an interface makes for easier mocking than just the raw struct.
type AuthOpt interface {
	// Validate returns an error if this is not a valid authenticated interface nil, otherwise.
	Validate() error
	// GetHTTPClient returns an authenticated goldclient.HTTPClient (which for non-mocked
	// implementations will be an http.Client)
	GetHTTPClient() (HTTPClient, error)
	// SetDryRun will toggle actually uploading to GCS or not. This should be set before
	// any calls to GetGoldUploader
	SetDryRun(isDryRun bool)
	// GetGoldUploader returns an authenticated goldclient.GoldUploader, the interface for
	// uploading to GCS.
	GetGoldUploader() (GoldUploader, error)
}

// authOpt implements the AuthOpt interface
type authOpt struct {
	Luci           bool
	ServiceAccount string
	GSUtil         bool

	dryRun bool //unexported, i.e. not saved to JSON
}

// Implements the AuthOpt interface.
func (a *authOpt) Validate() error {
	if !a.GSUtil && !a.Luci && a.ServiceAccount == "" {
		return skerr.Fmt("No valid authentication method provided.")
	}
	return nil
}

// Implements the AuthOpt interface.
func (a *authOpt) GetHTTPClient() (HTTPClient, error) {
	if a.GSUtil {
		return httputils.DefaultClientConfig().Client(), nil
	}
	var tokenSrc oauth2.TokenSource
	var err error
	if a.Luci {
		tokenSrc, err = auth.NewLUCIContextTokenSource(gstorage.ScopeFullControl)
	} else {
		tokenSrc, err = auth.NewJWTServiceAccountTokenSource("#bogus", a.ServiceAccount, gstorage.ScopeFullControl)
	}
	if err != nil {
		return nil, skerr.Fmt("Unable to instantiate auth token source: %s", err)
	}

	// Retrieve a token to make sure we can retrieve a token. We assume this is cached
	// inside tokenSrc.
	if _, err := tokenSrc.Token(); err != nil {
		return nil, skerr.Fmt("Error retrieving initial auth token: %s", err)
	}
	return httputils.DefaultClientConfig().WithTokenSource(tokenSrc).Client(), nil
}

// Implements the AuthOpt interface.
func (a *authOpt) GetGoldUploader() (GoldUploader, error) {
	if a.dryRun {
		return &dryRunUploader{}, nil
	}
	if a.Luci || a.ServiceAccount != "" {
		if httpClient, err := a.GetHTTPClient(); err != nil {
			return nil, err
		} else {
			// TODO(kjlubick) Maybe take in context as a parameter here?
			return newHttpUploader(context.TODO(), httpClient.(*http.Client))
		}
	}
	return &gsutilUploader{}, nil
}

// Implements the AuthOpt interface.
func (a *authOpt) SetDryRun(isDryRun bool) {
	a.dryRun = isDryRun
}

// LoadAuthOpt will load a serialized *authOpt from disk and return it.
// If there is not one, it will return nil.
func LoadAuthOpt(workDir string) (*authOpt, error) {
	inFile := filepath.Join(workDir, authFile)
	ret := &authOpt{}
	found, err := loadJSONFile(inFile, &ret)
	if err != nil {
		return nil, skerr.Fmt("Unexpected error loading existing auth: %s", err)
	}

	if found {
		return ret, nil
	}
	return nil, nil
}

// InitServiceAccountAuth instantiates a workDir to be authenticated with the given
// serviceAccountFile.
func InitServiceAccountAuth(svcAccountFile, workDir string) error {
	a := authOpt{ServiceAccount: svcAccountFile}
	outFile := filepath.Join(workDir, authFile)
	if err := saveJSONFile(outFile, a); err != nil {
		return skerr.Fmt("Could not write JSON file: %s", err)
	}
	return nil
}

// InitLUCIAuth instantiates a workDir to be authenticated with LUCI
// credentials on this machine.
func InitLUCIAuth(workDir string) error {
	a := authOpt{Luci: true}
	outFile := filepath.Join(workDir, authFile)
	if err := saveJSONFile(outFile, a); err != nil {
		return skerr.Fmt("Could not write JSON file: %s", err)
	}
	return nil
}

// InitGSUtil instantiates a workDir to be authenticated with gsutil
// This is primarily used for local testing, and should not be relied
// upon for production usage.
func InitGSUtil(workDir string) error {
	a := authOpt{GSUtil: true}
	outFile := filepath.Join(workDir, authFile)
	if err := saveJSONFile(outFile, a); err != nil {
		return skerr.Fmt("Could not write JSON file: %s", err)
	}
	return nil
}
