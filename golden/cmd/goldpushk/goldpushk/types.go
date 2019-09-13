package goldpushk

import (
	"fmt"
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

// DeploymentOptions contains all the necessary information to deploy a
// DeployableUnit to Kubernetes.
type DeploymentOptions struct {
	// TODO(lovisolo): Add any missing fields.
	internal      bool // If true, deploy to the "skia-corp" cluster, otherwise deploy to "skia-public".
	configMapName string
}

// DeployableUnit represents a Gold instance/service pair that can be deployed
// to a Kubernetes cluster.
type DeployableUnit struct {
	DeployableUnitID
	DeploymentOptions
}

// DeployableUnitSet implements a set data structure for DeployableUnits. This
// structure is intended to be read-only outside of this package.
type DeployableUnitSet struct {
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
