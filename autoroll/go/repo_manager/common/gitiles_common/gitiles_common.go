package gitiles_common

import (
	"context"
	"fmt"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vfs"
	gitiles_vfs "go.skia.org/infra/go/vfs/gitiles"
)

// GitilesRepo provides helpers for dealing with repos which use Gitiles.
type GitilesRepo struct {
	gitiles.GitilesRepo
	branch            string
	defaultBugProject string
	deps              []*config.VersionFileConfig
}

// NewGitilesRepo returns a GitilesRepo instance.
func NewGitilesRepo(ctx context.Context, c *config.GitilesConfig, repo gitiles.GitilesRepo) (*GitilesRepo, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitilesRepo{
		GitilesRepo:       repo,
		branch:            c.Branch,
		deps:              c.Dependencies,
		defaultBugProject: c.DefaultBugProject,
	}, nil
}

// Branch returns the name of the branch tracked by this GitilesRepo.
func (r *GitilesRepo) Branch() string {
	return r.branch
}

// GetRevision returns a revision.Revision instance associated with the given
// revision ID, which may be a commit hash or fully-qualified ref name.
func (r *GitilesRepo) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	// Load the details for this revision.
	details, err := r.GitilesRepo.Details(ctx, id)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to retrieve revision %q", id)
	}
	return r.getRevisionHelper(ctx, details)
}

func (r *GitilesRepo) getRevisionHelper(ctx context.Context, details *vcsinfo.LongCommit) (*revision.Revision, error) {
	revLinkTmpl := fmt.Sprintf(gitiles.CommitURL, r.GitilesRepo.URL(), "%s")
	rev := revision.FromLongCommit(revLinkTmpl, r.defaultBugProject, details)
	deps, err := r.GetPinnedRevs(ctx, rev.Id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rev.Dependencies = deps
	return rev, nil
}

// GetPinnedRevs loads any dependencies pinned by the given commit, if any are
// configured. Otherwise returns nil.
func (r *GitilesRepo) GetPinnedRevs(ctx context.Context, commit string) (map[string]string, error) {
	if len(r.deps) > 0 {
		rv, _, err := version_file_common.GetPinnedRevs(ctx, r.deps, func(ctx context.Context, path string) (string, error) {
			return r.GetFile(ctx, path, commit)
		})
		return rv, skerr.Wrap(err)
	}
	return nil, nil
}

// LogRevisions implements the child.Child interface.
func (r *GitilesRepo) LogRevisions(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
	commits, err := r.LogFirstParent(ctx, from.Id, to.Id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	revs, err := r.ConvertRevisions(ctx, commits)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return revs, nil
}

// GetTipRevision returns a revision.Revision instance associated with the
// current tip of the branch tracked by this GitilesRepo.
func (r *GitilesRepo) GetTipRevision(ctx context.Context) (*revision.Revision, error) {
	return r.GetRevision(ctx, r.branch)
}

// ConvertRevisions converts the given slice of LongCommits to Revisions.
func (r *GitilesRepo) ConvertRevisions(ctx context.Context, commits []*vcsinfo.LongCommit) ([]*revision.Revision, error) {
	revs := make([]*revision.Revision, 0, len(commits))
	for _, commit := range commits {
		rev, err := r.getRevisionHelper(ctx, commit)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to retrieve revision")
		}
		revs = append(revs, rev)
	}
	return revs, nil
}

// GetFile retrieves the contents of the given file at the given ref.
func (r *GitilesRepo) GetFile(ctx context.Context, file, ref string) (string, error) {
	contents, err := r.ReadFileAtRef(ctx, file, ref)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return string(contents), nil
}

// VFS implements the child.Child interface.
func (r *GitilesRepo) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	return gitiles_vfs.New(ctx, r.GitilesRepo, rev.Id)
}
