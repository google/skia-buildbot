package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/cmd/goldpushk/goldpushk"
)

func TestParseAndValidateFlagsErrors(t *testing.T) {
	unittest.SmallTest(t)

	testCases := []struct {
		message string // Test case name.

		// Inputs.
		flagInstances []string
		flagServices  []string
		flagCanaries  []string

		// Expected outputs.
		errorMsg string
	}{

		{
			message:       "Error: --instances all,chrome",
			flagInstances: []string{"all", "chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{},
			errorMsg:      "flag --instances should contain either \"all\" or a list of Gold instances, but not both",
		},

		{
			message:       "Error: --services all,baselineserver",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"all", "baselineserver"},
			flagCanaries:  []string{},
			errorMsg:      "flag --services should contain either \"all\" or a list of Gold services, but not both",
		},

		{
			message:       "Error: --instances and --services both set to \"all\"",
			flagInstances: []string{"all"},
			flagServices:  []string{"all"},
			flagCanaries:  []string{},
			errorMsg:      "cannot set both --instances and --services to \"all\"",
		},

		{
			message:       "Error: Unknown instance",
			flagInstances: []string{"foo"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{},
			errorMsg:      "unknown Gold instance: \"foo\"",
		},

		{
			message:       "Error: Unknown service",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"foo"},
			flagCanaries:  []string{},
			errorMsg:      "unknown Gold service: \"foo\"",
		},

		{
			message:       "Error: No instances/services matched.",
			flagInstances: []string{"skia"},
			flagServices:  []string{"baselineserver"},
			errorMsg:      "no known Gold services match the values supplied with --instances and --services",
		},

		{
			message:       "Error: Invalid canary format",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"xxxxxxxxx"},
			errorMsg:      "invalid canary format: \"xxxxxxxxx\"",
		},

		{
			message:       "Error: Invalid canary due to unknown instance",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"foo:baselineserver"},
			errorMsg:      "invalid canary - unknown Gold instance: \"foo:baselineserver\"",
		},

		{
			message:       "Error: Invalid canary due to unknown service",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"chrome:foo"},
			errorMsg:      "invalid canary - unknown Gold service: \"chrome:foo\"",
		},

		{
			message:       "Error: Canary doesn't match --instances / --services",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"skia:diffserver"},
			errorMsg:      "canary does not match any targeted services: \"skia:diffserver\"",
		},

		{
			message:       "Error: All targeted services are canaried",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"chrome:baselineserver"},
			errorMsg:      "all targeted services are marked for canarying",
		},
	}

	for _, tc := range testCases {
		_, _, err := parseAndValidateFlags(goldpushk.ProductionDeployableUnits(), tc.flagInstances, tc.flagServices, tc.flagCanaries)
		require.Error(t, err, tc.message)
		require.Contains(t, err.Error(), tc.errorMsg, tc.message)
	}
}

func TestParseAndValidateFlagsSuccess(t *testing.T) {
	unittest.SmallTest(t)

	// Deployments shared among test cases.
	chromeBaselineServer := makeID(goldpushk.Chrome, goldpushk.BaselineServer)
	chromeDiffServer := makeID(goldpushk.Chrome, goldpushk.DiffServer)
	chromeGpuDiffServer := makeID(goldpushk.ChromeGPU, goldpushk.DiffServer)
	chromeGpuSkiaCorrectness := makeID(goldpushk.ChromeGPU, goldpushk.SkiaCorrectness)
	chromeIngestionBT := makeID(goldpushk.Chrome, goldpushk.IngestionBT)
	chromeSkiaCorrectness := makeID(goldpushk.Chrome, goldpushk.SkiaCorrectness)
	chromePublicSkiaCorrectness := makeID(goldpushk.ChromePublic, goldpushk.SkiaCorrectness)
	flutterDiffServer := makeID(goldpushk.Flutter, goldpushk.DiffServer)
	flutterEngineDiffServer := makeID(goldpushk.FlutterEngine, goldpushk.DiffServer)
	flutterEngineSkiaCorrectness := makeID(goldpushk.FlutterEngine, goldpushk.SkiaCorrectness)
	flutterSkiaCorrectness := makeID(goldpushk.Flutter, goldpushk.SkiaCorrectness)
	fuchsiaDiffServer := makeID(goldpushk.Fuchsia, goldpushk.DiffServer)
	fuchsiaPublicDiffServer := makeID(goldpushk.FuchsiaPublic, goldpushk.DiffServer)
	fuchsiaPublicSkiaCorrectness := makeID(goldpushk.FuchsiaPublic, goldpushk.SkiaCorrectness)
	fuchsiaSkiaCorrectness := makeID(goldpushk.Fuchsia, goldpushk.SkiaCorrectness)
	lottieDiffServer := makeID(goldpushk.Lottie, goldpushk.DiffServer)
	lottieSkiaCorrectness := makeID(goldpushk.Lottie, goldpushk.SkiaCorrectness)
	pdfiumDiffServer := makeID(goldpushk.Pdfium, goldpushk.DiffServer)
	pdfiumSkiaCorrectness := makeID(goldpushk.Pdfium, goldpushk.SkiaCorrectness)
	skiaDiffServer := makeID(goldpushk.Skia, goldpushk.DiffServer)
	skiaInfraDiffServer := makeID(goldpushk.SkiaInfra, goldpushk.DiffServer)
	skiaInfraSkiaCorrectness := makeID(goldpushk.SkiaInfra, goldpushk.SkiaCorrectness)
	skiaIngestionBT := makeID(goldpushk.Skia, goldpushk.IngestionBT)
	skiaPublicSkiaCorrectness := makeID(goldpushk.SkiaPublic, goldpushk.SkiaCorrectness)
	skiaSkiaCorrectness := makeID(goldpushk.Skia, goldpushk.SkiaCorrectness)

	testCases := []struct {
		message string // Test case name.

		// Inputs.
		flagInstances []string
		flagServices  []string
		flagCanaries  []string

		// Expected outputs.
		expectedDeployableUnitIDs         []goldpushk.DeployableUnitID
		expectedCanariedDeployableUnitIDs []goldpushk.DeployableUnitID
	}{

		////////////////////////////////////////////////////////////////////////////////////////////////
		// No wildcards                                                                               //
		////////////////////////////////////////////////////////////////////////////////////////////////

		{
			message:                   "Single instance, single service, no canary",
			flagInstances:             []string{"chrome"},
			flagServices:              []string{"baselineserver"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeBaselineServer},
		},

		{
			message:                   "Single instance, multiple services, no canary",
			flagInstances:             []string{"chrome"},
			flagServices:              []string{"baselineserver", "diffserver"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer},
		},

		{
			message:                           "Single instance, multiple services, one canary",
			flagInstances:                     []string{"chrome"},
			flagServices:                      []string{"baselineserver", "diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"chrome:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeSkiaCorrectness},
		},

		{
			message:                           "Single instance, multiple services, multiple canaries",
			flagInstances:                     []string{"chrome"},
			flagServices:                      []string{"baselineserver", "diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"chrome:diffserver", "chrome:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeBaselineServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:                   "Multiple instances, single service, no canary",
			flagInstances:             []string{"chrome", "skia", "skia-public"},
			flagServices:              []string{"skiacorrectness"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeSkiaCorrectness, skiaSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		{
			message:                           "Multiple instances, single service, one canary",
			flagInstances:                     []string{"chrome", "skia", "skia-public"},
			flagServices:                      []string{"skiacorrectness"},
			flagCanaries:                      []string{"skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaPublicSkiaCorrectness},
		},

		{
			message:                           "Multiple instances, single service, multiple canaries",
			flagInstances:                     []string{"chrome", "skia", "skia-public"},
			flagServices:                      []string{"skiacorrectness"},
			flagCanaries:                      []string{"skia:skiacorrectness", "skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		{
			message:                   "Multiple instances, multiple services, no canary",
			flagInstances:             []string{"chrome", "skia", "skia-public"},
			flagServices:              []string{"diffserver", "skiacorrectness"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		{
			message:                           "Multiple instances, multiple services, one canary",
			flagInstances:                     []string{"chrome", "skia", "skia-public"},
			flagServices:                      []string{"diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaPublicSkiaCorrectness},
		},

		{
			message:                           "Multiple instances, multiple services, multiple canaries",
			flagInstances:                     []string{"chrome", "skia", "skia-public"},
			flagServices:                      []string{"diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"skia:skiacorrectness", "skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		////////////////////////////////////////////////////////////////////////////////////////////////
		// Wildcard: --service all                                                                    //
		////////////////////////////////////////////////////////////////////////////////////////////////

		{
			message:                   "Single instance, all services, no canary",
			flagInstances:             []string{"chrome"},
			flagServices:              []string{"all"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer, chromeIngestionBT, chromeSkiaCorrectness},
		},

		{
			message:                           "Single instance, all services, one canary",
			flagInstances:                     []string{"chrome"},
			flagServices:                      []string{"all"},
			flagCanaries:                      []string{"chrome:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer, chromeIngestionBT},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeSkiaCorrectness},
		},

		{
			message:                           "Single instance, all services, multiple canaries",
			flagInstances:                     []string{"chrome"},
			flagServices:                      []string{"all"},
			flagCanaries:                      []string{"chrome:ingestion-bt", "chrome:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeIngestionBT, chromeSkiaCorrectness},
		},

		{
			message:                   "Multiple instances, all services, no canary",
			flagInstances:             []string{"chrome", "skia"},
			flagServices:              []string{"all"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer, chromeIngestionBT, chromeSkiaCorrectness, skiaDiffServer, skiaIngestionBT, skiaSkiaCorrectness},
		},

		{
			message:                           "Multiple instances, all services, one canary",
			flagInstances:                     []string{"chrome", "skia"},
			flagServices:                      []string{"all"},
			flagCanaries:                      []string{"skia:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer, chromeIngestionBT, chromeSkiaCorrectness, skiaDiffServer, skiaIngestionBT},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaSkiaCorrectness},
		},

		{
			message:                           "Multiple instances, all services, multiple canaries",
			flagInstances:                     []string{"chrome", "skia"},
			flagServices:                      []string{"all"},
			flagCanaries:                      []string{"skia:ingestion-bt", "skia:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer, chromeIngestionBT, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaIngestionBT, skiaSkiaCorrectness},
		},

		////////////////////////////////////////////////////////////////////////////////////////////////
		// Wildcard: --instance all                                                                   //
		////////////////////////////////////////////////////////////////////////////////////////////////

		{
			message:                   "All instances, single service, no canary",
			flagInstances:             []string{"all"},
			flagServices:              []string{"skiacorrectness"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeSkiaCorrectness, chromeGpuSkiaCorrectness, chromePublicSkiaCorrectness, flutterSkiaCorrectness, flutterEngineSkiaCorrectness, fuchsiaSkiaCorrectness, fuchsiaPublicSkiaCorrectness, lottieSkiaCorrectness, pdfiumSkiaCorrectness, skiaSkiaCorrectness, skiaInfraSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		{
			message:                           "All instances, single service, one canary",
			flagInstances:                     []string{"all"},
			flagServices:                      []string{"skiacorrectness"},
			flagCanaries:                      []string{"skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeSkiaCorrectness, chromeGpuSkiaCorrectness, chromePublicSkiaCorrectness, flutterSkiaCorrectness, flutterEngineSkiaCorrectness, fuchsiaSkiaCorrectness, fuchsiaPublicSkiaCorrectness, lottieSkiaCorrectness, pdfiumSkiaCorrectness, skiaSkiaCorrectness, skiaInfraSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaPublicSkiaCorrectness},
		},

		{
			message:                           "All instances, single service, multiple canaries",
			flagInstances:                     []string{"all"},
			flagServices:                      []string{"skiacorrectness"},
			flagCanaries:                      []string{"skia-infra:skiacorrectness", "skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeSkiaCorrectness, chromeGpuSkiaCorrectness, chromePublicSkiaCorrectness, flutterSkiaCorrectness, flutterEngineSkiaCorrectness, fuchsiaSkiaCorrectness, fuchsiaPublicSkiaCorrectness, lottieSkiaCorrectness, pdfiumSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaInfraSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		{
			message:                   "All instances, multiple services, no canary",
			flagInstances:             []string{"all"},
			flagServices:              []string{"diffserver", "skiacorrectness"},
			flagCanaries:              []string{},
			expectedDeployableUnitIDs: []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, chromeGpuDiffServer, chromeGpuSkiaCorrectness, chromePublicSkiaCorrectness, flutterDiffServer, flutterSkiaCorrectness, flutterEngineDiffServer, flutterEngineSkiaCorrectness, fuchsiaDiffServer, fuchsiaSkiaCorrectness, fuchsiaPublicDiffServer, fuchsiaPublicSkiaCorrectness, lottieDiffServer, lottieSkiaCorrectness, pdfiumDiffServer, pdfiumSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaInfraDiffServer, skiaInfraSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		{
			message:                           "All instances, multiple services, one canary",
			flagInstances:                     []string{"all"},
			flagServices:                      []string{"diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, chromeGpuDiffServer, chromeGpuSkiaCorrectness, chromePublicSkiaCorrectness, flutterDiffServer, flutterSkiaCorrectness, flutterEngineDiffServer, flutterEngineSkiaCorrectness, fuchsiaDiffServer, fuchsiaSkiaCorrectness, fuchsiaPublicDiffServer, fuchsiaPublicSkiaCorrectness, lottieDiffServer, lottieSkiaCorrectness, pdfiumDiffServer, pdfiumSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaInfraDiffServer, skiaInfraSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaPublicSkiaCorrectness},
		},

		{
			message:                           "All instances, multiple services, multiple canaries",
			flagInstances:                     []string{"all"},
			flagServices:                      []string{"diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"skia-infra:skiacorrectness", "skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, chromeGpuDiffServer, chromeGpuSkiaCorrectness, chromePublicSkiaCorrectness, flutterDiffServer, flutterSkiaCorrectness, flutterEngineDiffServer, flutterEngineSkiaCorrectness, fuchsiaDiffServer, fuchsiaSkiaCorrectness, fuchsiaPublicDiffServer, fuchsiaPublicSkiaCorrectness, lottieDiffServer, lottieSkiaCorrectness, pdfiumDiffServer, pdfiumSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaInfraDiffServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaInfraSkiaCorrectness, skiaPublicSkiaCorrectness},
		},

		////////////////////////////////////////////////////////////////////////////////////////////////
		// Miscellaneous                                                                              //
		////////////////////////////////////////////////////////////////////////////////////////////////

		{
			message:                           "Repeated inputs are ignored",
			flagInstances:                     []string{"chrome", "chrome", "skia", "chrome", "skia", "skia-public", "skia-public"},
			flagServices:                      []string{"diffserver", "skiacorrectness", "diffserver", "skiacorrectness"},
			flagCanaries:                      []string{"skia:diffserver", "skia-public:skiacorrectness", "skia:diffserver", "skia-public:skiacorrectness"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaDiffServer, skiaPublicSkiaCorrectness},
		},

		{
			message:                           "Outputs sorted by instance, then service",
			flagInstances:                     []string{"skia-public", "chrome", "skia"},
			flagServices:                      []string{"skiacorrectness", "diffserver"},
			flagCanaries:                      []string{"skia-public:skiacorrectness", "skia:diffserver"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{skiaDiffServer, skiaPublicSkiaCorrectness},
		},
	}

	for _, tc := range testCases {
		deployableUnits, canariedDeployableUnits, err := parseAndValidateFlags(goldpushk.ProductionDeployableUnits(), tc.flagInstances, tc.flagServices, tc.flagCanaries)
		deployableUnitIDs := mapUnitsToIDs(deployableUnits)
		canariedDeployableUnitIDs := mapUnitsToIDs(canariedDeployableUnits)

		require.NoError(t, err, tc.message)
		assert.Equal(t, tc.expectedDeployableUnitIDs, deployableUnitIDs, tc.message)
		assert.Equal(t, tc.expectedCanariedDeployableUnitIDs, canariedDeployableUnitIDs, tc.message)
	}
}

func TestParseAndValidateFlagsTestingSuccess(t *testing.T) {
	unittest.SmallTest(t)

	// Testing deployments on skia-public.
	testInstance1HealthyServer := makeID(goldpushk.TestInstance1, goldpushk.HealthyTestServer)
	testInstance1CrashingServer := makeID(goldpushk.TestInstance1, goldpushk.CrashingTestServer)
	testInstance2HealthyServer := makeID(goldpushk.TestInstance2, goldpushk.HealthyTestServer)
	testInstance2CrashingServer := makeID(goldpushk.TestInstance2, goldpushk.CrashingTestServer)

	// Testing deployments on skia-corp
	testCorpInstance1HealthyServer := makeID(goldpushk.TestCorpInstance1, goldpushk.HealthyTestServer)
	testCorpInstance1CrashingServer := makeID(goldpushk.TestCorpInstance1, goldpushk.CrashingTestServer)
	testCorpInstance2HealthyServer := makeID(goldpushk.TestCorpInstance2, goldpushk.HealthyTestServer)
	testCorpInstance2CrashingServer := makeID(goldpushk.TestCorpInstance2, goldpushk.CrashingTestServer)

	testCases := []struct {
		message string // Test case name.

		// Inputs.
		flagInstances []string
		flagServices  []string
		flagCanaries  []string

		// Expected outputs.
		expectedDeployableUnitIDs         []goldpushk.DeployableUnitID
		expectedCanariedDeployableUnitIDs []goldpushk.DeployableUnitID
	}{
		{
			message:                           "Testing, all instances, multiple services, multiple canaries",
			flagInstances:                     []string{"all"},
			flagServices:                      []string{"healthy-server", "crashing-server"},
			flagCanaries:                      []string{"goldpushk-test1:healthy-server", "goldpushk-test1:crashing-server"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{testCorpInstance1CrashingServer, testCorpInstance1HealthyServer, testCorpInstance2CrashingServer, testCorpInstance2HealthyServer, testInstance2CrashingServer, testInstance2HealthyServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{testInstance1CrashingServer, testInstance1HealthyServer},
		},

		{
			message:                           "Testing, multiple instances, all services, multiple canaries",
			flagInstances:                     []string{"goldpushk-test1", "goldpushk-test2", "goldpushk-corp-test1", "goldpushk-corp-test2"},
			flagServices:                      []string{"all"},
			flagCanaries:                      []string{"goldpushk-test1:healthy-server", "goldpushk-test1:crashing-server"},
			expectedDeployableUnitIDs:         []goldpushk.DeployableUnitID{testCorpInstance1CrashingServer, testCorpInstance1HealthyServer, testCorpInstance2CrashingServer, testCorpInstance2HealthyServer, testInstance2CrashingServer, testInstance2HealthyServer},
			expectedCanariedDeployableUnitIDs: []goldpushk.DeployableUnitID{testInstance1CrashingServer, testInstance1HealthyServer},
		},
	}

	for _, tc := range testCases {
		deployableUnits, canariedDeployableUnits, err := parseAndValidateFlags(goldpushk.TestingDeployableUnits(), tc.flagInstances, tc.flagServices, tc.flagCanaries)
		deployableUnitIDs := mapUnitsToIDs(deployableUnits)
		canariedDeployableUnitIDs := mapUnitsToIDs(canariedDeployableUnits)

		require.NoError(t, err, tc.message)
		require.Equal(t, tc.expectedDeployableUnitIDs, deployableUnitIDs, tc.message)
		require.Equal(t, tc.expectedCanariedDeployableUnitIDs, canariedDeployableUnitIDs, tc.message)
	}
}

func makeID(instance goldpushk.Instance, service goldpushk.Service) goldpushk.DeployableUnitID {
	return goldpushk.DeployableUnitID{
		Instance: instance,
		Service:  service,
	}
}

func mapUnitsToIDs(units []goldpushk.DeployableUnit) []goldpushk.DeployableUnitID {
	var ids []goldpushk.DeployableUnitID
	for _, unit := range units {
		ids = append(ids, unit.DeployableUnitID)
	}
	return ids
}
