package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
)

func TestFindImages(t *testing.T) {
	unittest.SmallTest(t)
	*project = "skia-public"
	Init()
	buildInfo := cloudbuild.Build{
		Results: &cloudbuild.Results{
			Images: []*cloudbuild.BuiltImage{
				{Name: "testing-clang9"},
				{Name: "gcr.io/skia-public/fiddler:prod"},
				{Name: "gcr.io/skia-public/skottie:prod"},
			},
		},
	}
	images := imagesFromInfo([]string{"fiddler", "skottie"}, buildInfo)
	assert.Equal(t, "gcr.io/skia-public/fiddler:prod", images[0])
	assert.Equal(t, "gcr.io/skia-public/skottie:prod", images[1])

	images = imagesFromInfo([]string{"skottie"}, buildInfo)
	assert.Equal(t, "gcr.io/skia-public/skottie:prod", images[0])
}

func TestBaseImageName(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "", baseImageName(""))
	assert.Equal(t, "", baseImageName("debian"))
	assert.Equal(t, "fiddler", baseImageName("gcr.io/skia-public/fiddler:prod"))
	assert.Equal(t, "docserver", baseImageName("gcr.io/skia-public/docserver:123456"))
}
