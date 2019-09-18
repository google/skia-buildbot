package goldpushk

import (
	"fmt"
	"path/filepath"
)

const (
	// Paths below are relative to $SKIA_INFRA_ROOT.
	k8sConfigTemplatesDir = "golden/k8s-config-templates"
	k8sInstancesDir       = "golden/k8s-instances"
	configOutDir          = "golden/build"
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
	// TODO(lovisolo): Add any missing fields.
	internal          bool   // If true, deploy to the "skia-corp" cluster, otherwise deploy to "skia-public".
	configMapName     string // If set, a ConfigMap will be created using the contents of field as its name.
	configMapFile     string // File (relative to $SKIA_INFRA_ROOT) from which to create the ConfigMap. Must be set if configMapName is set.
	configMapTemplate string // If set, a ConfigMap file will be generated from this template (relative to $SKIA_INFRA_ROOT), and save it to configMapFile.
}

// DeployableUnit represents a Gold instance/service pair that can be deployed
// to a Kubernetes cluster.
type DeployableUnit struct {
	DeployableUnitID
	DeploymentOptions
}

// getDeploymentFileTemplatePath returns the path to the .yaml template file
// used to generate the deployment file for this DeployableUnit.
func (u *DeployableUnit) getDeploymentFileTemplatePath(skiaInfraRootPath string) string {
	return filepath.Join(skiaInfraRootPath, k8sConfigTemplatesDir, fmt.Sprintf("gold-%s-template.yaml", u.Service))
}

// getDeploymentFilePath returns the path to the deployment file for this
// DeployableUnit.
func (u *DeployableUnit) getDeploymentFilePath(skiaInfraRootPath string) string {
	return filepath.Join(skiaInfraRootPath, configOutDir, fmt.Sprintf("gold-%s-%s.yaml", u.Instance, u.Service))
}

// getConfigMapFileTemplatePath returns the path to the .yaml template file used to
// generate the ConfigMap file for this DeployableUnit.
func (u *DeployableUnit) getConfigMapFileTemplatePath(skiaInfraRootPath string) string {
	return filepath.Join(skiaInfraRootPath, u.configMapTemplate)
}

// getConfigMapFilePath returns the path to the ConfigMap file for this
// DeployableUnit.
func (u *DeployableUnit) getConfigMapFilePath(skiaInfraRootPath string) string {
	return filepath.Join(skiaInfraRootPath, u.configMapFile)
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
