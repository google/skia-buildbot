package child

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/client/cipd/pkg"
	"go.chromium.org/luci/cipd/common"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

const (
	cipdPackageUrlTmpl  = "%s/p/%s/+/%s"
	cipdBuganizerPrefix = "b/"
	cipdGitRevisionTag  = "git_revision"
)

var (
	cipdDetailsRegex = regexp.MustCompile(`details(\d+)`)
)

// NewCIPD returns an implementation of Child which deals with a CIPD package.
// If the caller calls CIPDChild.Download, the destination must be a descendant of
// the provided workdir.
func NewCIPD(ctx context.Context, c *config.CIPDChildConfig, cipdClient cipd.CIPDClient, repo gitiles.GitilesRepo, workdir string) (*CIPDChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	gitilesConfig := c.SourceRepo
	if gitilesConfig == nil && c.GitilesRepo != "" {
		gitilesConfig = &config.GitilesConfig{
			Branch:  "branch is unused",
			RepoUrl: c.GitilesRepo,
		}
	}
	var gitilesRepo *gitiles_common.GitilesRepo
	var err error
	if gitilesConfig != nil {
		gitilesRepo, err = gitiles_common.NewGitilesRepo(ctx, gitilesConfig, repo)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return &CIPDChild{
		client:                cipdClient,
		name:                  c.Name,
		root:                  workdir,
		ref:                   c.Tag,
		gitRepo:               gitilesRepo,
		revisionIdTag:         c.RevisionIdTag,
		revisionIdTagStripKey: c.RevisionIdTagStripKey,
		platforms:             c.Platform,
	}, nil
}

// CIPDChild is an implementation of Child which deals with a CIPD package.
type CIPDChild struct {
	client                cipd.CIPDClient
	name                  string
	root                  string
	ref                   string
	gitRepo               *gitiles_common.GitilesRepo
	revisionIdTag         string
	revisionIdTagStripKey bool
	platforms             []string
}

// GetRevision implements Child.
func (c *CIPDChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	pkgNames := []string{c.name}
	if len(c.platforms) > 0 {
		pkgNames = make([]string, 0, len(c.platforms))
		for _, platform := range c.platforms {
			pkgNames = append(pkgNames, strings.ReplaceAll(c.name, cipd.PlatformPlaceholder, platform))
		}
	}
	instances := make([]*cipd_api.InstanceDescription, 0, len(pkgNames))
	var invalidReason string
	for _, pkgName := range pkgNames {
		instance, err := c.findInstance(ctx, pkgName, id)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if instance == nil {
			invalidReason = fmt.Sprintf("no package instance exists for %q with version %q", pkgName, id)
		}
		// Always keep the instance, even if nil. This simplifies handling of
		// Meta below.
		instances = append(instances, instance)
	}
	if len(instances) == 0 {
		return nil, skerr.Fmt("failed to find instances matching %q", id)
	}
	rev, err := CIPDInstanceToRevision(pkgNames[0], instances[0], c.revisionIdTag, c.revisionIdTagStripKey)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.gitRepo != nil {
		gitRevision := getGitRevisionFromCIPDInstance(instances[0])
		if gitRevision == "" {
			rev.InvalidReason = "No git_revision tag"
		} else {
			oldChecksum := rev.Checksum
			rev, err = c.gitRepo.GetRevision(ctx, gitRevision)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			rev.Id = fmt.Sprintf("%s:%s", cipdGitRevisionTag, gitRevision)
			rev.Checksum = oldChecksum
		}
	}

	// Set the Meta field, if applicable.
	if len(c.platforms) > 0 {
		rev.Meta = make(map[string]string, len(instances))
		for idx, platform := range c.platforms {
			instance := instances[idx]
			if instance != nil {
				sha256, err := cipd.InstanceIDToSha256(instance.Pin.InstanceID)
				if err != nil {
					return nil, skerr.Wrap(err)
				}
				rev.Meta[platform] = sha256
			}
		}
	}
	rev.InvalidReason = invalidReason
	return rev, nil
}

func (c *CIPDChild) findInstance(ctx context.Context, pkgName, id string) (*cipd_api.InstanceDescription, error) {
	if id == c.ref {
		pin, err := c.client.ResolveVersion(ctx, pkgName, c.ref)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		id = pin.InstanceID
	}
	instance, err := c.client.Describe(ctx, pkgName, id, false)
	if err == nil {
		return instance, nil
	}
	sklog.Warningf("Failed to Describe instance %q of package %q; falling back to search: %s", id, pkgName, err)
	tag := id
	if c.revisionIdTag != "" && c.revisionIdTagStripKey {
		tag = joinCIPDTag(c.revisionIdTag, id)
	}
	pins, err2 := c.client.SearchInstances(ctx, pkgName, []string{tag})
	if err2 != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve revision ID %q and failed to search by tag %q with: %s", id, tag, err2)
	}
	if len(pins) == 0 {
		return nil, nil
	} else if len(pins) > 1 {
		sklog.Errorf("Found more than one matching instance for tag %q; arbitrarily returning the first.", tag)
	}
	instance, err = c.client.Describe(ctx, pkgName, pins[0].InstanceID, false)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return instance, nil
}

// LogRevisions implements Child.
func (c *CIPDChild) LogRevisions(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
	if c.gitRepo != nil {
		// Obtain the git revisions from the backing repo.
		_, fromHash, err := splitCIPDTag(from.Id)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		_, toHash, err := splitCIPDTag(to.Id)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		revs, err := c.gitRepo.LogRevisions(ctx, &revision.Revision{Id: fromHash}, &revision.Revision{Id: toHash})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for _, rev := range revs {
			// Fix the IDs to be CIPD tags rather than Git commit hashes.
			rev.Id = fmt.Sprintf("%s:%s", cipdGitRevisionTag, rev.Id)

			// Make in-between revisions invalid, since we only have CIPD
			// package instances associated with from and to.
			if rev.Id == from.Id {
				rev.Checksum = from.Checksum
			} else if rev.Id == to.Id {
				rev.Checksum = to.Checksum
			} else {
				rev.Checksum = ""
				rev.InvalidReason = "No associated CIPD package."
			}
		}
		return revs, nil
	}
	revs := []*revision.Revision{}
	if from.Id != to.Id {
		revs = append(revs, to)
	}
	return revs, nil
}

// Update implements Child.
// Note: that this just finds the newest version of the CIPD package.
func (c *CIPDChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, err := c.GetRevision(ctx, c.ref)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	notRolledRevs, err := c.LogRevisions(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return tipRev, notRolledRevs, nil
}

// VFS implements the Child interface.
func (c *CIPDChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	fs, err := vfs.TempDir(ctx, c.root, "tmp")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	dest, err := filepath.Rel(c.root, fs.Dir())
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if _, err := c.client.EnsurePackages(ctx, common.PinSliceBySubdir{
		dest: []common.Pin{
			{
				PackageName: c.name,
				InstanceID:  rev.Id,
			},
		},
	}, &cipd_api.EnsureOptions{
		Paranoia:            cipd_api.CheckPresence,
		DryRun:              false,
		OverrideInstallMode: pkg.InstallModeCopy,
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return fs, nil
}

// GetNotSubmittedReason implements Child.
func (c *CIPDChild) GetNotSubmittedReason(context.Context, *revision.Revision) (string, error) {
	// We just assume that all CIPD revisions are submitted because CIPD
	// packages don't really have a concept of being submitted and because those
	// which are built from Git revisions are generally only built from Git
	// revisions which are submitted.
	return "", nil
}

// SetClientForTesting sets the CIPDClient used by the CIPDChild so that it can
// be overridden for testing.
func (c *CIPDChild) SetClientForTesting(client cipd.CIPDClient) {
	c.client = client
}

type cipdDetailsLine struct {
	index int
	line  string
}

// CIPDInstanceToRevision creates a revision.Revision based on the given
// InstanceInfo.
func CIPDInstanceToRevision(name string, instance *cipd_api.InstanceDescription, revisionIdTag string, revisionIdTagStripKey bool) (*revision.Revision, error) {
	sha256, err := cipd.InstanceIDToSha256(instance.Pin.InstanceID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rev := &revision.Revision{
		Id:          instance.Pin.InstanceID,
		Checksum:    sha256,
		Author:      instance.RegisteredBy,
		Description: instance.Pin.String(),
		Timestamp:   time.Time(instance.RegisteredTs),
		URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.DefaultServiceURL, name, instance.Pin.InstanceID),
	}
	detailsLines := []*cipdDetailsLine{}
	foundRevisionTag := false
	for _, tag := range instance.Tags {
		key, val, err := splitCIPDTag(tag.Tag)
		if err != nil {
			sklog.Errorf("Invalid CIPD tag: %s", err)
			continue
		}
		if key == revisionIdTag {
			if revisionIdTagStripKey {
				rev.Id = val
			} else {
				rev.Id = tag.Tag
			}
			foundRevisionTag = true
		}
		if key == "bug" {
			// For bugs, we expect either eg. "chromium:1234" or "b/1234".
			split := strings.SplitN(val, ":", 2)
			if rev.Bugs == nil {
				rev.Bugs = map[string][]string{}
			}
			if len(split) == 2 {
				rev.Bugs[split[0]] = append(rev.Bugs[split[0]], split[1])
			} else if strings.HasPrefix(val, cipdBuganizerPrefix) {
				rev.Bugs[revision.BugProjectBuganizer] = append(rev.Bugs[revision.BugProjectBuganizer], val[len(cipdBuganizerPrefix):])
			} else {
				sklog.Errorf("Invalid format for \"bug\" tag: %s", tag.Tag)
			}
		} else if m := cipdDetailsRegex.FindStringSubmatch(key); len(m) == 2 {
			// For details, the tag value becomes one line. The tag key includes
			// an int which is used to determine the ordering of the lines.
			index, err := strconv.Atoi(m[1])
			if err != nil {
				// This shouldn't happen thanks to the regex.
				sklog.Errorf("Failed to parse int from details tag %q: %s", tag.Tag, err)
				continue
			}
			detailsLines = append(detailsLines, &cipdDetailsLine{
				index: index,
				line:  val,
			})
		}
	}
	if !foundRevisionTag && revisionIdTag != "" {
		rev.InvalidReason = fmt.Sprintf("Package instance has no tag %q", revisionIdTag)
	}

	// Set the display string.
	rev.Display = rev.Id
	idSplit := strings.SplitN(rev.Display, ":", 2)
	if len(idSplit) == 2 {
		// Strip the tag key from the display string.
		rev.Display = idSplit[1]
	}
	rev.Display = util.Truncate(rev.Display, 20)

	// Concatenate the details lines.
	if len(detailsLines) > 0 {
		sort.Slice(detailsLines, func(i, j int) bool {
			if detailsLines[i].index == detailsLines[j].index {
				return detailsLines[i].line < detailsLines[j].line
			}
			return detailsLines[i].index < detailsLines[j].index
		})
		for idx, line := range detailsLines {
			rev.Details += line.line
			if idx < len(detailsLines)-1 {
				rev.Details += "\n"
			}
		}
	}
	return rev, nil
}

// getGitRevisionFromCIPDInstance retrieves the git_revision tag from the given
// CIPD package instance, or the empty string if none exists.
func getGitRevisionFromCIPDInstance(instance *cipd_api.InstanceDescription) string {
	for _, tag := range instance.Tags {
		key, value, err := splitCIPDTag(tag.Tag)
		if err != nil {
			sklog.Error(err)
			continue
		}
		if cipdGitRevisionTag == key {
			return value
		}
	}
	return ""
}

// splitCIPDTag returns the key and value of the tag, which must be in
// "key:value" form.
func splitCIPDTag(tag string) (string, string, error) {
	split := strings.SplitN(tag, ":", 2)
	if len(split) != 2 {
		return "", "", skerr.Fmt("Invalid CIPD tag %q; expected <key>:<value>", tag)
	}
	return split[0], split[1], nil
}

// joinCIPDTag joins the key and value into a CIPD tag.
func joinCIPDTag(key, value string) string {
	return fmt.Sprintf("%s:%s", key, value)
}

// CIPDGitRevisionTag creates a git_revision tag for the given hash.
func CIPDGitRevisionTag(hash string) string {
	return joinCIPDTag(cipdGitRevisionTag, hash)
}

var _ Child = &CIPDChild{}
