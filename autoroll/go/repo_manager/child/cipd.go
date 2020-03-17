package child

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"

	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
)

const cipdPackageUrlTmpl = "%s/p/%s/+/%s"

// CipdChildConfig provides configuration for CipdChild.
type CipdChildConfig struct {
	CipdAssetName string `json:"cipdAssetName"`
	CipdAssetTag  string `json:"cipdAssetTag"`
}

// See documentation for util.Validator interface.
func (c *CipdChildConfig) Validate() error {
	if c.CipdAssetName == "" {
		return fmt.Errorf("CipdAssetName is required.")
	}
	if c.CipdAssetTag == "" {
		return fmt.Errorf("CipdAssetTag is required.")
	}
	return nil
}

// See documentation for RepoManagerConfig interface.
func (c *CipdChildConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// CipdChild is an implementation of Child which rolls a CIPD package.
type CipdChild struct {
	assetName string
	assetTag  string
	client    cipd.CIPDClient
}

// NewCipdChild returns a CipdChild instance.
func NewCipdChild(c CipdChildConfig, client *http.Client, workdir string) (*CipdChild, error) {
	cipdClient, err := cipd.NewClient(client, path.Join(workdir, "cipd"))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &CipdChild{
		assetName: c.CipdAssetName,
		assetTag:  c.CipdAssetTag,
		client:    cipdClient,
	}, nil
}

// instanceToRevision converts the InstanceInfo to a Revision.
func (c *CipdChild) instanceToRevision(instance *cipd_api.InstanceInfo) *revision.Revision {
	return &revision.Revision{
		Id:          instance.Pin.InstanceID,
		Display:     instance.Pin.InstanceID[:5] + "...",
		Description: instance.Pin.String(),
		Timestamp:   time.Time(instance.RegisteredTs),
		URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.SERVICE_URL, c.assetName, instance.Pin.InstanceID),
	}
}

// See documentation for Child interface.
func (c *CipdChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	instance, err := c.client.Describe(ctx, c.assetName, id)
	if err != nil {
		return nil, err
	}
	return c.instanceToRevision(&instance.InstanceInfo), nil
}

// See documentation for Child interface.
//
// Note: that this just finds all versions of the package between the last
// rolled version and the version currently pointed to by assetTag; we can't
// know whether the ref we're tracking was ever actually applied to any of the
// package instances in between.
func (c *CipdChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	head, err := c.client.ResolveVersion(ctx, c.assetName, c.assetTag)
	if err != nil {
		return nil, nil, err
	}
	tipRev, err := c.GetRevision(ctx, head.InstanceID)
	if err != nil {
		return nil, nil, err
	}
	if lastRollRev.Id == head.InstanceID {
		return tipRev, []*revision.Revision{}, nil
	}
	iter, err := c.client.ListInstances(ctx, c.assetName)
	if err != nil {
		return nil, nil, err
	}
	notRolledRevs := []*revision.Revision{}
	foundHead := false
	for {
		instances, err := iter.Next(ctx, 100)
		if err != nil {
			return nil, nil, err
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
				notRolledRevs = append(notRolledRevs, c.instanceToRevision(&instance))
			}
		}
	}
	return tipRev, notRolledRevs, nil
}

// SetCipdClientForTesting replaces the CipdChild's CIPD client.
func (c *CipdChild) SetCipdClientForTesting(client cipd.CIPDClient) {
	c.client = client
}
