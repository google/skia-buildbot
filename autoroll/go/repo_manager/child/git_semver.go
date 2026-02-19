package child

import (
	"context"
	"fmt"
	"sort"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
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
func NewGitSemVerChild(ctx context.Context, c *config.GitSemVerChildConfig, repo gitiles.GitilesRepo) (*gitSemVerChild, error) {
	parser, err := semver.NewParser(c.VersionRegex)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	c.Gitiles.Branch = "fakefakefake" // Unused but required below.
	gr, err := gitiles_common.NewGitilesRepo(ctx, c.Gitiles, repo)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gitSemVerChild{
		repo:         gr,
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
		versionsStr += fmt.Sprintf("  - %s -> %s\n", version, versionToHash[version.String()])
	}
	sklog.Info(versionsStr)
	hashToVersionsStr := "Hash to version mapping:\n"
	for hash, versions := range hashToVersions {
		hashToVersionsStr += fmt.Sprintf("  - %s -> %v", hash, versions)
	}
	sklog.Info(hashToVersionsStr)

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

	notRolledRevs, err := c.logRevisions(ctx, lastRollRev, tipRev, versions, versionToHash, hashToVersions)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to log revisions")
	}
	return tipRev, notRolledRevs, nil
}

func (c *gitSemVerChild) fixupRevision(rev *revision.Revision, hashToVersions map[string][]*semver.Version) {
	if versions, ok := hashToVersions[rev.Id]; ok {
		// If there are multiple tags pointing to this commit, use the latest.
		rev.Release = versions[0].String()
		// Make the commit message show the version instead of the commit hash.
		rev.Display = rev.Release
		sklog.Infof("fixupRevision(%s): Chose version %s from %v", rev.Id, versions[0], versions)
	} else {
		rev.InvalidReason = "No associated tag matching the configured regex."
		sklog.Infof("fixupRevision(%s): No associated version tag", rev.Id)
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
	sklog.Infof("getTipRevision: versions[0] is %q -> %q", versions[0], versionToHash[versions[0].String()])
	return c.getRevision(ctx, versionToHash[versions[0].String()], hashToVersions)
}

// Download implements Child.
func (c *gitSemVerChild) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	return git_common.Clone(ctx, c.repo.URL(), dest, rev)
}

func (c *gitSemVerChild) logRevisions(ctx context.Context, from, to *revision.Revision, versions []*semver.Version, versionToHash map[string]string, hashToVersions map[string][]*semver.Version) ([]*revision.Revision, error) {
	detailsCache := map[string]*revision.Revision{}

	// getDetails retrieves commit details for the version, making use of a
	// cache for efficiency.
	getDetails := func(version *semver.Version) (*revision.Revision, error) {
		hash := versionToHash[version.String()]
		if details, ok := detailsCache[hash]; ok {
			return details.Copy(), nil
		}
		rev, err := c.repo.GetRevision(ctx, hash)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		detailsCache[hash] = rev
		return rev, nil
	}

	// findNearestVersion returns the version pointing to the given Revision, or
	// the newest version older than it.
	findNearestVersion := func(rev *revision.Revision) (*semver.Version, error) {
		if versions, ok := hashToVersions[rev.Id]; ok {
			return versions[0], nil
		}
		for _, version := range versions {
			details, err := getDetails(version)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if details.Timestamp.Before(rev.Timestamp) {
				return version, nil
			}
		}
		return nil, nil
	}
	fromVersion, err := findNearestVersion(from)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Note: we allow fromVersion to be nil. This would happen if all tagged
	// releases were newer than the from-revision. In that case, we'll just
	// return all of the versions.

	toVersion, err := findNearestVersion(to)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// On the other hand, we don't allow toVersion to be nil, because the log
	// doesn't make sense in that case.
	if toVersion == nil {
		return nil, skerr.Fmt("unable to find a version associated with %q", to.Id)
	}

	// Scan the list of versions, return those between fromVersion and
	// toVersion.
	revs := make([]*revision.Revision, 0, len(versions))
	collecting := false
	for _, version := range versions {
		// Don't include the from-version or anything before it in the results.
		if fromVersion != nil && version.Compare(fromVersion) == 0 {
			break
		}
		if version.Compare(toVersion) == 0 {
			collecting = true
		}
		if collecting {
			rev, err := getDetails(version)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			// TODO(borenet): This duplicates the contents of fixupRevision.
			rev.Release = version.String()
			rev.Display = rev.Release

			revs = append(revs, rev)
		}
	}
	return revs, nil
}

// LogRevisions implements Child.
func (c *gitSemVerChild) LogRevisions(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
	versions, versionToHash, hashToVersions, err := c.getVersions(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return c.logRevisions(ctx, from, to, versions, versionToHash, hashToVersions)
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
