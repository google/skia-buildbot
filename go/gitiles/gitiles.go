package gitiles

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

/*
	Utilities for working with Gitiles.
*/

const (
	DOWNLOAD_URL = "%s/+/%s/%s?format=TEXT"
)

// Repo is an object used for interacting with a single Git repo using Gitiles.
type Repo struct {
	client *http.Client
	URL    string
}

// NewRepo creates and returns a new Repo object.
func NewRepo(url string) *Repo {
	c := httputils.NewTimeoutClient()
	httputils.AddMetricsToClient(c)
	return &Repo{
		client: c,
		URL:    url,
	}
}

// ReadFileAtRef reads the given file at the given ref.
func (r *Repo) ReadFileAtRef(srcPath, ref string, w io.Writer) error {
	resp, err := r.client.Get(fmt.Sprintf(DOWNLOAD_URL, r.URL, ref, srcPath))
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request got status %q", resp.Status)
	}
	d := base64.NewDecoder(base64.StdEncoding, resp.Body)
	if _, err := io.Copy(w, d); err != nil {
		return err
	}
	return nil
}

// ReadFile reads the current version of the given file from the master branch
// of the Repo.
func (r *Repo) ReadFile(srcPath string, w io.Writer) error {
	return r.ReadFileAtRef(srcPath, "master", w)
}

// DownloadFile downloads the current version of the given file from the master
// branch of the Repo.
func (r *Repo) DownloadFile(srcPath, dstPath string) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer util.Close(f)
	if err := r.ReadFile(srcPath, f); err != nil {
		return err
	}
	return nil
}
