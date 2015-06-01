package gitiles

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

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
	URL string
}

// NewRepo creates and returns a new Repo object.
func NewRepo(url string) *Repo {
	return &Repo{
		URL: url,
	}
}

// ReadFile reads the current version of the given file from the master branch
// of the Repo.
func (r *Repo) ReadFile(srcPath string, w io.Writer) error {
	resp, err := util.NewTimeoutClient().Get(fmt.Sprintf(DOWNLOAD_URL, r.URL, "master", srcPath))
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	d := base64.NewDecoder(base64.StdEncoding, resp.Body)
	if _, err := io.Copy(w, d); err != nil {
		return err
	}
	return nil
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
