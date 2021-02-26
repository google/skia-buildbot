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
	angleFrontend := makeID(goldpushk.Angle, goldpushk.Frontend)
	angleDiffServer := makeID(goldpushk.Angle, goldpushk.DiffServer)
	chromeBaselineServer := makeID(goldpushk.Chrome, goldpushk.BaselineServer)
	chromeDiffCalculator := makeID(goldpushk.Chrome, goldpushk.DiffCalculator)
	chromeDiffServer := makeID(goldpushk.Chrome, goldpushk.DiffServer)
	chromeIngestionBT := makeID(goldpushk.Chrome, goldpushk.IngestionBT)
	chromeFrontend := makeID(goldpushk.Chrome, goldpushk.Frontend)
	chromeGitilesFollower := makeID(goldpushk.Chrome, goldpushk.GitilesFollower)
	chromePublicFrontend := makeID(goldpushk.ChromePublic, goldpushk.Frontend)
	chromiumTastFrontend := makeID(goldpushk.ChromiumOSTastDev, goldpushk.Frontend)
	chromiumTastDiffServer := makeID(goldpushk.ChromiumOSTastDev, goldpushk.DiffServer)
	flutterDiffServer := makeID(goldpushk.Flutter, goldpushk.DiffServer)
	flutterEngineDiffServer := makeID(goldpushk.FlutterEngine, goldpushk.DiffServer)
	flutterEngineFrontend := makeID(goldpushk.FlutterEngine, goldpushk.Frontend)
	flutterFrontend := makeID(goldpushk.Flutter, goldpushk.Frontend)
	fuchsiaDiffServer := makeID(goldpushk.Fuchsia, goldpushk.DiffServer)
	fuchsiaPublicDiffServer := makeID(goldpushk.FuchsiaPublic, goldpushk.DiffServer)
	fuchsiaPublicFrontend := makeID(goldpushk.FuchsiaPublic, goldpushk.Frontend)
	fuchsiaFrontend := makeID(goldpushk.Fuchsia, goldpushk.Frontend)
	lottieDiffServer := makeID(goldpushk.Lottie, goldpushk.DiffServer)
	lottieFrontend := makeID(goldpushk.Lottie, goldpushk.Frontend)
	pdfiumDiffServer := makeID(goldpushk.Pdfium, goldpushk.DiffServer)
	pdfiumFrontend := makeID(goldpushk.Pdfium, goldpushk.Frontend)
	skiaDiffCalculator := makeID(goldpushk.Skia, goldpushk.DiffCalculator)
	skiaDiffServer := makeID(goldpushk.Skia, goldpushk.DiffServer)
	skiaInfraDiffServer := makeID(goldpushk.SkiaInfra, goldpushk.DiffServer)
	skiaInfraFrontend := makeID(goldpushk.SkiaInfra, goldpushk.Frontend)
	skiaIngestionBT := makeID(goldpushk.Skia, goldpushk.IngestionBT)
	skiaPublicFrontend := makeID(goldpushk.SkiaPublic, goldpushk.Frontend)
	skiaFrontend := makeID(goldpushk.Skia, goldpushk.Frontend)
	skiaGitilesFollower := makeID(goldpushk.Skia, goldpushk.GitilesFollower)

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
		[]string{"chrome"}, []string{"baselineserver", "diffserver"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer},
		nil)
	test("Single instance, multiple services, one canary",
		[]string{"chrome"}, []string{"baselineserver", "diffserver", "frontend"}, []string{"chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffServer},
		[]goldpushk.DeployableUnitID{chromeFrontend})
	test("Single instance, multiple services, multiple canaries",
		[]string{"chrome"}, []string{"baselineserver", "diffserver", "frontend"}, []string{"chrome:diffserver", "chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer},
		[]goldpushk.DeployableUnitID{chromeDiffServer, chromeFrontend})
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
		[]string{"chrome", "skia", "skia-public"}, []string{"diffserver", "frontend"}, nil,
		[]goldpushk.DeployableUnitID{chromeDiffServer, chromeFrontend, skiaDiffServer, skiaFrontend, skiaPublicFrontend},
		nil)
	test("Multiple instances, multiple services, one canary",
		[]string{"chrome", "skia", "skia-public"}, []string{"diffserver", "frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeDiffServer, chromeFrontend, skiaDiffServer, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})
	test("Multiple instances, multiple services, multiple canaries",
		[]string{"chrome", "skia", "skia-public"}, []string{"diffserver", "frontend"}, []string{"skia:frontend", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeDiffServer, chromeFrontend, skiaDiffServer},
		[]goldpushk.DeployableUnitID{skiaFrontend, skiaPublicFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Wildcard: --service all                                                                    //
	////////////////////////////////////////////////////////////////////////////////////////////////
	test("Single instance, all services, no canary",
		[]string{"chrome"}, []string{"all"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeDiffServer, chromeIngestionBT, chromeFrontend, chromeGitilesFollower},
		nil)
	test("Single instance, all services, one canary",
		[]string{"chrome"}, []string{"all"}, []string{"chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeDiffServer, chromeIngestionBT, chromeGitilesFollower},
		[]goldpushk.DeployableUnitID{chromeFrontend})
	test("Single instance, all services, multiple canaries",
		[]string{"chrome"}, []string{"all"}, []string{"chrome:ingestion-bt", "chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeDiffServer, chromeGitilesFollower},
		[]goldpushk.DeployableUnitID{chromeIngestionBT, chromeFrontend})
	test("Multiple instances, all services, no canary",
		[]string{"chrome", "skia"}, []string{"all"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeDiffServer, chromeIngestionBT, chromeFrontend, chromeGitilesFollower, skiaDiffCalculator, skiaDiffServer, skiaIngestionBT, skiaFrontend, skiaGitilesFollower},
		nil)
	test("Multiple instances, all services, one canary",
		[]string{"chrome", "skia"}, []string{"all"}, []string{"skia:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeDiffServer, chromeIngestionBT, chromeFrontend, chromeGitilesFollower, skiaDiffCalculator, skiaDiffServer, skiaIngestionBT, skiaGitilesFollower},
		[]goldpushk.DeployableUnitID{skiaFrontend})
	test("Multiple instances, all services, multiple canaries",
		[]string{"chrome", "skia"}, []string{"all"}, []string{"skia:ingestion-bt", "skia:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeDiffServer, chromeIngestionBT, chromeFrontend, chromeGitilesFollower, skiaDiffCalculator, skiaDiffServer, skiaGitilesFollower},
		[]goldpushk.DeployableUnitID{skiaIngestionBT, skiaFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Wildcard: --instance all                                                                   //
	////////////////////////////////////////////////////////////////////////////////////////////////
	test("All instances, single service, no canary",
		[]string{"all"}, []string{"frontend"}, nil,
		[]goldpushk.DeployableUnitID{angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, fuchsiaFrontend, fuchsiaPublicFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend, skiaPublicFrontend},
		nil)
	test("All instances, single service, one canary",
		[]string{"all"}, []string{"frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, fuchsiaFrontend, fuchsiaPublicFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})
	test("All instances, single service, multiple canaries",
		[]string{"all"}, []string{"frontend"}, []string{"skia-infra:frontend", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, flutterFrontend, flutterEngineFrontend, fuchsiaFrontend, fuchsiaPublicFrontend, lottieFrontend, pdfiumFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaInfraFrontend, skiaPublicFrontend})
	test("All instances, multiple services, no canary",
		[]string{"all"}, []string{"diffserver", "frontend"}, nil,
		[]goldpushk.DeployableUnitID{angleFrontend, angleDiffServer, chromeDiffServer, chromeFrontend, chromePublicFrontend, chromiumTastDiffServer, chromiumTastFrontend, flutterDiffServer, flutterFrontend, flutterEngineDiffServer, flutterEngineFrontend, fuchsiaDiffServer, fuchsiaFrontend, fuchsiaPublicDiffServer, fuchsiaPublicFrontend, lottieDiffServer, lottieFrontend, pdfiumDiffServer, pdfiumFrontend, skiaDiffServer, skiaFrontend, skiaInfraDiffServer, skiaInfraFrontend, skiaPublicFrontend},
		nil)
	test("All instances, multiple services, one canary",
		[]string{"all"}, []string{"diffserver", "frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, angleDiffServer, chromeDiffServer, chromeFrontend, chromePublicFrontend, chromiumTastDiffServer, chromiumTastFrontend, flutterDiffServer, flutterFrontend, flutterEngineDiffServer, flutterEngineFrontend, fuchsiaDiffServer, fuchsiaFrontend, fuchsiaPublicDiffServer, fuchsiaPublicFrontend, lottieDiffServer, lottieFrontend, pdfiumDiffServer, pdfiumFrontend, skiaDiffServer, skiaFrontend, skiaInfraDiffServer, skiaInfraFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Miscellaneous                                                                              //
	////////////////////////////////////////////////////////////////////////////////////////////////

	test("Repeated inputs are ignored",
		[]string{"chrome", "chrome", "skia", "chrome", "skia", "skia-public", "skia-public"}, []string{"diffserver", "frontend", "diffserver", "frontend"}, []string{"skia:diffserver", "skia-public:frontend", "skia:diffserver", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{chromeDiffServer, chromeFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaDiffServer, skiaPublicFrontend})
	test("Outputs sorted by instance, then service",
		[]string{"skia-public", "chrome", "skia"}, []string{"frontend", "diffserver"}, []string{"skia-public:frontend", "skia:diffserver"},
		[]goldpushk.DeployableUnitID{chromeDiffServer, chromeFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaDiffServer, skiaPublicFrontend})
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
