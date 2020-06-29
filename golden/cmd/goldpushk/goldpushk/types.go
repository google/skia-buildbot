package goldpushk

import (
	"fmt"
	"path/filepath"
)

// Instance represents the name of a Gold instance, e.g. "skia".
type Instance string

// Service represents the name of a Gold service, e.g. "diffserver".
type Service string

// DeployableUnitID identifies a Gold instance/service pair.
type DeployableUnitID struct {
	Instance Instance
	Service  Service
}

// CanonicalName returns the canonical name of a Gold instance/service pair,
// e.g. "gold-skia-diffserver".
//
// Among other things, this is used to determine the configuration file
// corresponding to a DeployableUnit, e.g. "gold-skia-diffserver.yaml".
func (d DeployableUnitID) CanonicalName() string {
	return fmt.Sprintf("gold-%s-%s", d.Instance, d.Service)
}

// DeploymentOptions contains any additional information required to deploy a
// DeployableUnit to Kubernetes.
type DeploymentOptions struct {
	internal bool // If true, deploy to the "skia-corp" cluster, otherwise deploy to "skia-public".
}

// DeployableUnit represents a Gold instance/service pair that can be deployed
// to a Kubernetes cluster.
type DeployableUnit struct {
	DeployableUnitID
	DeploymentOptions
}

// getDeploymentFileTemplatePath returns the path to the .yaml template file
// used to generate the deployment file for this DeployableUnit.
func (u *DeployableUnit) getDeploymentFileTemplatePath(goldSrcDir string) string {
	return filepath.Join(goldSrcDir, k8sConfigTemplatesDir, fmt.Sprintf("gold-%s-template.yaml", u.Service))
}

// DeployableUnitSet implements a set data structure for DeployableUnits, and contains information
// about all the known Gold instances and services. This structure is intended to be read-only
// outside of this package.
type DeployableUnitSet struct {
	knownInstances  []Instance
	knownServices   []Service
	deployableUnits []DeployableUnit
}

// add builds a DeployableUnit using the provided instance and service names
// with default DeploymentOptions, and adds it to the set.
//
// If the set already contains a DeployableUnit with the same DeployableUnitID,
// it will be overwritten.
func (s *DeployableUnitSet) add(instance Instance, service Service) {
	s.addWithOptions(instance, service, DeploymentOptions{})
}

// addWithOptions builds a DeployableUnit using the provided instance and
// service names and DeploymentOptions, and adds it to the set.
//
// If the set already contains a DeployableUnit with the same DeployableUnitID,
// it will be overwritten.
func (s *DeployableUnitSet) addWithOptions(instance Instance, service Service, options DeploymentOptions) {
	unit := DeployableUnit{
		DeployableUnitID: DeployableUnitID{
			Instance: instance,
			Service:  service,
		},
		DeploymentOptions: options,
	}

	// Overwrite if it already exists, then return.
	for i, existingUnit := range s.deployableUnits {
		if existingUnit.DeployableUnitID == unit.DeployableUnitID {
			s.deployableUnits[i] = unit
			return
		}
	}

	s.deployableUnits = append(s.deployableUnits, unit)
}

// Get finds and returns DeployableUnits by DeployableUnitID. If not found, the
// returned bool will be set to false.
func (s *DeployableUnitSet) Get(d DeployableUnitID) (DeployableUnit, bool) {
	for _, unit := range s.deployableUnits {
		if unit.DeployableUnitID == d {
			return unit, true
		}
	}
	return DeployableUnit{}, false
}

// KnownInstances returns the list of Gold instances known by the DeployableUnitSet.
func (s *DeployableUnitSet) KnownInstances() []Instance {
	return s.knownInstances
}

// KnownServices returns the list of Gold services known by the DeployableUnitSet.
func (s *DeployableUnitSet) KnownServices() []Service {
	return s.knownServices
}

// IsKnownInstance returns true if the given Gold instance is known by the DeployableUnitSet.
func (s *DeployableUnitSet) IsKnownInstance(instance Instance) bool {
	for _, validInstance := range s.knownInstances {
		if instance == validInstance {
			return true
		}
	}
	return false
}

// IsKnownService returns true if the given Gold service is known by the DeployableUnitSet.
func (s *DeployableUnitSet) IsKnownService(service Service) bool {
	for _, validService := range s.knownServices {
		if service == validService {
			return true
		}
	}
	return false
}
