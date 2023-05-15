package stages

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/docker"
	docker_mocks "go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs"
)

const (
	fakeImage                = "gcr.io/skia-public/status"
	fakeRepo                 = "https://my-repo.git"
	initialStageFileContents = `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "26e5feb7b61942474d710233719db3d0304f5e58",
          "digest": "sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45"
        }
      }
    }
  }
}`
	deploymentPath            = "skia-infra-public/status.yaml"
	initialDeploymentContents = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status:prod, auth-proxy:prod"
    spec:
      containers:
        - name: status
          image: gcr.io/skia-public/status@sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`
)

var (
	imageInstances = map[string]*docker.ImageInstance{
		"sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2": {
			Digest: "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2",
			Tags: []string{
				"git-6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
				"my-tag",
			},
		},
	}
)

func assertFileContents(t *testing.T, ctx context.Context, fs vfs.FS, path, expect string) {
	actual, err := vfs.ReadFile(ctx, fs, path)
	require.NoError(t, err)
	require.Equal(t, expect, string(actual))
}

func setup(t *testing.T) (context.Context, *StageManager, vfs.FS, *docker_mocks.Client, func()) {
	ctx := context.Background()
	fs, err := vfs.TempDir(ctx, "", "")
	require.NoError(t, err)
	for _, configDir := range configDirs {
		require.NoError(t, os.MkdirAll(filepath.Join(fs.Dir(), configDir), os.ModePerm))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(fs.Dir(), path.Dir(StageFilePath)), os.ModePerm))
	require.NoError(t, vfs.WriteFile(ctx, fs, StageFilePath, []byte(initialStageFileContents)))
	require.NoError(t, os.MkdirAll(filepath.Join(fs.Dir(), path.Dir(deploymentPath)), os.ModePerm))
	require.NoError(t, vfs.WriteFile(ctx, fs, deploymentPath, []byte(initialDeploymentContents)))
	dc := &docker_mocks.Client{}
	sm := NewStageManager(ctx, fs, dc)
	return ctx, sm, fs, dc, func() {
		require.NoError(t, fs.Close(ctx))
	}
}

func TestStageManager_AddImage(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	require.NoError(t, sm.AddImage(ctx, "gcr.io/skia-public/second-image", ""))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/second-image": {},
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "26e5feb7b61942474d710233719db3d0304f5e58",
          "digest": "sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45"
        }
      }
    }
  }
}`)
	assertFileContents(t, ctx, fs, deploymentPath, initialDeploymentContents)
}

func TestStageManager_AddImage_WithGitRepo(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	require.NoError(t, sm.AddImage(ctx, "gcr.io/skia-public/second-image", "https://custom-repo.git"))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/second-image": {
      "git_repo": "https://custom-repo.git"
    },
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "26e5feb7b61942474d710233719db3d0304f5e58",
          "digest": "sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45"
        }
      }
    }
  }
}`)
	assertFileContents(t, ctx, fs, deploymentPath, initialDeploymentContents)
}

func TestStageManager_AddImage_AlreadyExists(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	err := sm.AddImage(ctx, fakeImage, fakeRepo)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image \"gcr.io/skia-public/status\" already exists")
	assertFileContents(t, ctx, fs, StageFilePath, initialStageFileContents)
}

func TestStageManager_RemoveImage(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	require.NoError(t, sm.RemoveImage(ctx, fakeImage))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git"
}`)
	assertFileContents(t, ctx, fs, deploymentPath, initialDeploymentContents)

	// Can't remove an image we don't have.
	err := sm.RemoveImage(ctx, fakeImage)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image \"gcr.io/skia-public/status\" does not exist")
}

func TestStageManager_SetStage_ByDigest(t *testing.T) {
	ctx, sm, fs, dc, cleanup := setup(t)
	defer cleanup()

	dc.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/status", "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2").Return(&docker.Manifest{
		Digest: "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2",
	}, nil)
	dc.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/status").Return(imageInstances, nil)
	require.NoError(t, sm.SetStage(ctx, fakeImage, "prod", "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        }
      }
    }
  }
}`)
	assertFileContents(t, ctx, fs, deploymentPath, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status:prod, auth-proxy:prod"
    spec:
      containers:
        - name: status
          image: gcr.io/skia-public/status@sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`)
}

func TestStageManager_SetStage_ByTag(t *testing.T) {
	ctx, sm, fs, dc, cleanup := setup(t)
	defer cleanup()

	dc.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/status", "my-tag").Return(&docker.Manifest{
		Digest: "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2",
	}, nil)
	dc.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/status").Return(imageInstances, nil)
	require.NoError(t, sm.SetStage(ctx, fakeImage, "prod", "my-tag"))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        }
      }
    }
  }
}`)
	assertFileContents(t, ctx, fs, deploymentPath, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status:prod, auth-proxy:prod"
    spec:
      containers:
        - name: status
          image: gcr.io/skia-public/status@sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`)
}

func TestStageManager_SetStage_ByGitHash(t *testing.T) {
	ctx, sm, fs, dc, cleanup := setup(t)
	defer cleanup()

	dc.On("GetManifest", testutils.AnyContext, "gcr.io", "skia-public/status", "git-6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8").Return(&docker.Manifest{
		Digest: "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2",
	}, nil)
	dc.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/status").Return(imageInstances, nil)
	require.NoError(t, sm.SetStage(ctx, fakeImage, "prod", "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8"))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        }
      }
    }
  }
}`)
	assertFileContents(t, ctx, fs, deploymentPath, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status:prod, auth-proxy:prod"
    spec:
      containers:
        - name: status
          image: gcr.io/skia-public/status@sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`)
}

func TestStageManager_PromoteStage(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	require.NoError(t, sm.PromoteStage(ctx, fakeImage, "latest", "prod"))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        },
        "prod": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        }
      }
    }
  }
}`)
	assertFileContents(t, ctx, fs, deploymentPath, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status:prod, auth-proxy:prod"
    spec:
      containers:
        - name: status
          image: gcr.io/skia-public/status@sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`)
}

func TestStageManager_RemoveStage(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	require.NoError(t, sm.RemoveStage(ctx, fakeImage, "prod"))
	assertFileContents(t, ctx, fs, StageFilePath, `{
  "default_git_repo": "https://default-repo.git",
  "images": {
    "gcr.io/skia-public/status": {
      "git_repo": "https://my-repo.git",
      "stages": {
        "latest": {
          "git_hash": "6e2fb7dbe8533d1497dae1d08b2f855a5b3bd2e8",
          "digest": "sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2"
        }
      }
    }
  }
}`)
	// Despite our removal of the "prod" stage, which the deployment uses, we
	// don't mess with the k8s config file.
	assertFileContents(t, ctx, fs, deploymentPath, initialDeploymentContents)
}

func TestStageManager_Apply(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	// Change status to track "latest".
	newContents := strings.ReplaceAll(initialDeploymentContents, "prod", "latest")
	require.NoError(t, vfs.WriteFile(ctx, fs, deploymentPath, []byte(newContents)))

	// We didn't touch the stage file, but the deployment is updated to the
	// digest of the newly-tracked stage.
	require.NoError(t, sm.Apply(ctx))
	assertFileContents(t, ctx, fs, StageFilePath, initialStageFileContents)
	assertFileContents(t, ctx, fs, deploymentPath, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status:latest, auth-proxy:latest"
    spec:
      containers:
        - name: status
          image: gcr.io/skia-public/status@sha256:4a75315fabbfe385da4cdebc9d0db6a742ef8d41319fe3100dd435a6e4bc58c2
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`)
}

func TestPromoteStage_RefuseMultipleEdits(t *testing.T) {
	ctx, sm, fs, _, cleanup := setup(t)
	defer cleanup()

	// Add a second container running the same image. Our parsing code doesn't
	// know how to distinguish between multiples of the same image within the
	// same top-level YAML document, so we error out to avoid accidentally-
	// incorrect updates. In practice it should be unlikely that we have
	// multiple containers in the same Deployment/StatefulSet/etc which use the
	// same image. If we need to support that, we'll need to update our YAML
	// parsing to keep track of the source locations of the individual container
	// definitions.
	initialDeploymentContentsWithDuplicateImages := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: status
  namespace: status
spec:
  template:
    metadata:
      annotations:
        skia.org/stage: "status1:prod, status2:prod, auth-proxy:prod"
    spec:
      containers:
        - name: status1
          image: gcr.io/skia-public/status@sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45
        - name: status2
          image: gcr.io/skia-public/status@sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45
        - name: auth-proxy
          image: gcr.io/skia-public/auth-proxy@sha256:daeaa0ab4a7857532c13ef4f5e6c6686b84420223ebac049b7a05e62f0b7f5ef
`

	require.NoError(t, vfs.WriteFile(ctx, fs, deploymentPath, []byte(initialDeploymentContentsWithDuplicateImages)))

	err := sm.PromoteStage(ctx, fakeImage, "latest", "prod")
	require.Error(t, err)
	require.Contains(t, err.Error(), "found more than one instance of gcr.io/skia-public/status@sha256:1a1ea5d8514940de464c7893c6ba1ceb847b711a06dba6a940b15d30ea06db45")
	require.Contains(t, err.Error(), "updating containers of status")
	assertFileContents(t, ctx, fs, deploymentPath, initialDeploymentContentsWithDuplicateImages)
}
