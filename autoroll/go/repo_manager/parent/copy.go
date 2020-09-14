package parent

import (
	"context"
	"net/http"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

// CopyEntry describes a single file or directory which is copied from a Child
// into a Parent. Directories are specified using a trailing "/".
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
	GitilesConfig

	// Copies indicates which files and directories to copy from the
	// Child into the Parent.
	Copies []CopyEntry `json:"copies,omitempty"`
}

// See documentation for util.Validator interface.
func (c CopyConfig) Validate() error {
	if err := c.GitilesConfig.Validate(); err != nil {
		return skerr.Wrap(err)
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
func NewCopy(ctx context.Context, cfg CopyConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, userName, userEmail string, dep child.Child) (*gitilesParent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	getContentsAtRev := func(ctx context.Context, rev *revision.Revision) (map[string]string, error) {
		fs, err := dep.VFS(ctx, rev)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv := map[string]string{}
		for _, cp := range cfg.Copies {
			if err := vfs.Walk(ctx, fs, cp.SrcRelPath, func(fp string, info os.FileInfo, err error) error {
				if err != nil {
					return skerr.Wrap(err)
				}
				if info.IsDir() {
					return nil
				}
				contents, err := vfs.ReadFile(ctx, fs, fp)
				if err != nil {
					return skerr.Wrap(err)
				}
				if !strings.HasPrefix(fp, cp.SrcRelPath) {
					return skerr.Fmt("Path %q does not have expected prefix %q", fp, cp.SrcRelPath)
				}
				parentPath := path.Join(cp.DstRelPath, strings.TrimPrefix(fp, cp.SrcRelPath))
				rv[parentPath] = string(contents)
				return nil
			}); err != nil {
				return nil, skerr.Wrap(err)
			}
		}
		return rv, nil
	}
	getChangesHelper := gitilesFileGetChangesForRollFunc(cfg.DependencyConfig)
	getChangesForRoll := func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, error) {
		changes, err := getChangesHelper(ctx, repo, baseCommit, from, to, rolling)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		before, err := getContentsAtRev(ctx, from)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		after, err := getContentsAtRev(ctx, to)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		filenames := util.StringSet{}
		for f := range before {
			filenames[f] = true
		}
		for f := range after {
			filenames[f] = true
		}
		for f := range filenames {
			if before[f] != after[f] {
				changes[f] = after[f]
			}
		}
		return changes, nil
	}
	return newGitiles(ctx, cfg.GitilesConfig, reg, client, serverURL, getChangesForRoll)
}
