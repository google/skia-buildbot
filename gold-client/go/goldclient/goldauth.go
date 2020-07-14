package goldclient

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
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
	// any calls to GetGCSUploader
	SetDryRun(isDryRun bool)
	// GetGCSUploader returns an authenticated goldclient.GCSUploader, the interface for
	// uploading to GCS.
	GetGCSUploader() (GCSUploader, error)

	// GetImageDownloader returns an authenticated goldclient.ImageDownloader, the interface for
	// downloading from GCS.
	GetImageDownloader() (ImageDownloader, error)
}

// ImageDownloader implementations provide functions to download images from Gold.
type ImageDownloader interface {
	// DownloadImage returns the bytes belonging to a digest from a given instance.
	DownloadImage(ctx context.Context, goldURL string, digest types.Digest) ([]byte, error)
}

// authOpt implements the AuthOpt interface
type authOpt struct {
	Luci           bool
	ServiceAccount string
	GSUtil         bool

	dryRun bool //unexported, i.e. not saved to JSON
}

// Validate implements the AuthOpt interface.
func (a *authOpt) Validate() error {
	if !a.GSUtil && !a.Luci && a.ServiceAccount == "" {
		return skerr.Fmt("No valid authentication method provided.")
	}
	return nil
}

// GetHTTPClient implements the AuthOpt interface.
func (a *authOpt) GetHTTPClient() (HTTPClient, error) {
	if a.GSUtil {
		return httputils.DefaultClientConfig().Client(), nil
	}
	var tokenSrc oauth2.TokenSource
	if a.Luci {
		var err error
		tokenSrc, err = auth.NewLUCIContextTokenSource(gstorage.ScopeFullControl, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			return nil, skerr.Wrapf(err, "instantiating LUCI auth token source")
		}
	} else {
		var err error
		tokenSrc, err = auth.NewJWTServiceAccountTokenSource("", a.ServiceAccount, gstorage.ScopeFullControl, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			return nil, skerr.Wrapf(err, "instantiating JWT auth token source")
		}
	}

	// Retrieve a token to make sure we can retrieve a token. We assume this is cached
	// inside tokenSrc.
	if _, err := tokenSrc.Token(); err != nil {
		return nil, skerr.Wrapf(err, "retrieving initial auth token")
	}
	return httputils.DefaultClientConfig().WithTokenSource(tokenSrc).Client(), nil
}

// GetGCSUploader implements the AuthOpt interface.
func (a *authOpt) GetGCSUploader() (GCSUploader, error) {
	if a.dryRun {
		return &dryRunImpl{}, nil
	}
	if a.Luci || a.ServiceAccount != "" {
		return a.httpGCSImpl()
	}
	return &gsutilImpl{}, nil
}

// GetImageDownloader implements the AuthOpt interface.
func (a *authOpt) GetImageDownloader() (ImageDownloader, error) {
	if a.dryRun {
		return &dryRunImpl{}, nil
	}
	hc, err := a.GetHTTPClient()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &httpImageDownloader{
		httpClient: hc,
	}, nil
}

func (a *authOpt) httpGCSImpl() (*clientImpl, error) {
	if httpClient, err := a.GetHTTPClient(); err != nil {
		return nil, err
	} else {
		hc, ok := httpClient.(*http.Client)
		if !ok {
			// Should never happen, but is easier to debug than a panic
			return nil, skerr.Fmt("HTTPClient was wrong type: %#v", httpClient)
		}
		// TODO(kjlubick) Maybe take in context as a parameter here?
		return newGCSClient(context.TODO(), hc)
	}
}

// SetDryRun implements the AuthOpt interface.
func (a *authOpt) SetDryRun(isDryRun bool) {
	a.dryRun = isDryRun
}

// httpImageDownloader implements the ImageDownloaderAPI by downloading images over HTTP.
type httpImageDownloader struct {
	httpClient HTTPClient
}

// DownloadImage implements the ImageDownloader API. It downloads the image from the instance
// (not from GCS itself), which removes the need for the service account to have read access.
func (h *httpImageDownloader) DownloadImage(_ context.Context, goldURL string, digest types.Digest) ([]byte, error) {
	u := fmt.Sprintf("%s/img/images/%s.png", goldURL, digest)
	resp, err := h.httpClient.Get(u)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting digest from url %s", u)
	}
	defer util.Close(resp.Body)
	return ioutil.ReadAll(resp.Body)
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
		return skerr.Wrapf(err, "writing to JSON file %s", outFile)
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
