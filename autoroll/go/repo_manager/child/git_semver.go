package child

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/semver"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
)

// gitSemVerChild is an implementation of Child which rolls using semantically-
// versioned git tags.
type gitSemVerChild struct {
	repo         *gitiles_common.GitilesRepo
	semVerParser *semver.Parser
}

// NewGitSemVerChild returns an implementation of Child which rolls using
// semantically-versioned git tags.
func NewGitSemVerChild(ctx context.Context, c *config.GitSemVerChildConfig, client *http.Client) (*gitSemVerChild, error) {
	parser, err := semver.NewParser(c.VersionRegex)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	c.Gitiles.Branch = "fakefakefake" // Unused but required below.
	repo, err := gitiles_common.NewGitilesRepo(ctx, c.Gitiles, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gitSemVerChild{
		repo:         repo,
		semVerParser: parser,
	}, nil
}

func (c *gitSemVerChild) getVersions(ctx context.Context) ([]*semver.Version, map[string]string, map[string][]*semver.Version, error) {
	// Obtain the set of git tagToHash.
	tagToHash, err := c.repo.Tags(ctx)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to list tags")
	}

	// Find all semantic versions matching the given regex.
	// Create a reverse mapping of commit hash to tag(s).
	versions := make([]*semver.Version, 0, len(tagToHash))
	versionToHash := make(map[string]string, len(tagToHash))
	hashToVersions := make(map[string][]*semver.Version, len(tagToHash))
	for tag, hash := range tagToHash {
		ver, err := c.semVerParser.Parse(tag)
		if err == semver.ErrNoMatch {
			continue
		} else if err != nil {
			// If the regex matched the string but we couldn't parse it,
			// there's almost certainly a problem with the regex itself.
			// Returning an error here will cause the roller to fail and
			// crash-loop, which is reasonable in this circumstance.
			return nil, nil, nil, skerr.Wrapf(err, "version %q matches regex %q but parsing semantic version failed. The regex is probably incorrect.", tag, c.semVerParser.Regex().String())
		}
		versions = append(versions, ver)
		versionToHash[ver.String()] = hash
		hashToVersions[hash] = append(hashToVersions[hash], ver)
	}
	sort.Sort(sort.Reverse(semver.VersionSlice(versions)))
	for hash := range hashToVersions {
		sort.Sort(sort.Reverse(semver.VersionSlice(hashToVersions[hash])))
	}

	// Log the versions we found for debugging purposes.
	versionsStr := "Found versions:\n"
	for _, version := range versions {
		versionsStr += fmt.Sprintf("  - %s\n", version)
	}
	sklog.Info(versionsStr)

	return versions, versionToHash, hashToVersions, nil
}

// Update implements Child.
func (c *gitSemVerChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	versions, versionToHash, hashToVersions, err := c.getVersions(ctx)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	tipRev, err := c.getTipRevision(ctx, versions, versionToHash, hashToVersions)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to get details for tip revision %q", versions[0].String())
	}

	notRolledRevs, err := c.logRevisions(ctx, lastRollRev, tipRev, hashToVersions)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to log revisions")
	}
	return tipRev, notRolledRevs, nil
}

func (c *gitSemVerChild) fixupRevision(rev *revision.Revision, hashToVersions map[string][]*semver.Version) {
	if versions, ok := hashToVersions[rev.Id]; ok {
		// If there are multiple tags pointing to this commit, use the latest.
		rev.Release = versions[0].String()
	} else {
		rev.InvalidReason = "No associated tag matching the configured regex."
	}
}

func (c *gitSemVerChild) getRevision(ctx context.Context, id string, hashToVersions map[string][]*semver.Version) (*revision.Revision, error) {
	rev, err := c.repo.GetRevision(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	c.fixupRevision(rev, hashToVersions)
	return rev, nil
}

// GetRevision implements Child.
func (c *gitSemVerChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	_, _, hashToVersions, err := c.getVersions(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return c.getRevision(ctx, id, hashToVersions)
}

// GetTipRevision returns a revision.Revision instance associated with the
// most recent version.
func (c *gitSemVerChild) GetTipRevision(ctx context.Context) (*revision.Revision, error) {
	versions, versionToHash, hashToVersions, err := c.getVersions(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return c.getTipRevision(ctx, versions, versionToHash, hashToVersions)
}

func (c *gitSemVerChild) getTipRevision(ctx context.Context, versions []*semver.Version, versionToHash map[string]string, hashToVersions map[string][]*semver.Version) (*revision.Revision, error) {
	return c.getRevision(ctx, versionToHash[versions[0].String()], hashToVersions)
}

// Download implements Child.
func (c *gitSemVerChild) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	return git_common.Clone(ctx, c.repo.URL(), dest, rev)
}

func (c *gitSemVerChild) logRevisions(ctx context.Context, from, to *revision.Revision, hashToVersions map[string][]*semver.Version) ([]*revision.Revision, error) {
	revs, err := c.repo.LogRevisions(ctx, from, to)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	for _, rev := range revs {
		c.fixupRevision(rev, hashToVersions)
	}
	return revs, nil
}

// LogRevisions implements Child.
func (c *gitSemVerChild) LogRevisions(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
	_, _, hashToVersions, err := c.getVersions(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return c.logRevisions(ctx, from, to, hashToVersions)
}

// GetNotSubmittedReason implements Child.
func (c *gitSemVerChild) GetNotSubmittedReason(ctx context.Context, rev *revision.Revision) (string, error) {
	if rev.Release != "" {
		// Anything tagged as a release has been submitted.
		return "", nil
	}
	return git_common.GetNotSubmittedReason(ctx, c.repo, rev.Id, git.MainBranch)
}

// VFS implements Child.
func (c *gitSemVerChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	return c.repo.VFS(ctx, rev)
}

var _ Child = &gitSemVerChild{}
