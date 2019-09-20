package goldpushk

import (
	"path/filepath"
	"strings"
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

	assert.Equal(t, p("/foo/bar/golden/k8s-config-templates/gold-diffserver-template.yaml"), unit.getDeploymentFileTemplatePath(p("/foo/bar")))
}

func TestDeployableUnitGetConfigMapFileTemplatePath(t *testing.T) {
	unittest.SmallTest(t)

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffServer,
		},
		DeploymentOptions: DeploymentOptions{
			configMapTemplate: p("path/to/config-map-template.json5"),
		},
	}

	assert.Equal(t, p("/foo/bar/path/to/config-map-template.json5"), unit.getConfigMapFileTemplatePath(p("/foo/bar")))
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

// p takes a Unix path and replaces forward slashes with the correct separators
// for the OS under which this test is running.
func p(path string) string {
	return filepath.Join(strings.Split(path, "/")...)
}
