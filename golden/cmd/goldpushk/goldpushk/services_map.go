package goldpushk

import "fmt"

// The contents of this file are goldpushk's source of truth, specifically the DeployableUnitSet
// returned by ProductionDeployableUnits().

const (
	// Gold instances.
	Chrome        Instance = "chrome"
	ChromeGPU     Instance = "chrome-gpu"
	Flutter       Instance = "flutter"
	FlutterEngine Instance = "flutter-engine"
	Fuchsia       Instance = "fuchsia"
	FuchsiaPublic Instance = "fuchsia-public"
	Lottie        Instance = "lottie"
	Pdfium        Instance = "pdfium"
	Skia          Instance = "skia"
	SkiaInfra     Instance = "skia-infra"
	SkiaPublic    Instance = "skia-public"

	// Gold services.
	BaselineServer  Service = "baselineserver"
	DiffServer      Service = "diffserver"
	IngestionBT     Service = "ingestion-bt"
	SkiaCorrectness Service = "skiacorrectness"

	// Testing Gold instances.
	TestInstance1     Instance = "goldpushk-test1"
	TestInstance2     Instance = "goldpushk-test2"
	TestCorpInstance1 Instance = "goldpushk-corp-test1"
	TestCorpInstance2 Instance = "goldpushk-corp-test2"

	// Testing Gold services.
	HealthyTestServer  Service = "healthy-server"
	CrashingTestServer Service = "crashing-server"
)

var (
	// knownPublicInstances is the set of Gold instances that are public.
	//
	// Note: consider rearchitecting this file in a manner that does not require any global state,
	// especially if we add more public instances in the future. For some potential ideas, see Kevin's
	// comments here: https://skia-review.googlesource.com/c/buildbot/+/243778.
	knownPublicInstances = []Instance{
		SkiaPublic,
	}
)

// ProductionDeployableUnits returns the DeployableUnitSet that will be used as the source of truth
// across all of goldpushk.
func ProductionDeployableUnits() DeployableUnitSet {
	s := DeployableUnitSet{
		knownInstances: []Instance{
			Chrome,
			ChromeGPU,
			Flutter,
			FlutterEngine,
			Fuchsia,
			FuchsiaPublic,
			Lottie,
			Pdfium,
			Skia,
			SkiaInfra,
			SkiaPublic,
		},
		knownServices: []Service{
			BaselineServer,
			DiffServer,
			IngestionBT,
			SkiaCorrectness,
		},
	}

	// Add common services to all known instances.
	for _, instance := range s.knownInstances {
		if isPublicInstance(instance) {
			// Add common services for public instances.
			s.addWithOptions(instance, SkiaCorrectness, DeploymentOptions{
				configMapName: fmt.Sprintf("%s-authorized-params", instance),
				configMapFile: "k8s-instances/skia-public/authorized-params.json5",
			})
		} else {
			// Add common services for internal instances.
			s.add(instance, DiffServer)
			s.addWithOptions(instance, IngestionBT, makeDeploymentOptionsForIngestionBT(instance, false))
			s.add(instance, SkiaCorrectness)
		}
	}

	// Add BaselineServer to the instances that require it.
	publicInstancesNeedingBaselineServer := []Instance{
		Chrome, ChromeGPU, Flutter, FlutterEngine, FuchsiaPublic, SkiaInfra,
	}
	for _, instance := range publicInstancesNeedingBaselineServer {
		s.addWithOptions(instance, BaselineServer, DeploymentOptions{
			useJSON5InsteadOfFlags: true,
		})
	}
	// Internal baseline options.
	s.addWithOptions(Fuchsia, BaselineServer, DeploymentOptions{
		internal:               true,
		useJSON5InsteadOfFlags: true,
	})

	// Overwrite common services for "fuchsia" instance, which need to run on skia-corp.
	s.addWithOptions(Fuchsia, DiffServer, DeploymentOptions{internal: true})
	s.addWithOptions(Fuchsia, IngestionBT, makeDeploymentOptionsForIngestionBT(Fuchsia, true))
	s.addWithOptions(Fuchsia, SkiaCorrectness, DeploymentOptions{internal: true})

	return s
}

// makeDeploymentOptionsForIngestionBT builds and returns the deployment options necessary for the
// IngestionBT service corresponding to the given instance.
func makeDeploymentOptionsForIngestionBT(instance Instance, internal bool) DeploymentOptions {
	return DeploymentOptions{
		internal:          internal,
		configMapName:     fmt.Sprintf("gold-%s-ingestion-config-bt", instance),
		configMapTemplate: "k8s-config-templates/ingest-config-template.json5",
	}
}

// isPublicInstance returns true if the given instance is in knownPublicInstances.
func isPublicInstance(instance Instance) bool {
	for _, i := range knownPublicInstances {
		if i == instance {
			return true
		}
	}
	return false
}

// TestingDeployableUnits returns a DeployableUnitSet comprised of dummy services that can be
// deployed without disrupting any production services for the purpose of testing goldpushk.
func TestingDeployableUnits() DeployableUnitSet {
	s := DeployableUnitSet{
		knownInstances: []Instance{
			TestInstance1,
			TestInstance2,
			TestCorpInstance1,
			TestCorpInstance2,
		},
		knownServices: []Service{
			HealthyTestServer,
			CrashingTestServer,
		},
	}

	addHealthyServerInstance := func(instance Instance, service Service, internal bool) {
		s.addWithOptions(instance, service, DeploymentOptions{
			configMapName:     fmt.Sprintf("gold-%s-healthy-server-config", instance),
			configMapTemplate: "cmd/goldpushk/testing/healthy_server/config-template.json5",
			internal:          internal,
		})
	}

	addHealthyServerInstance(TestInstance1, HealthyTestServer, false)
	s.add(TestInstance1, CrashingTestServer)
	addHealthyServerInstance(TestInstance2, HealthyTestServer, false)
	s.add(TestInstance2, CrashingTestServer)
	addHealthyServerInstance(TestCorpInstance1, HealthyTestServer, true)
	s.addWithOptions(TestCorpInstance1, CrashingTestServer, DeploymentOptions{internal: true})
	addHealthyServerInstance(TestCorpInstance2, HealthyTestServer, true)
	s.addWithOptions(TestCorpInstance2, CrashingTestServer, DeploymentOptions{internal: true})

	return s
}
