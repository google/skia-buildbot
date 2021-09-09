package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"

	gstorage "cloud.google.com/go/storage"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/luciauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/gold-client/go/gcsuploader"
	"go.skia.org/infra/gold-client/go/httpclient"
	"go.skia.org/infra/gold-client/go/imagedownloader"
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
	GetHTTPClient() (httpclient.HTTPClient, error)
	// SetDryRun will toggle actually uploading to GCS or not. This should be set before
	// any calls to GetGCSUploader
	SetDryRun(isDryRun bool)
	// GetGCSUploader returns an authenticated goldclient.GCSUploader, the interface for
	// uploading to GCS.
	GetGCSUploader(context.Context) (gcsuploader.GCSUploader, error)

	// GetImageDownloader returns an authenticated goldclient.ImageDownloader, the interface for
	// downloading from GCS.
	GetImageDownloader() (imagedownloader.ImageDownloader, error)
}

// authOpt implements the AuthOpt interface
type authOpt struct {
	Luci           bool
	ServiceAccount string
	GSUtil         bool

	dryRun bool // unexported, i.e. not saved to JSON
}

// Validate implements the AuthOpt interface.
func (a *authOpt) Validate() error {
	if !a.GSUtil && !a.Luci && a.ServiceAccount == "" {
		return skerr.Fmt("No valid authentication method provided.")
	}
	return nil
}

// GetHTTPClient implements the AuthOpt interface.
func (a *authOpt) GetHTTPClient() (httpclient.HTTPClient, error) {
	if a.GSUtil {
		return httputils.DefaultClientConfig().WithoutRetries().Client(), nil
	}
	var tokenSrc oauth2.TokenSource
	if a.Luci {
		var err error
		tokenSrc, err = luciauth.NewLUCIContextTokenSource(gstorage.ScopeFullControl, auth.ScopeUserinfoEmail)
		if err != nil {
			return nil, skerr.Wrapf(err, "instantiating LUCI auth token source")
		}
	} else {
		var err error
		tokenSrc, err = auth.NewJWTServiceAccountTokenSource("", a.ServiceAccount, gstorage.ScopeFullControl, auth.ScopeUserinfoEmail)
		if err != nil {
			return nil, skerr.Wrapf(err, "instantiating JWT auth token source")
		}
	}

	// Retrieve a token to make sure we can retrieve a token. We assume this is cached
	// inside tokenSrc.
	if _, err := tokenSrc.Token(); err != nil {
		return nil, skerr.Wrapf(err, "retrieving initial auth token")
	}
	return httputils.DefaultClientConfig().WithoutRetries().WithTokenSource(tokenSrc).Client(), nil
}

// GetGCSUploader implements the AuthOpt interface.
func (a *authOpt) GetGCSUploader(ctx context.Context) (gcsuploader.GCSUploader, error) {
	if a.dryRun {
		return &gcsuploader.DryRunImpl{}, nil
	}
	if a.Luci || a.ServiceAccount != "" {
		return a.httpGCSImpl(ctx)
	}
	return &gcsuploader.GsutilImpl{}, nil
}

// GetImageDownloader implements the AuthOpt interface.
func (a *authOpt) GetImageDownloader() (imagedownloader.ImageDownloader, error) {
	if a.dryRun {
		return &imagedownloader.DryRunImpl{}, nil
	}
	hc, err := a.GetHTTPClient()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return imagedownloader.New(hc), nil
}

func (a *authOpt) httpGCSImpl(ctx context.Context) (gcsuploader.GCSUploader, error) {
	if httpClient, err := a.GetHTTPClient(); err != nil {
		return nil, err
	} else {
		hc, ok := httpClient.(*http.Client)
		if !ok {
			// Should never happen, but is easier to debug than a panic
			return nil, skerr.Fmt("HTTPClient was wrong type: %#v", httpClient)
		}
		return gcsuploader.New(ctx, hc)
	}
}

// SetDryRun implements the AuthOpt interface.
func (a *authOpt) SetDryRun(isDryRun bool) {
	a.dryRun = isDryRun
}

func (a *authOpt) writeToDisk(workDir string) error {
	outFile := filepath.Join(workDir, authFile)
	err := util.WithWriteFile(outFile, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(a)
	})
	if err != nil {
		return skerr.Wrapf(err, "writing/serializing to JSON file %s", outFile)
	}
	return nil
}

// LoadAuthOpt will load a serialized *authOpt from disk and return it.
// If there is not one, it will return nil.
func LoadAuthOpt(workDir string) (*authOpt, error) {
	aFile := filepath.Join(workDir, authFile)
	if !fileutil.FileExists(aFile) {
		return nil, skerr.Fmt("File %s does not exist", aFile)
	}

	ret := &authOpt{}
	err := util.WithReadFile(aFile, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&ret)
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "reading/parsing JSON file: %s", aFile)
	}

	return ret, nil
}

// InitServiceAccountAuth instantiates a workDir to be authenticated with the given
// serviceAccountFile.
func InitServiceAccountAuth(svcAccountFile, workDir string) error {
	a := authOpt{ServiceAccount: svcAccountFile}
	if err := a.writeToDisk(workDir); err != nil {
		return skerr.Wrapf(err, "writing to work dir: %s", workDir)
	}
	return nil
}

// InitLUCIAuth instantiates a workDir to be authenticated with LUCI
// credentials on this machine.
func InitLUCIAuth(workDir string) error {
	a := authOpt{Luci: true}
	if err := a.writeToDisk(workDir); err != nil {
		return skerr.Wrapf(err, "writing to work dir: %s", workDir)
	}
	return nil
}

// InitGSUtil instantiates a workDir to be authenticated with gsutil
// This is primarily used for local testing, and should not be relied
// upon for production usage.
func InitGSUtil(workDir string) error {
	a := authOpt{GSUtil: true}
	if err := a.writeToDisk(workDir); err != nil {
		return skerr.Wrapf(err, "writing to work dir: %s", workDir)
	}
	return nil
}
