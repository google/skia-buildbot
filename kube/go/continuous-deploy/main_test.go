package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
)

func TestFindImages(t *testing.T) {
	testutils.SmallTest(t)
	*project = "skia-public"
	Init()
	buildInfo := cloudbuild.Build{
		Results: &cloudbuild.Results{
			Images: []*cloudbuild.BuiltImage{
				&cloudbuild.BuiltImage{Name: "testing-clang9"},
				&cloudbuild.BuiltImage{Name: "gcr.io/skia-public/fiddler:prod"},
				&cloudbuild.BuiltImage{Name: "gcr.io/skia-public/skottie:prod"},
			},
		},
	}
	b, err := json.Marshal(buildInfo)
	assert.NoError(t, err)
	images, err := imagesFromMsg([]string{"fiddler", "skottie"}, b)
	assert.NoError(t, err)
	assert.Equal(t, "gcr.io/skia-public/fiddler:prod", images[0])
	assert.Equal(t, "gcr.io/skia-public/skottie:prod", images[1])

	images, err = imagesFromMsg([]string{"skottie"}, b)
	assert.NoError(t, err)
	assert.Equal(t, "gcr.io/skia-public/skottie:prod", images[0])
}

func TestBaseImageName(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "", baseImageName(""))
	assert.Equal(t, "", baseImageName("debian"))
	assert.Equal(t, "fiddler", baseImageName("gcr.io/skia-public/fiddler:prod"))
	assert.Equal(t, "docserver", baseImageName("gcr.io/skia-public/docserver:123456"))
}
