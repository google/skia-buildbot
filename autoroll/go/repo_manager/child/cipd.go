package child

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
)

const (
	cipdPackageUrlTmpl = "%s/p/%s/+/%s"
)

// CIPDConfig provides configuration for CIPDChild.
type CIPDConfig struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

// See documentation for util.Validator interface.
func (c *CIPDConfig) Validate() error {
	if c.Name == "" {
		return skerr.Fmt("Name is required.")
	}
	if c.Tag == "" {
		return skerr.Fmt("Tag is required.")
	}
	return nil
}

// NewCIPD returns an implementation of Child which deals with a CIPD package.
// If the caller calls CIPDChild.Download, the destination must be a descendant of
// the provided workdir.
func NewCIPD(ctx context.Context, c CIPDConfig, client *http.Client, workdir string) (*CIPDChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	cipdClient, err := cipd.NewClient(client, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &CIPDChild{
		client: cipdClient,
		name:   c.Name,
		root:   workdir,
		tag:    c.Tag,
	}, nil
}

// CIPDChild is an implementation of Child which deals with a CIPD package.
type CIPDChild struct {
	client cipd.CIPDClient
	name   string
	root   string
	tag    string
}

// See documentation for Child interface.
func (c *CIPDChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	instance, err := c.client.Describe(ctx, c.name, id)
	if err != nil {
		return nil, err
	}
	return CIPDInstanceToRevision(c.name, &instance.InstanceInfo), nil
}

// See documentation for Child interface.
// Note: that this just finds all versions of the package between the last
// rolled version and the version currently pointed to by the configured tag; we
// can't know whether the tag we're tracking was ever actually applied to any of
// the package instances in between.
func (c *CIPDChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	head, err := c.client.ResolveVersion(ctx, c.name, c.tag)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	tipRev, err := c.GetRevision(ctx, head.InstanceID)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if lastRollRev.Id == tipRev.Id {
		return tipRev, []*revision.Revision{}, nil
	}
	iter, err := c.client.ListInstances(ctx, c.name)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	notRolledRevs := []*revision.Revision{}
	foundHead := false
	for {
		instances, err := iter.Next(ctx, 100)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		if len(instances) == 0 {
			break
		}
		for _, instance := range instances {
			id := instance.Pin.InstanceID
			if id == head.InstanceID {
				foundHead = true
			}
			if id == lastRollRev.Id {
				return tipRev, notRolledRevs, nil
			}
			if foundHead {
				notRolledRevs = append(notRolledRevs, CIPDInstanceToRevision(c.name, &instance))
			}
		}
	}
	return tipRev, notRolledRevs, nil
}

// See documentation for Child interface.
// The destination must be a descendant of workdir provided to NewCIPD.
func (c *CIPDChild) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	var err error
	dest, err = filepath.Rel(c.root, dest)
	if err != nil {
		return skerr.Wrapf(err, "destination must be a descendant of the workdir provided to NewCIPD")
	}
	pin := common.Pin{
		PackageName: c.name,
		InstanceID:  rev.Id,
	}
	return skerr.Wrap(c.client.FetchAndDeployInstance(ctx, dest, pin, 0))
}

// SetClientForTesting sets the CIPDClient used by the CIPDChild so that it can
// be overridden for testing.
func (c *CIPDChild) SetClientForTesting(client cipd.CIPDClient) {
	c.client = client
}

// CIPDInstanceToRevision creates a revision.Revision based on the given
// InstanceInfo.
func CIPDInstanceToRevision(name string, instance *cipd_api.InstanceInfo) *revision.Revision {
	return &revision.Revision{
		Id:          instance.Pin.InstanceID,
		Display:     instance.Pin.InstanceID[:5] + "...",
		Description: instance.Pin.String(),
		Timestamp:   time.Time(instance.RegisteredTs),
		URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.ServiceUrl, name, instance.Pin.InstanceID),
	}
}

var _ Child = &CIPDChild{}
