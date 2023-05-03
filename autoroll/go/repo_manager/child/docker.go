package child

import (
	"context"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

// NewDocker returns an implementation of Child which deals with Docker images.
func NewDocker(ctx context.Context, c *config.DockerChildConfig) (*DockerChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	dockerClient, err := docker.NewClient(ctx, c.Registry)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DockerChild{
		client: dockerClient,
		repo:   c.Repository,
		tag:    c.Tag,
	}, nil
}

// DockerChild is an implementation of Child which deals with Docker images.
type DockerChild struct {
	client docker.Client
	repo   string
	tag    string
}

// GetRevision implements Child.
func (c *DockerChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	manifest, err := c.client.GetManifest(ctx, c.repo, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	config, err := c.client.GetConfig(ctx, c.repo, manifest.Config.Digest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Use a shortened digest for display, without the "sha256:" prefix.
	display := strings.TrimPrefix(manifest.Digest, "sha256:")
	if len(display) > 12 {
		display = display[:12]
	}

	// Sometimes creation timestamps are zero. I'm not sure why this is, but if
	// we dig into the image history we can find some which are non-zero. The
	// most recent layer(s) should be close to the correct time.
	timestamp := config.Created
	if util.TimeIsZero(timestamp) {
		for _, hist := range config.History {
			if hist.Created.After(timestamp) {
				timestamp = hist.Created
			}
		}
	}
	return &revision.Revision{
		Id:        manifest.Digest,
		Checksum:  manifest.Digest,
		Author:    config.Author,
		Display:   display,
		Timestamp: timestamp,
	}, nil
}

// Update implements Child.
func (c *DockerChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, err := c.GetRevision(ctx, c.tag)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	var notRolledRevs []*revision.Revision
	if tipRev.Id != lastRollRev.Id {
		notRolledRevs = []*revision.Revision{tipRev}
	}
	return tipRev, notRolledRevs, nil
}

// VFS implements the Child interface.
func (c *DockerChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	return nil, skerr.Fmt("VFS not implemented for DockerChild")
}

// Assert that DockerChild implements Child.
var _ Child = &DockerChild{}
