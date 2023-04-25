package child

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/testutils"
)

const (
	fakeDockerRepo         = "skia-public/autoroll-be"
	fakeDockerTag          = "latest"
	fakeDockerDigest       = "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc"
	fakeDockerConfigDigest = "sha256:413668c8e4f58c8c979f9f8c4e3fbc9a5447149cc7f3a7e345b6d67a0615c5d6"
)

var (
	fakeDockerManifest = &docker.Manifest{
		Digest: fakeDockerDigest,
		Config: docker.MediaConfig{
			Digest: fakeDockerConfigDigest,
		},
	}
	fakeDockerConfig = &docker.ImageConfig{
		Author:  "Bazel",
		Created: time.Time{}, // This seems to be frequently empty.
		History: []docker.ImageConfig_History{
			{
				Created: time.Unix(1682445445, 0),
			},
		},
	}
)

func TestDockerChild_GetRevision(t *testing.T) {
	ctx := context.Background()
	client := &mocks.Client{}
	client.On("GetManifest", testutils.AnyContext, fakeDockerRepo, fakeDockerTag).Return(fakeDockerManifest, nil)
	client.On("GetConfig", testutils.AnyContext, fakeDockerRepo, fakeDockerConfigDigest).Return(fakeDockerConfig, nil)
	c := &DockerChild{
		client: client,
		repo:   fakeDockerRepo,
		tag:    fakeDockerTag,
	}
	rev, err := c.GetRevision(ctx, fakeDockerTag)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:        "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc",
		Checksum:  "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc",
		Author:    "Bazel",
		Display:   "000ba24df84b",
		Timestamp: fakeDockerConfig.History[0].Created,
	}, rev)
}

func TestDockerChild_Update(t *testing.T) {
	ctx := context.Background()
	client := &mocks.Client{}
	client.On("GetManifest", testutils.AnyContext, fakeDockerRepo, fakeDockerTag).Return(fakeDockerManifest, nil)
	client.On("GetConfig", testutils.AnyContext, fakeDockerRepo, fakeDockerConfigDigest).Return(fakeDockerConfig, nil)
	c := &DockerChild{
		client: client,
		repo:   fakeDockerRepo,
		tag:    fakeDockerTag,
	}
	tipRev, notRolledRevs, err := c.Update(ctx, &revision.Revision{Id: "sha256:bbad"})
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:        "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc",
		Checksum:  "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc",
		Author:    "Bazel",
		Display:   "000ba24df84b",
		Timestamp: fakeDockerConfig.History[0].Created,
	}, tipRev)
	require.Equal(t, []*revision.Revision{tipRev}, notRolledRevs)
}
