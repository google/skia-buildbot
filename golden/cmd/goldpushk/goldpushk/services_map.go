// The contents of this file are goldpushk's source of truth, specifically the
// GoldServicesMap returned by BuildServicesMap().
//
// When updating the map, remember to update services_map_test.go with any
// invariants you would like to enforce (e.g. all services of instance X should
// run in skia-corp.)

package goldpushk

const (
	// Gold instances.
	Chrome     GoldInstance = "chrome"
	ChromeGPU  GoldInstance = "chrome-gpu"
	Flutter    GoldInstance = "flutter"
	Fuchsia    GoldInstance = "fuchsia"
	Lottie     GoldInstance = "lottie"
	Pdfium     GoldInstance = "pdfium"
	Skia       GoldInstance = "skia"
	SkiaPublic GoldInstance = "skia-public"

	// Gold services.
	BaselineServer  GoldService = "baselineserver"
	DiffServer      GoldService = "diffserver"
	Ingestion       GoldService = "ingestion"
	IngestionBT     GoldService = "ingestion-bt"
	SkiaCorrectness GoldService = "skiacorrectness"
	TraceServer     GoldService = "traceserver"
)

var (
	// Known instances. Should be kept in sync with the constants defined above.
	KnownGoldInstances = []GoldInstance{
		Chrome,
		ChromeGPU,
		Flutter,
		Fuchsia,
		Lottie,
		Pdfium,
		Skia,
		SkiaPublic,
	}

	// Known services. Should be kept in sync with the constants defined above.
	KnownGoldServices = []GoldService{
		BaselineServer,
		DiffServer,
		Ingestion,
		IngestionBT,
		SkiaCorrectness,
		TraceServer,
	}
)

// Returns the GoldServicesMap that will be used as the source of truth across goldpushk.
// TODO(lovisolo): Add any missing information.
func BuildServicesMap() GoldServicesMap {
	m := make(GoldServicesMap)

	// Populate "chrome" instance.
	m.AddDeployment(Chrome, BaselineServer, GoldServiceDeployment{})
	m.AddDeployment(Chrome, DiffServer, GoldServiceDeployment{})
	m.AddDeployment(Chrome, IngestionBT, GoldServiceDeployment{ConfigMapName: "gold-chrome-ingestion-config-bt"})
	m.AddDeployment(Chrome, SkiaCorrectness, GoldServiceDeployment{})

	// Populate "chrome-gpu" instance.
	m.AddDeployment(ChromeGPU, BaselineServer, GoldServiceDeployment{})
	m.AddDeployment(ChromeGPU, DiffServer, GoldServiceDeployment{})
	m.AddDeployment(ChromeGPU, IngestionBT, GoldServiceDeployment{ConfigMapName: "gold-chrome-gpu-ingestion-config-bt"})
	m.AddDeployment(ChromeGPU, SkiaCorrectness, GoldServiceDeployment{})

	// Populate "flutter" instance.
	m.AddDeployment(Flutter, BaselineServer, GoldServiceDeployment{})
	m.AddDeployment(Flutter, DiffServer, GoldServiceDeployment{})
	m.AddDeployment(Flutter, IngestionBT, GoldServiceDeployment{ConfigMapName: "gold-flutter-ingestion-config-bt"})
	m.AddDeployment(Flutter, SkiaCorrectness, GoldServiceDeployment{})

	// Populate "fuchsia" instance.
	m.AddDeployment(Fuchsia, BaselineServer, GoldServiceDeployment{Internal: true})
	m.AddDeployment(Fuchsia, DiffServer, GoldServiceDeployment{Internal: true})
	m.AddDeployment(Fuchsia, Ingestion, GoldServiceDeployment{Internal: true, ConfigMapName: "gold-fuchsia-ingestion-config"})
	m.AddDeployment(Fuchsia, IngestionBT, GoldServiceDeployment{Internal: true, ConfigMapName: "gold-fuchsia-ingestion-config-bt"})
	m.AddDeployment(Fuchsia, SkiaCorrectness, GoldServiceDeployment{Internal: true})
	m.AddDeployment(Fuchsia, TraceServer, GoldServiceDeployment{Internal: true})

	// Populate "lottie" instance.
	m.AddDeployment(Lottie, DiffServer, GoldServiceDeployment{})
	m.AddDeployment(Lottie, IngestionBT, GoldServiceDeployment{})
	m.AddDeployment(Lottie, SkiaCorrectness, GoldServiceDeployment{})

	// Populate "pdfium" instance.
	m.AddDeployment(Pdfium, DiffServer, GoldServiceDeployment{})
	m.AddDeployment(Pdfium, IngestionBT, GoldServiceDeployment{ConfigMapName: "gold-pdfium-ingestion-config-bt"})
	m.AddDeployment(Pdfium, SkiaCorrectness, GoldServiceDeployment{})

	// Populate "skia" instance.
	m.AddDeployment(Skia, DiffServer, GoldServiceDeployment{})
	m.AddDeployment(Skia, IngestionBT, GoldServiceDeployment{ConfigMapName: "gold-skia-ingestion-config-bt"})
	m.AddDeployment(Skia, SkiaCorrectness, GoldServiceDeployment{})

	// Populate "skia-public" instance.
	m.AddDeployment(SkiaPublic, SkiaCorrectness, GoldServiceDeployment{ConfigMapName: "skia-public-authorized-params"})

	return m
}

// Returns true if the given Gold instance is in the list of known services.
func IsKnownGoldInstance(instance GoldInstance) bool {
	for _, validInstance := range KnownGoldInstances {
		if instance == validInstance {
			return true
		}
	}
	return false
}

// Returns true if the given Gold service is in the list of known services.
func IsKnownGoldService(service GoldService) bool {
	for _, validService := range KnownGoldServices {
		if service == validService {
			return true
		}
	}
	return false
}
