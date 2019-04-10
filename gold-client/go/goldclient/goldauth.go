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
	// AUTH_FILE is the file in the work directory where the auth options are cached.
	AUTH_FILE = "auth_opt.json"
)

type AuthOpt interface {
	Validate() error
	GetHTTPClient() (HTTPClient, error)
	SetDryRun(isDryRun bool)
	GetGoldUploader() (GoldUploader, error)
}

// authOpt implements the AuthOpt interface
type authOpt struct {
	Luci           bool
	ServiceAccount string
	GSUtil         bool

	dryRun bool //unexported, i.e. not saved to JSON
}

// Validate returns a nil error if the authOpt object is valid.
func (a *authOpt) Validate() error {
	if !a.GSUtil && !a.Luci && a.ServiceAccount == "" {
		return skerr.Fmt("No valid authentication method provided.")
	}
	return nil
}

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

func (a *authOpt) GetGoldUploader() (GoldUploader, error) {
	if a.dryRun {
		return &dryRunUploader{}, nil
	}
	if a.Luci || a.ServiceAccount != "" {
		if httpClient, err := a.GetHTTPClient(); err != nil {
			return nil, err
		} else {
			// TODO(kjlubick) Maybe take in context as a paramater here?
			return newHttpUploader(context.TODO(), httpClient.(*http.Client))
		}
	}
	return &gsutilUploader{}, nil
}

func (a *authOpt) SetDryRun(isDryRun bool) {
	a.dryRun = isDryRun
}

func LoadAuthOpt(workDir string) (*authOpt, error) {
	inFile := filepath.Join(workDir, AUTH_FILE)
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

// InitServiceAccountAuth returns an AuthOpt instance that configures a service account file
// to use to generate a TokenSource for authentication with GCP.
func InitServiceAccountAuth(svcAccountFile, workDir string) (*authOpt, error) {
	a := authOpt{ServiceAccount: svcAccountFile}
	outFile := filepath.Join(workDir, AUTH_FILE)
	if err := saveJSONFile(outFile, a); err != nil {
		return nil, skerr.Fmt("Could not write JSON file: %s", err)
	}
	return &a, nil
}

// InitLUCIAuth returns an AuthOpt instance to get auth information from the LUCI context.
func InitLUCIAuth(workDir string) (*authOpt, error) {
	a := authOpt{Luci: true}
	outFile := filepath.Join(workDir, AUTH_FILE)
	if err := saveJSONFile(outFile, a); err != nil {
		return nil, skerr.Fmt("Could not write JSON file: %s", err)
	}
	return &a, nil
}

// InitGSUtil returns an AuthOpt instance which uses gsutil.
// TODO(kjlubick): Local dev only?
func InitGSUtil(workDir string) (*authOpt, error) {
	a := authOpt{GSUtil: true}
	outFile := filepath.Join(workDir, AUTH_FILE)
	if err := saveJSONFile(outFile, a); err != nil {
		return nil, skerr.Fmt("Could not write JSON file: %s", err)
	}
	return &a, nil
}
