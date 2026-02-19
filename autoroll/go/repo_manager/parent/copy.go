package parent

import (
	"context"
	"os"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

// NewCopy returns a Parent implementation which copies the Child into itself.
// It uses a local git checkout and uploads changes to Gerrit.
func NewCopy(ctx context.Context, cfg *config.CopyParentConfig, repo gitiles.GitilesRepo, gerrit gerrit.GerritInterface, serverURL string, dep child.Child) (*gitilesParent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	getChangesHelper := gitilesFileGetChangesForRollFunc(cfg.Gitiles.Dep)
	var p *gitilesParent
	getChangesForRoll := func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, error) {
		changes, err := getChangesHelper(ctx, repo, baseCommit, from, to, rolling)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		parentFs, err := p.VFS(ctx, &revision.Revision{Id: baseCommit})
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		childFs, err := dep.VFS(ctx, to)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		copies, err := copyParentGetCopies(ctx, cfg.Copies, parentFs, childFs)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for path, contents := range copies {
			changes[path] = contents
		}
		return changes, nil
	}
	var err error
	p, err = newGitiles(ctx, cfg.Gitiles, repo, gerrit, serverURL, getChangesForRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return p, nil
}

func copyParentGetCopies(ctx context.Context, copies []*config.CopyParentConfig_CopyEntry, parentFs, childFs vfs.FS) (map[string]string, error) {
	oldContents := map[string]string{}
	newContents := map[string]string{}
	for _, cp := range copies {
		// Get the existing contents in the parent repo.
		oldParentContents, err := copyParentGetFileContents(ctx, parentFs, cp.DstRelPath)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for path, contents := range oldParentContents {
			oldContents[path] = contents
		}

		// Get the updated contents from the child repo.
		childContents, err := copyParentGetFileContents(ctx, childFs, cp.SrcRelPath)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		// Map the child repo path to the parent repo path to set the
		// updated contents in the parent repo.
		for childPath, childContents := range childContents {
			parentPath := cp.DstRelPath + strings.TrimPrefix(childPath, cp.SrcRelPath)
			newContents[parentPath] = childContents
		}

	}

	// Determine what needs to go into the CL. Scanning both maps ensures
	// that we handle added and deleted files.
	filenames := util.StringSet{}
	for f := range oldContents {
		filenames[f] = true
	}
	for f := range newContents {
		filenames[f] = true
	}
	changes := map[string]string{}
	for f := range filenames {
		if oldContents[f] != newContents[f] {
			changes[f] = newContents[f]
		}
	}
	return changes, nil
}

// copyParentGetFileContents returns a mapping of file paths to their contents
// within the given vfs.FS. The given path may be a full file path, in which
// case only the contents of that file are returned, or a directory, in which
// case the contents of all files within that directory (recursively) are
// returned.
func copyParentGetFileContents(ctx context.Context, fs vfs.FS, path string) (map[string]string, error) {
	rv := map[string]string{}
	if err := vfs.Walk(ctx, fs, path, func(fp string, info os.FileInfo, err error) error {
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
		if !strings.HasPrefix(fp, path) {
			return skerr.Fmt("Path %q does not have expected prefix %q", fp, path)
		}
		rv[fp] = string(contents)
		return nil
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}
