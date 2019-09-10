package goldpushk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNewGoldServiceDeployment(t *testing.T) {
	unittest.SmallTest(t)

	expected := GoldServiceDeployment{
		GoldInstanceServicePair: GoldInstanceServicePair{
			Chrome,
			DiffServer,
		},
	}
	actual := NewGoldServiceDeployment(Chrome, DiffServer)

	assert.Equal(t, expected, actual)
}

func TestGoldServiceDeploymentCanonicalName(t *testing.T) {
	unittest.SmallTest(t)
	d := NewGoldServiceDeployment(Chrome, DiffServer)
	assert.Equal(t, "gold-chrome-diffserver", d.CanonicalName())
}

func TestGoldServicesMapAddDeployment(t *testing.T) {
	unittest.SmallTest(t)

	m := make(GoldServicesMap)
	m.AddDeployment(Chrome, DiffServer, GoldServiceDeployment{Internal: true})

	expected := GoldServicesMap{
		Chrome: {
			DiffServer: {
				GoldInstanceServicePair: GoldInstanceServicePair{
					Instance: Chrome,
					Service:  DiffServer,
				},
				Internal: true,
			},
		},
	}

	assert.Equal(t, expected, m)
}

func TestGoldServicesMapForAll(t *testing.T) {
	unittest.SmallTest(t)

	chromeDiffServer := NewGoldServiceDeployment(Chrome, DiffServer)
	chromeSkiaCorrectness := NewGoldServiceDeployment(Chrome, SkiaCorrectness)
	skiaDiffServer := NewGoldServiceDeployment(Skia, DiffServer)
	skiaSkiaCorrectness := NewGoldServiceDeployment(Skia, SkiaCorrectness)
	skiaPublicDiffServer := NewGoldServiceDeployment(SkiaPublic, DiffServer)

	servicesMap := GoldServicesMap{
		Chrome: {
			DiffServer:      chromeDiffServer,
			SkiaCorrectness: chromeSkiaCorrectness,
		},
		Skia: {
			DiffServer:      skiaDiffServer,
			SkiaCorrectness: skiaSkiaCorrectness,
		},
		SkiaPublic: {
			DiffServer: skiaPublicDiffServer,
		},
	}

	type item struct {
		instance   GoldInstance
		service    GoldService
		deployment GoldServiceDeployment
	}

	items := make([]item, 0)
	servicesMap.ForAll(func(instance GoldInstance, service GoldService, deployment GoldServiceDeployment) {
		items = append(items, item{instance, service, deployment})
	})

	expectedItems := []item{
		{Chrome, DiffServer, chromeDiffServer},
		{Chrome, SkiaCorrectness, chromeSkiaCorrectness},
		{Skia, DiffServer, skiaDiffServer},
		{Skia, SkiaCorrectness, skiaSkiaCorrectness},
		{SkiaPublic, DiffServer, skiaPublicDiffServer},
	}

	assert.Equal(t, expectedItems, items)
}
