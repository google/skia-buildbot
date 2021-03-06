package goldpushk

// The contents of this file are goldpushk's source of truth, specifically the DeployableUnitSet
// returned by ProductionDeployableUnits().

const (
	// Gold instances.
	Angle             Instance = "angle"
	Chrome            Instance = "chrome"
	ChromePublic      Instance = "chrome-public"
	ChromiumOSTastDev Instance = "cros-tast-dev"
	Flutter           Instance = "flutter"
	FlutterEngine     Instance = "flutter-engine"
	Lottie            Instance = "lottie"
	Pdfium            Instance = "pdfium"
	Skia              Instance = "skia"
	SkiaInfra         Instance = "skia-infra"
	SkiaPublic        Instance = "skia-public"

	// Gold services.
	BaselineServer  Service = "baselineserver"
	DiffCalculator  Service = "diffcalculator"
	Frontend        Service = "frontend"
	GitilesFollower Service = "gitilesfollower"
	Ingestion       Service = "ingestion"    // New, SQL based ingestion
	IngestionBT     Service = "ingestion-bt" // Deprecated, BigTable based ingestion
	PeriodicTasks   Service = "periodictasks"

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
		ChromePublic, SkiaPublic,
	}
)

// ProductionDeployableUnits returns the DeployableUnitSet that will be used as the source of truth
// across all of goldpushk.
func ProductionDeployableUnits() DeployableUnitSet {
	s := DeployableUnitSet{
		knownInstances: []Instance{
			Angle,
			Chrome,
			ChromePublic,
			ChromiumOSTastDev,
			Flutter,
			FlutterEngine,
			Lottie,
			Pdfium,
			Skia,
			SkiaInfra,
			SkiaPublic,
		},
		knownServices: []Service{
			BaselineServer,
			DiffCalculator,
			Frontend,
			GitilesFollower,
			Ingestion,
			IngestionBT,
			PeriodicTasks,
		},
	}

	// Add common services to all known instances.
	for _, instance := range s.knownInstances {
		if isPublicInstance(instance) {
			// There is only one service for public view instances: - frontend.
			s.add(instance, Frontend)
		} else {
			// Add common services for regular instances.
			s.add(instance, DiffCalculator)
			s.add(instance, Frontend)
			s.add(instance, Ingestion)
			s.add(instance, IngestionBT)
			s.add(instance, PeriodicTasks)
			s.add(instance, GitilesFollower)
		}
	}

	// Add BaselineServer to the instances that require it.
	publicInstancesNeedingBaselineServer := []Instance{
		Angle, Chrome, ChromiumOSTastDev, Flutter, FlutterEngine, Pdfium, SkiaInfra,
	}
	for _, instance := range publicInstancesNeedingBaselineServer {
		s.add(instance, BaselineServer)
	}
	return s
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

// TestingDeployableUnits returns a DeployableUnitSet comprised of test services that can be
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
			internal: internal,
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
