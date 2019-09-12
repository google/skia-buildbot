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

func TestDeployableUnitSetAdd(t *testing.T) {
	unittest.SmallTest(t)

	s := DeployableUnitSet{}
	s.Add(Chrome, DiffServer)

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
	s.AddWithOptions(Chrome, DiffServer, DeploymentOptions{internal: true})

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

	// Add element with AddWithOptions().
	s.AddWithOptions(Chrome, DiffServer, DeploymentOptions{internal: true})
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

	// Overwrite with AddWithOptions().
	s.AddWithOptions(Chrome, DiffServer, DeploymentOptions{internal: false})
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

	// Overwrite with Add().
	s.Add(Chrome, DiffServer)
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
