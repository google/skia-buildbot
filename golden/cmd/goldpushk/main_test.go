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
			flagCanaries:  []string{"skia:diffcalculator"},
			errorMsg:      "canary does not match any targeted services: \"skia:diffcalculator\"",
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
	angleBaselineServer := makeID(goldpushk.Angle, goldpushk.BaselineServer)
	angleFrontend := makeID(goldpushk.Angle, goldpushk.Frontend)
	chromeBaselineServer := makeID(goldpushk.Chrome, goldpushk.BaselineServer)
	chromeDiffCalculator := makeID(goldpushk.Chrome, goldpushk.DiffCalculator)
	chromeFrontend := makeID(goldpushk.Chrome, goldpushk.Frontend)
	chromeIngestion := makeID(goldpushk.Chrome, goldpushk.Ingestion)
	chromeIngestionBT := makeID(goldpushk.Chrome, goldpushk.IngestionBT)
	chromeGitilesFollower := makeID(goldpushk.Chrome, goldpushk.GitilesFollower)
	chromePeriodicTasks := makeID(goldpushk.Chrome, goldpushk.PeriodicTasks)
	chromePublicFrontend := makeID(goldpushk.ChromePublic, goldpushk.Frontend)
	chromiumTastBaselineServer := makeID(goldpushk.ChromiumOSTastDev, goldpushk.BaselineServer)
	chromiumTastFrontend := makeID(goldpushk.ChromiumOSTastDev, goldpushk.Frontend)
	flutterBaselineServer := makeID(goldpushk.Flutter, goldpushk.BaselineServer)
	flutterEngineBaselineServer := makeID(goldpushk.FlutterEngine, goldpushk.BaselineServer)
	flutterEngineFrontend := makeID(goldpushk.FlutterEngine, goldpushk.Frontend)
	flutterFrontend := makeID(goldpushk.Flutter, goldpushk.Frontend)
	lottieFrontend := makeID(goldpushk.Lottie, goldpushk.Frontend)
	pdfiumBaselineServer := makeID(goldpushk.Pdfium, goldpushk.BaselineServer)
	pdfiumFrontend := makeID(goldpushk.Pdfium, goldpushk.Frontend)
	skiaDiffCalculator := makeID(goldpushk.Skia, goldpushk.DiffCalculator)
	skiaFrontend := makeID(goldpushk.Skia, goldpushk.Frontend)
	skiaGitilesFollower := makeID(goldpushk.Skia, goldpushk.GitilesFollower)
	skiaPeriodicTasks := makeID(goldpushk.Skia, goldpushk.PeriodicTasks)
	skiaInfraBaselineServer := makeID(goldpushk.SkiaInfra, goldpushk.BaselineServer)
	skiaInfraFrontend := makeID(goldpushk.SkiaInfra, goldpushk.Frontend)
	skiaIngestion := makeID(goldpushk.Skia, goldpushk.Ingestion)
	skiaIngestionBT := makeID(goldpushk.Skia, goldpushk.IngestionBT)
	skiaPublicFrontend := makeID(goldpushk.SkiaPublic, goldpushk.Frontend)

	test := func(name string, flagInstances, flagServices, flagCanaries []string, expectedDeployableUnitIDs, expectedCanariedDeployableUnitIDs []goldpushk.DeployableUnitID) {
		t.Run(name, func(t *testing.T) {
			deployableUnits, canariedDeployableUnits, err := parseAndValidateFlags(goldpushk.ProductionDeployableUnits(), flagInstances, flagServices, flagCanaries)
			deployableUnitIDs := mapUnitsToIDs(deployableUnits)
			canariedDeployableUnitIDs := mapUnitsToIDs(canariedDeployableUnits)

			require.NoError(t, err)
			assert.ElementsMatch(t, expectedDeployableUnitIDs, deployableUnitIDs)
			assert.ElementsMatch(t, expectedCanariedDeployableUnitIDs, canariedDeployableUnitIDs)
		})
	}

	// Cases with no wild cards
	test("Single instance, single service, no canary",
		[]string{"chrome"}, []string{"baselineserver"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer},
		nil)
	test("Single instance, multiple services, no canary",
		[]string{"chrome"}, []string{"baselineserver", "frontend"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeFrontend},
		nil)
	test("Single instance, multiple services, one canary",
		[]string{"chrome"}, []string{"baselineserver", "frontend"}, []string{"chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer},
		[]goldpushk.DeployableUnitID{chromeFrontend})
	test("Single instance, multiple services, multiple canaries",
		[]string{"chrome"}, []string{"baselineserver", "diffcalculator", "frontend"}, []string{"chrome:diffcalculator", "chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer},
		[]goldpushk.DeployableUnitID{chromeDiffCalculator, chromeFrontend})
	test("Multiple instances, single service, no canary",
		[]string{"chrome", "skia", "skia-public"}, []string{"frontend"}, nil,
		[]goldpushk.DeployableUnitID{chromeFrontend, skiaFrontend, skiaPublicFrontend},
		nil)
	test("Multiple instances, single service, one canary",
		[]string{"chrome", "skia", "skia-public"}, []string{"frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})
	test("Multiple instances, single service, multiple canaries",
		[]string{"chrome", "skia", "skia-public"}, []string{"frontend"}, []string{"skia:frontend", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeFrontend},
		[]goldpushk.DeployableUnitID{skiaFrontend, skiaPublicFrontend})
	test("Multiple instances, multiple services, no canary",
		[]string{"chrome", "skia", "skia-public"}, []string{"diffcalculator", "frontend"}, nil,
		[]goldpushk.DeployableUnitID{chromeDiffCalculator, chromeFrontend, skiaDiffCalculator, skiaFrontend, skiaPublicFrontend},
		nil)
	test("Multiple instances, multiple services, one canary",
		[]string{"chrome", "skia", "skia-public"}, []string{"diffcalculator", "frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeDiffCalculator, chromeFrontend, skiaDiffCalculator, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})
	test("Multiple instances, multiple services, multiple canaries",
		[]string{"chrome", "skia", "skia-public"}, []string{"diffcalculator", "frontend"}, []string{"skia:frontend", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeDiffCalculator, chromeFrontend, skiaDiffCalculator},
		[]goldpushk.DeployableUnitID{skiaFrontend, skiaPublicFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Wildcard: --service all                                                                    //
	////////////////////////////////////////////////////////////////////////////////////////////////
	test("Single instance, all services, no canary",
		[]string{"chrome"}, []string{"all"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestionBT, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks},
		nil)
	test("Single instance, all services, one canary",
		[]string{"chrome"}, []string{"all"}, []string{"chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestionBT, chromeIngestion, chromeGitilesFollower, chromePeriodicTasks},
		[]goldpushk.DeployableUnitID{chromeFrontend})
	test("Single instance, all services, multiple canaries",
		[]string{"chrome"}, []string{"all"}, []string{"chrome:ingestion-bt", "chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestion, chromeGitilesFollower, chromePeriodicTasks},
		[]goldpushk.DeployableUnitID{chromeIngestionBT, chromeFrontend})
	test("Multiple instances, all services, no canary",
		[]string{"chrome", "skia"}, []string{"all"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestionBT, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks, skiaDiffCalculator, skiaIngestionBT, skiaIngestion, skiaFrontend, skiaGitilesFollower, skiaPeriodicTasks},
		nil)
	test("Multiple instances, all services, one canary",
		[]string{"chrome", "skia"}, []string{"all"}, []string{"skia:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestionBT, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks, skiaDiffCalculator, skiaIngestionBT, skiaIngestion, skiaGitilesFollower, skiaPeriodicTasks},
		[]goldpushk.DeployableUnitID{skiaFrontend})
	test("Multiple instances, all services, multiple canaries",
		[]string{"chrome", "skia"}, []string{"all"}, []string{"skia:ingestion-bt", "skia:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestionBT, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks, skiaDiffCalculator, skiaIngestion, skiaGitilesFollower, skiaPeriodicTasks},
		[]goldpushk.DeployableUnitID{skiaIngestionBT, skiaFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Wildcard: --instance all                                                                   //
	////////////////////////////////////////////////////////////////////////////////////////////////
	test("All instances, single service, no canary",
		[]string{"all"}, []string{"frontend"}, nil,
		[]goldpushk.DeployableUnitID{angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend, skiaPublicFrontend},
		nil)
	test("All instances, single service, one canary",
		[]string{"all"}, []string{"frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})
	test("All instances, single service, multiple canaries",
		[]string{"all"}, []string{"frontend"}, []string{"skia-infra:frontend", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaInfraFrontend, skiaPublicFrontend})
	test("All instances, multiple services, no canary",
		[]string{"all"}, []string{"baselineserver", "frontend"}, nil,
		[]goldpushk.DeployableUnitID{
			angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend, skiaPublicFrontend,
			angleBaselineServer, chromeBaselineServer, chromiumTastBaselineServer, flutterBaselineServer, flutterEngineBaselineServer, pdfiumBaselineServer, skiaInfraBaselineServer},
		nil)
	test("All instances, multiple services, one canary",
		[]string{"all"}, []string{"baselineserver", "frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{
			angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend,
			angleBaselineServer, chromeBaselineServer, chromiumTastBaselineServer, flutterBaselineServer, flutterEngineBaselineServer, pdfiumBaselineServer, skiaInfraBaselineServer},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Miscellaneous                                                                              //
	////////////////////////////////////////////////////////////////////////////////////////////////

	test("Repeated inputs are ignored",
		[]string{"chrome", "chrome", "skia", "chrome", "skia", "skia-public", "skia-public"}, []string{"diffcalculator", "frontend", "diffcalculator", "frontend"}, []string{"skia:diffcalculator", "skia-public:frontend", "skia:diffcalculator", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeDiffCalculator, chromeFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaDiffCalculator, skiaPublicFrontend})
	test("Outputs sorted by instance, then service",
		[]string{"skia-public", "chrome", "skia"}, []string{"frontend", "diffcalculator"}, []string{"skia-public:frontend", "skia:diffcalculator"},
		[]goldpushk.DeployableUnitID{chromeDiffCalculator, chromeFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaDiffCalculator, skiaPublicFrontend})
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
