package goldpushk

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.Equal(t, "gold-chrome-diffserver", unit.CanonicalName())
}

func TestDeployableUnitGetDeploymentFileTemplatePath(t *testing.T) {
	unittest.SmallTest(t)

	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: Skia,
			Service:  DiffServer,
		},
	}

	require.Equal(t, p("/foo/bar/golden/k8s-config-templates/gold-diffserver-template.yaml"), unit.getDeploymentFileTemplatePath(p("/foo/bar/golden")))
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
	require.Equal(t, expected, s)
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
	require.Equal(t, expected, s)
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
	require.Equal(t, expected, s)

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
	require.Equal(t, expected, s)

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
	require.Equal(t, expected, s)
}

func TestDeployableUnitSetGet(t *testing.T) {
	unittest.SmallTest(t)

	// Item not found.
	s := DeployableUnitSet{}
	_, ok := s.Get(DeployableUnitID{Instance: Chrome, Service: DiffServer})
	require.False(t, ok)

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
	require.True(t, ok)
	require.Equal(t, expectedUnit, unit)
}

func TestDeployableUnitSetKnownInstances(t *testing.T) {
	unittest.SmallTest(t)
	s := DeployableUnitSet{
		knownInstances: []Instance{Skia, Flutter, Fuchsia},
	}
	require.Equal(t, []Instance{Skia, Flutter, Fuchsia}, s.KnownInstances())
}

func TestDeployableUnitSetKnownServices(t *testing.T) {
	unittest.SmallTest(t)
	s := DeployableUnitSet{
		knownServices: []Service{BaselineServer, DiffServer, Frontend},
	}
	require.Equal(t, []Service{BaselineServer, DiffServer, Frontend}, s.KnownServices())
}

func TestDeployableUnitSetIsKnownInstance(t *testing.T) {
	unittest.SmallTest(t)
	s := DeployableUnitSet{
		knownInstances: []Instance{Skia, Flutter, Fuchsia},
	}
	require.True(t, s.IsKnownInstance(Skia))
	require.True(t, s.IsKnownInstance(Flutter))
	require.True(t, s.IsKnownInstance(Fuchsia))
	require.False(t, s.IsKnownInstance(Instance("foo")))
}

func TestDeployableUnitSetIsKnownService(t *testing.T) {
	unittest.SmallTest(t)
	s := DeployableUnitSet{
		knownServices: []Service{BaselineServer, DiffServer, Frontend},
	}
	require.True(t, s.IsKnownService(BaselineServer))
	require.True(t, s.IsKnownService(DiffServer))
	require.True(t, s.IsKnownService(Frontend))
	require.False(t, s.IsKnownService(Service("foo")))
}

// p takes a Unix path and replaces forward slashes with the correct separators
// for the OS under which this test is running.
func p(path string) string {
	return filepath.Join(strings.Split(path, "/")...)
}
