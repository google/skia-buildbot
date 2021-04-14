package child

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

const (
	cipdPackageUrlTmpl  = "%s/p/%s/+/%s"
	cipdBuganizerPrefix = "b/"
	gitRevisionTag      = "git_revision"
)

var (
	cipdDetailsRegex = regexp.MustCompile(`details(\d+)`)
)

// NewCIPD returns an implementation of Child which deals with a CIPD package.
// If the caller calls CIPDChild.Download, the destination must be a descendant of
// the provided workdir.
func NewCIPD(ctx context.Context, c *config.CIPDChildConfig, reg *config_vars.Registry, client *http.Client, workdir string) (*CIPDChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	cipdClient, err := cipd.NewClient(client, workdir, cipd.DefaultServiceURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var gitRepo *gitiles_common.GitilesRepo
	if c.GitilesRepo != "" {
		gitRepo, err = gitiles_common.NewGitilesRepo(ctx, &config.GitilesConfig{
			Branch:  git.DefaultBranch,
			RepoUrl: c.GitilesRepo,
		}, reg, client)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return &CIPDChild{
		client:  cipdClient,
		name:    c.Name,
		root:    workdir,
		tag:     c.Tag,
		gitRepo: gitRepo,
	}, nil
}

// CIPDChild is an implementation of Child which deals with a CIPD package.
type CIPDChild struct {
	client  cipd.CIPDClient
	name    string
	root    string
	tag     string
	gitRepo *gitiles_common.GitilesRepo
}

// GetRevision implements Child.
func (c *CIPDChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	instance, err := c.client.Describe(ctx, c.name, id)
	if err != nil {
		pins, err2 := c.client.SearchInstances(ctx, c.name, []string{id})
		if err2 != nil {
			return nil, skerr.Wrapf(err, "failed to retrieve revision ID %q and failed to search by tag with: %s", id, err2)
		}
		if len(pins) == 0 {
			return nil, skerr.Wrapf(err, "failed to retrieve revision ID %q and found no matching instances by tag.", id)
		}
		if len(pins) > 1 {
			sklog.Errorf("Found more than one matching instance for tag %q; arbitrarily returning the first.", id)
		}
		instance, err = c.client.Describe(ctx, c.name, pins[0].InstanceID)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	rev := CIPDInstanceToRevision(c.name, instance)
	if c.gitRepo != nil {
		gitRevision := getGitRevisionFromCIPDInstance(instance)
		if gitRevision == "" {
			rev.InvalidReason = "No git_revision tag"
		} else {
			rev, err = c.gitRepo.GetRevision(ctx, gitRevision)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			rev.Id = fmt.Sprintf("%s:%s", gitRevisionTag, gitRevision)
		}
	}
	return rev, nil
}

// Update implements Child.
// Note: that this just finds the newest version of the CIPD package.
func (c *CIPDChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	head, err := c.client.ResolveVersion(ctx, c.name, c.tag)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	tipRev, err := c.GetRevision(ctx, head.InstanceID)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	notRolledRevs := []*revision.Revision{}
	if lastRollRev.Id != tipRev.Id {
		notRolledRevs = append(notRolledRevs, tipRev)
	}
	if c.gitRepo != nil {
		// Obtain the git revisions from the backing repo.
		_, lastRollRevHash, err := splitCIPDTag(lastRollRev.Id)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		_, tipRevHash, err := splitCIPDTag(tipRev.Id)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		notRolledCommits, err := c.gitRepo.LogFirstParent(ctx, lastRollRevHash, tipRevHash)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		notRolledRevs, err = c.gitRepo.ConvertRevisions(ctx, notRolledCommits)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		for _, rev := range notRolledRevs {
			// Fix the IDs to be CIPD tags rather than Git commit hashes.
			rev.Id = fmt.Sprintf("%s:%s", gitRevisionTag, rev.Id)

			// Make in-between revisions invalid, since we only have CIPD
			// package instances associated with lastRollRev and tipRev.
			if rev.Id != lastRollRev.Id && rev.Id != tipRev.Id {
				rev.InvalidReason = "No associated CIPD package."
			}
		}
	}
	return tipRev, notRolledRevs, nil
}

// VFS implements the Child interface.
func (c *CIPDChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	fs, err := vfs.TempDir(ctx, c.root, "tmp")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	pin := common.Pin{
		PackageName: c.name,
		InstanceID:  rev.Id,
	}
	dest, err := filepath.Rel(c.root, fs.Dir())
	if err := c.client.FetchAndDeployInstance(ctx, dest, pin, 0); err != nil {
		return nil, skerr.Wrap(err)
	}
	return fs, nil
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
func CIPDInstanceToRevision(name string, instance *cipd_api.InstanceDescription) *revision.Revision {
	rev := &revision.Revision{
		Id:          instance.Pin.InstanceID,
		Author:      instance.RegisteredBy,
		Display:     util.Truncate(instance.Pin.InstanceID, 12),
		Description: instance.Pin.String(),
		Timestamp:   time.Time(instance.RegisteredTs),
		URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.DefaultServiceURL, name, instance.Pin.InstanceID),
	}
	detailsLines := []*cipdDetailsLine{}
	for _, tag := range instance.Tags {
		split := strings.SplitN(tag.Tag, ":", 2)
		if len(split) != 2 {
			sklog.Errorf("Invalid CIPD tag %q; expected <key>:<value>", tag.Tag)
			continue
		}
		key := split[0]
		val := split[1]
		if key == "bug" {
			// For bugs, we expect either eg. "chromium:1234" or "b/1234".
			split := strings.SplitN(val, ":", 2)
			if rev.Bugs == nil {
				rev.Bugs = map[string][]string{}
			}
			if len(split) == 2 {
				rev.Bugs[split[0]] = append(rev.Bugs[split[0]], split[1])
			} else if strings.HasPrefix(val, cipdBuganizerPrefix) {
				rev.Bugs[util.BUG_PROJECT_BUGANIZER] = append(rev.Bugs[util.BUG_PROJECT_BUGANIZER], val[len(cipdBuganizerPrefix):])
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
	return rev
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
		if gitRevisionTag == key {
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

// gitRevTag creates a git_revision tag for the given hash.
func gitRevTag(hash string) string {
	return joinCIPDTag(gitRevisionTag, hash)
}

var _ Child = &CIPDChild{}
