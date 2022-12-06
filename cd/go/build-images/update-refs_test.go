package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBazelRegexAndReplaceForImage(t *testing.T) {
	const fileContents = `# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:0ae30b768fb1bdcbea5b6721075b758806c4076a74a8a99a67ff3632df87cf5a",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:17e18164238a4162ce2c30b7328a7e44fbe569e56cab212ada424dc7378c1f5f",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)`
	const expectedContents = `# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:0ae30b768fb1bdcbea5b6721075b758806c4076a74a8a99a67ff3632df87cf5a",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:f5f1c8737cd424ada212bac65e965ebf44e7a8237b03c2ec2614a83246181e71",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)`
	image := &SingleImageInfo{
		Image:  "gcr.io/skia-public/cd-base",
		Tag:    "unused",
		Sha256: "sha256:f5f1c8737cd424ada212bac65e965ebf44e7a8237b03c2ec2614a83246181e71",
	}
	regex, replace := bazelRegexAndReplaceForImage(image)
	updatedContents := regex.ReplaceAllString(fileContents, replace)
	require.Equal(t, expectedContents, updatedContents)
}
