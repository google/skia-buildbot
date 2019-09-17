package goldpushk

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestDeployableUnitIDCanonicalName(t *testing.T) {
	unittest.SmallTest(t)
	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Chrome,
			Service:  DiffServer,
		},
	}
	assert.Equal(t, "gold-chrome-diffserver", unit.CanonicalName())
}

func TestDeployableUnitGetDeploymentFileTemplatePath(t *testing.T) {
	unittest.SmallTest(t)

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffServer,
		},
	}

	assert.Equal(t, "/foo/bar/golden/k8s-config-templates/gold-diffserver-template.yaml", unit.getDeploymentFileTemplatePath("/foo/bar"))
}

func TestDeployableUnitGetDeploymentFilePath(t *testing.T) {
	unittest.SmallTest(t)

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffServer,
		},
	}

	assert.Equal(t, "/foo/bar/golden/build/gold-skia-diffserver.yaml", unit.getDeploymentFilePath("/foo/bar"))
}

func TestDeployableUnitGetConfigMapFileTemplatePath(t *testing.T) {
	unittest.SmallTest(t)

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffServer,
		},
		DeploymentOptions: DeploymentOptions{
			configMapTemplate: "path/to/config-map-template.json5",
		},
	}

	assert.Equal(t, "/foo/bar/path/to/config-map-template.json5", unit.getConfigMapTemplatePath("/foo/bar"))
}

func TestDeployableUnitGetConfigMapFilePath(t *testing.T) {
	unittest.SmallTest(t)

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffServer,
		},
		DeploymentOptions: DeploymentOptions{
			configMapFile: "path/to/config-map-file.json5",
		},
	}

	assert.Equal(t, "/foo/bar/path/to/config-map-file.json5", unit.getConfigMapFilePath("/foo/bar"))
}

func TestDeployableUnitSetAdd(t *testing.T) {
	unittest.SmallTest(t)

	s := DeployableUnitSet{}
	s.add(Chrome, DiffServer)

	expected := DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffServer,
				},
			},
		},
	}
	assert.Equal(t, expected, s)
}

func TestDeployableUnitSetAddWithOptions(t *testing.T) {
	unittest.SmallTest(t)

	s := DeployableUnitSet{}
	s.addWithOptions(Chrome, DiffServer, DeploymentOptions{internal: true})

	expected := DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffServer,
				},
				DeploymentOptions: DeploymentOptions{
					internal: true,
				},
			},
		},
	}
	assert.Equal(t, expected, s)
}

func TestDeployableUnitSetOverwriteElements(t *testing.T) {
	unittest.SmallTest(t)

	s := DeployableUnitSet{}

	// Add element with addWithOptions().
	s.addWithOptions(Chrome, DiffServer, DeploymentOptions{internal: true})
	expected := DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffServer,
				},
				DeploymentOptions: DeploymentOptions{
					internal: true,
				},
			},
		},
	}
	assert.Equal(t, expected, s)

	// Overwrite with addWithOptions().
	s.addWithOptions(Chrome, DiffServer, DeploymentOptions{internal: false})
	expected = DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffServer,
				},
				DeploymentOptions: DeploymentOptions{
					internal: false,
				},
			},
		},
	}
	assert.Equal(t, expected, s)

	// Overwrite with add().
	s.add(Chrome, DiffServer)
	expected = DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffServer,
				},
			},
		},
	}
	assert.Equal(t, expected, s)
}

func TestDeployableUnitSetGet(t *testing.T) {
	unittest.SmallTest(t)

	// Item not found.
	s := DeployableUnitSet{}
	_, ok := s.Get(DeployableUnitID{Instance: Chrome, Service: DiffServer})
	assert.False(t, ok)

	// Item found.
	s = DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffServer,
				},
			},
		},
	}
	unit, ok := s.Get(DeployableUnitID{Instance: Chrome, Service: DiffServer})
	expectedUnit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Chrome,
			Service:  DiffServer,
		},
	}
	assert.True(t, ok)
	assert.Equal(t, expectedUnit, unit)
}
