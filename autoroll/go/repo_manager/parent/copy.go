package parent

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// CopyEntry describes a single file which is copied from a Child into a Parent.
type CopyEntry struct {
	SrcRelPath string `json:"srcRelPath"`
	DstRelPath string `json:"dstRelPath"`
}

// See documentation for util.Validator interface.
func (e CopyEntry) Validate() error {
	if e.SrcRelPath == "" {
		return skerr.Fmt("SrcRelPath is required")
	}
	if e.DstRelPath == "" {
		return skerr.Fmt("DstRelPath is required")
	}
	return nil
}

// CopyConfig provides configuration for a Parent which copies the Child
// into itself. It uses a local git checkout and uploads changes to Gerrit.
type CopyConfig struct {
	GitCheckoutGerritConfig

	// VersionFile is the path of the file within the repo which contains
	// the current version of the Child.
	VersionFile string `json:"versionFile"`

	// Copies indicates which files and directories to copy from the
	// Child into the Parent.
	Copies []CopyEntry `json:"copies,omitempty"`
}

// See documentation for util.Validator interface.
func (c CopyConfig) Validate() error {
	if err := c.GitCheckoutGerritConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.VersionFile == "" {
		return skerr.Fmt("VersionFile is required")
	}
	if len(c.Copies) == 0 {
		return skerr.Fmt("Copies are required")
	}
	for _, copy := range c.Copies {
		if err := copy.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// NewCopy returns a Parent implementation which copies the Child into itself.
// It uses a local git checkout and uploads changes to Gerrit.
func NewCopy(ctx context.Context, c CopyConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir string, dep child.Child) (*GitCheckoutParent, error) {
	getLastRollRev := VersionFileGetLastRollRevFunc(c.VersionFile, c.ChildRepo)
	createRoll := func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Create a temporary directory.
		tmp, err := ioutil.TempDir("", "")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		defer util.RemoveAll(tmp)

		// Download the Child into the temporary directory.
		if err := dep.Download(ctx, to, tmp); err != nil {
			return "", skerr.Wrap(err)
		}

		// Perform the copies.
		for _, copy := range c.Copies {
			src := filepath.Join(tmp, copy.SrcRelPath)
			dst := filepath.Join(co.Dir(), copy.DstRelPath)

			// Remove the existing version, if any.
			if _, err := os.Stat(dst); err == nil {
				if _, err := co.Git(ctx, "rm", "-rf", dst); err != nil {
					return "", skerr.Wrap(err)
				}
			}
			// Copy the new version.
			if _, err := exec.RunCwd(ctx, workdir, "cp", "-rT", src, dst); err != nil {
				return "", skerr.Wrap(err)
			}
			if _, err := co.Git(ctx, "add", copy.DstRelPath); err != nil {
				return "", skerr.Wrap(err)
			}
		}

		// Write the new version file.
		versionFile := filepath.Join(co.Dir(), c.VersionFile)
		if err := ioutil.WriteFile(versionFile, []byte(to.Id), os.ModePerm); err != nil {
			return "", skerr.Wrap(err)
		}
		if _, err := co.Git(ctx, "add", c.VersionFile); err != nil {
			return "", skerr.Wrap(err)
		}

		// Commit.
		if _, err := co.Git(ctx, "commit", "-m", commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}
		out, err := co.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(out), nil
	}
	return NewGitCheckoutGerrit(ctx, c.GitCheckoutGerritConfig, reg, client, serverURL, workdir, nil, getLastRollRev, createRoll)
}
