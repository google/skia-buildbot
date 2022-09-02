package goldpushk

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeployableUnitIDCanonicalName(t *testing.T) {
	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Chrome,
			Service:  DiffCalculator,
		},
	}
	require.Equal(t, "gold-chrome-diffcalculator", unit.CanonicalName())
}

func TestDeployableUnitGetDeploymentFileTemplatePath(t *testing.T) {

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffCalculator,
		},
	}

	require.Equal(t, p("/foo/bar/golden/k8s-config-templates/gold-diffcalculator-template.yaml"), unit.getDeploymentFileTemplatePath(p("/foo/bar/golden")))
}

func TestDeployableUnitSetAdd(t *testing.T) {

	s := DeployableUnitSet{}
	s.add(Chrome, DiffCalculator)

	expected := DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffCalculator,
				},
			},
		},
	}
	require.Equal(t, expected, s)
}

func TestDeployableUnitSetAddWithOptions(t *testing.T) {

	s := DeployableUnitSet{}
	s.addWithOptions(Chrome, DiffCalculator, DeploymentOptions{internal: true})

	expected := DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffCalculator,
				},
				DeploymentOptions: DeploymentOptions{
					internal: true,
				},
			},
		},
	}
	require.Equal(t, expected, s)
}

func TestDeployableUnitSetOverwriteElements(t *testing.T) {

	s := DeployableUnitSet{}

	// Add element with addWithOptions().
	s.addWithOptions(Chrome, DiffCalculator, DeploymentOptions{internal: true})
	expected := DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffCalculator,
				},
				DeploymentOptions: DeploymentOptions{
					internal: true,
				},
			},
		},
	}
	require.Equal(t, expected, s)

	// Overwrite with addWithOptions().
	s.addWithOptions(Chrome, DiffCalculator, DeploymentOptions{internal: false})
	expected = DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffCalculator,
				},
				DeploymentOptions: DeploymentOptions{
					internal: false,
				},
			},
		},
	}
	require.Equal(t, expected, s)

	// Overwrite with add().
	s.add(Chrome, DiffCalculator)
	expected = DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffCalculator,
				},
			},
		},
	}
	require.Equal(t, expected, s)
}

func TestDeployableUnitSetGet(t *testing.T) {

	// Item not found.
	s := DeployableUnitSet{}
	_, ok := s.Get(DeployableUnitID{Instance: Chrome, Service: DiffCalculator})
	require.False(t, ok)

	// Item found.
	s = DeployableUnitSet{
		deployableUnits: []DeployableUnit{
			{
				DeployableUnitID: DeployableUnitID{
					Instance: Chrome,
					Service:  DiffCalculator,
				},
			},
		},
	}
	unit, ok := s.Get(DeployableUnitID{Instance: Chrome, Service: DiffCalculator})
	expectedUnit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Chrome,
			Service:  DiffCalculator,
		},
	}
	require.True(t, ok)
	require.Equal(t, expectedUnit, unit)
}

func TestDeployableUnitSetKnownInstances(t *testing.T) {
	s := DeployableUnitSet{
		knownInstances: []Instance{Skia, Flutter},
	}
	require.Equal(t, []Instance{Skia, Flutter}, s.KnownInstances())
}

func TestDeployableUnitSetKnownServices(t *testing.T) {
	s := DeployableUnitSet{
		knownServices: []Service{BaselineServer, DiffCalculator, Frontend},
	}
	require.Equal(t, []Service{BaselineServer, DiffCalculator, Frontend}, s.KnownServices())
}

func TestDeployableUnitSetIsKnownInstance(t *testing.T) {
	s := DeployableUnitSet{
		knownInstances: []Instance{Skia, Flutter},
	}
	require.True(t, s.IsKnownInstance(Skia))
	require.True(t, s.IsKnownInstance(Flutter))
	require.False(t, s.IsKnownInstance("foo"))
}

func TestDeployableUnitSetIsKnownService(t *testing.T) {
	s := DeployableUnitSet{
		knownServices: []Service{BaselineServer, DiffCalculator, Frontend},
	}
	require.True(t, s.IsKnownService(BaselineServer))
	require.True(t, s.IsKnownService(DiffCalculator))
	require.True(t, s.IsKnownService(Frontend))
	require.False(t, s.IsKnownService("foo"))
}

// p takes a Unix path and replaces forward slashes with the correct separators
// for the OS under which this test is running.
func p(path string) string {
	return filepath.Join(strings.Split(path, "/")...)
}
