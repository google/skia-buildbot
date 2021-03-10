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
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

const (
	cipdPackageUrlTmpl  = "%s/p/%s/+/%s"
	cipdBuganizerPrefix = "b/"
)

var (
	cipdDetailsRegex = regexp.MustCompile(`details(\d+)`)
)

// NewCIPD returns an implementation of Child which deals with a CIPD package.
// If the caller calls CIPDChild.Download, the destination must be a descendant of
// the provided workdir.
func NewCIPD(ctx context.Context, c *config.CIPDChildConfig, client *http.Client, workdir string) (*CIPDChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	cipdClient, err := cipd.NewClient(client, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &CIPDChild{
		client:  cipdClient,
		name:    c.Name,
		root:    workdir,
		tag:     c.Tag,
		tagAsID: c.TagAsId,
	}, nil
}

// CIPDChild is an implementation of Child which deals with a CIPD package.
type CIPDChild struct {
	client  cipd.CIPDClient
	name    string
	root    string
	tag     string
	tagAsID string
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
	return CIPDInstanceToRevision(c.name, c.tagAsID, instance), nil
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
func CIPDInstanceToRevision(name, tagAsID string, instance *cipd_api.InstanceDescription) *revision.Revision {
	rev := &revision.Revision{
		Id:          instance.Pin.InstanceID,
		Author:      instance.RegisteredBy,
		Display:     util.Truncate(instance.Pin.InstanceID, 12),
		Description: instance.Pin.String(),
		Timestamp:   time.Time(instance.RegisteredTs),
		URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.ServiceUrl, name, instance.Pin.InstanceID),
	}
	foundTagAsID := false
	detailsLines := []*cipdDetailsLine{}
	for _, tag := range instance.Tags {
		split := strings.SplitN(tag.Tag, ":", 2)
		if len(split) != 2 {
			sklog.Errorf("Invalid CIPD tag %q; expected <key>:<value>", tag.Tag)
			continue
		}
		key := split[0]
		val := split[1]
		if tagAsID == key {
			rev.Id = tag.Tag
			rev.Display = tag.Tag
			foundTagAsID = true
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
	if tagAsID != "" && !foundTagAsID {
		rev.InvalidReason = fmt.Sprintf("No %q tag", tagAsID)
	}
	return rev
}

var _ Child = &CIPDChild{}
