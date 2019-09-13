package goldpushk

import "fmt"

// The contents of this file are goldpushk's source of truth, specifically the
// DeployableUnitSet returned by BuildDeployableUnitSet().

const (
	// Gold instances.
	Chrome     Instance = "chrome"
	ChromeGPU  Instance = "chrome-gpu"
	Flutter    Instance = "flutter"
	Fuchsia    Instance = "fuchsia"
	Lottie     Instance = "lottie"
	Pdfium     Instance = "pdfium"
	Skia       Instance = "skia"
	SkiaPublic Instance = "skia-public"

	// Gold services.
	BaselineServer  Service = "baselineserver"
	DiffServer      Service = "diffserver"
	IngestionBT     Service = "ingestion-bt"
	SkiaCorrectness Service = "skiacorrectness"
)

var (
	// KnownInstances lists all known Gold instances. Should be kept in sync with
	// the constants defined above.
	KnownInstances = []Instance{
		Chrome,
		ChromeGPU,
		Flutter,
		Fuchsia,
		Lottie,
		Pdfium,
		Skia,
		SkiaPublic,
	}

	// knownPublicInstances is the subset of the KnownInstances that are public.
	knownPublicInstances = []Instance{
		SkiaPublic,
	}

	// KnownServices lists all known Gold services. Should be kept in sync with
	// the constants defined above.
	KnownServices = []Service{
		BaselineServer,
		DiffServer,
		IngestionBT,
		SkiaCorrectness,
	}
)

// BuildDeployableUnitSet returns the DeployableUnitSet that will be used as the
// source of truth across all of goldpushk.
func BuildDeployableUnitSet() DeployableUnitSet {
	// TODO(lovisolo): Add any missing information.

	s := DeployableUnitSet{}

	// Add common services to all known instances.
	for _, instance := range KnownInstances {
		if isPublicInstance(instance) {
			// Add common services for public instances.
			s.addWithOptions(instance, SkiaCorrectness, DeploymentOptions{configMapName: fmt.Sprintf("%s-authorized-params", instance)})
		} else {
			// Add common services for internal instances.
			s.add(instance, DiffServer)
			s.addWithOptions(instance, IngestionBT, DeploymentOptions{configMapName: fmt.Sprintf("gold-%s-ingestion-config-bt", instance)})
			s.add(instance, SkiaCorrectness)
		}
	}

	// Add BaselineServer to the instances that require it.
	s.add(Chrome, BaselineServer)
	s.add(ChromeGPU, BaselineServer)
	s.add(Flutter, BaselineServer)
	s.addWithOptions(Fuchsia, BaselineServer, DeploymentOptions{internal: true})

	// Overwrite common services for "fuchsia" instance, which need to run on skia-corp.
	s.addWithOptions(Fuchsia, DiffServer, DeploymentOptions{internal: true})
	s.addWithOptions(Fuchsia, IngestionBT, DeploymentOptions{internal: true, configMapName: "gold-fuchsia-ingestion-config-bt"})
	s.addWithOptions(Fuchsia, SkiaCorrectness, DeploymentOptions{internal: true})

	return s
}

// IsKnownInstance returns true if the given instance is in KnownInstances.
func IsKnownInstance(instance Instance) bool {
	for _, validInstance := range KnownInstances {
		if instance == validInstance {
			return true
		}
	}
	return false
}

// IsKnownService returns true if the given service is in KnownServices.
func IsKnownService(service Service) bool {
	for _, validService := range KnownServices {
		if service == validService {
			return true
		}
	}
	return false
}

// isPublicInstance returns true if the given instance is in
// knownPublicInstances.
func isPublicInstance(instance Instance) bool {
	for _, i := range knownPublicInstances {
		if i == instance {
			return true
		}
	}
	return false
}
