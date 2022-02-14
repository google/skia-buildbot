package goldpushk

// The contents of this file are goldpushk's source of truth, specifically the DeployableUnitSet
// returned by ProductionDeployableUnits().

const (
	// Gold instances.
	Angle                       Instance = "angle"
	Chrome                      Instance = "chrome"
	ChromePublic                Instance = "chrome-public"
	ChromiumOSTast              Instance = "cros-tast"
	DeprecatedChromiumOSTastDev Instance = "cros-tast-dev"
	ESkia                       Instance = "eskia"
	Flutter                     Instance = "flutter"
	FlutterEngine               Instance = "flutter-engine"
	Lottie                      Instance = "lottie"
	LottieSpec                  Instance = "lottie-spec"
	Pdfium                      Instance = "pdfium"
	Skia                        Instance = "skia"
	SkiaInfra                   Instance = "skia-infra"
	SkiaPublic                  Instance = "skia-public"

	// Gold services.
	BaselineServer  Service = "baselineserver"
	DiffCalculator  Service = "diffcalculator"
	Frontend        Service = "frontend"
	GitilesFollower Service = "gitilesfollower"
	Ingestion       Service = "ingestion"
	PeriodicTasks   Service = "periodictasks"
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
			ChromiumOSTast,
			DeprecatedChromiumOSTastDev,
			ESkia,
			Flutter,
			FlutterEngine,
			Lottie,
			LottieSpec,
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
			s.add(instance, PeriodicTasks)
			s.add(instance, GitilesFollower)
		}
	}

	// Add BaselineServer to the instances that require it.
	publicInstancesNeedingBaselineServer := []Instance{
		Angle, Chrome, ChromiumOSTast, DeprecatedChromiumOSTastDev, ESkia, Flutter, FlutterEngine, Pdfium, SkiaInfra, Skia,
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
