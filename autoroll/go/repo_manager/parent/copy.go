package parent

import (
	"context"
	"net/http"
	"os"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
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
func NewCopy(ctx context.Context, cfg CopyConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, userName, userEmail string, dep child.Child, uploadRoll git_common.UploadRollFunc) (*gitilesParent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	getContentsAtRev := func(ctx context.Context, rev *revision.Revision) (map[string]string, error) {
		rv := map[string]string{}
		for _, cp := range cfg.Copies {
			if err := child.Walk(ctx, dep, rev, cp.SrcRelPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return skerr.Wrap(err)
				}
				if info.IsDir() {
					return nil
				}
				contents, err := dep.ReadFile(ctx, rev, path)
				if err != nil {
					return skerr.Wrap(err)
				}
				rv[path] = contents
				return nil
			}); err != nil {
				return nil, skerr.Wrap(err)
			}
		}
		return rv, nil
	}
	getChangesForRoll := func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, error) {
		before, err := getContentsAtRev(ctx, from)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		after, err := getContentsAtRev(ctx, to)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

	}
	return newGitiles(ctx, cfg.GitilesConfig, reg, client, serverURL, getChangesForRoll)
}

// CopyRevisionFilter is a RevisionFilter which excludes Revisions which do not
// modify the given files or directories.
type CopyRevisionFilter struct {
	dirs  []string
	files map[string]bool
}

// NewCopyRevisionFilter returns a RevisionFilter which excludes Revisions which
// do not modify the given paths. Directories are specified with a trailing "/".
func NewCopyRevisionFilter(parentCfg CopyConfig) *CopyRevisionFilter {
	var dirs []string
	files := map[string]bool{}
	for _, path := range parentCfg.Copies {
		if strings.HasSuffix(path.SrcRelPath, "/") {
			dirs = append(dirs, path.SrcRelPath)
		} else {
			files[path.SrcRelPath] = true
		}
	}
	return &CopyRevisionFilter{
		dirs:  dirs,
		files: files,
	}
}

// Skip implements RevisionFilter.
func (rf *CopyRevisionFilter) Skip(ctx context.Context, rev *revision.Revision) (string, error) {
	for _, mod := range rev.Modifications {
		if rf.files[mod] {
			return "", nil
		}
		for _, dir := range rf.dirs {
			// Because directories are specified using the "/" suffix, we don't
			// need to worry about mistakenly matching files which have the same
			// prefix, eg. "src/foobar" matched by "src/foo/".
			if strings.HasPrefix(mod, dir) {
				return "", nil
			}
		}
	}
	return "No watched files were modified by this revision.", nil
}

var _ revision_filter.RevisionFilter = &CopyRevisionFilter{}
