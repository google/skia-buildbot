package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/cmd/goldpushk/goldpushk"
)

func TestParseAndValidateFlags_ErrorCases(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name string, instances, services, canaries []string, errMsg string) {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseAndValidateFlags(goldpushk.ProductionDeployableUnits(), instances, services, canaries)
			require.Error(t, err)
			assert.Contains(t, err.Error(), errMsg)
		})
	}

	test("instances all,chrome", []string{"all", "chrome"}, []string{"baselineserver"}, nil,
		"flag --instances should contain either \"all\" or a list of Gold instances, but not both")

	test("services all,baselineserver", []string{"chrome"}, []string{"all", "baselineserver"}, nil,
		"flag --services should contain either \"all\" or a list of Gold services, but not both")

	test("--instances and --services both set to \"all\"", []string{"all"}, []string{"all"}, nil,
		"cannot set both --instances and --services to \"all\"")

	test("Unknown instance", []string{"foo"}, []string{"baselineserver"}, nil,
		"unknown Gold instance: \"foo\"")

	test("Unknown service", []string{"chrome"}, []string{"foo"}, nil,
		"unknown Gold service: \"foo\"")

	test("No instances/services matched.", []string{"skia-public"}, []string{"baselineserver"}, nil,
		"no known Gold services match the values supplied with --instances and --services")

	test("Invalid canary format", []string{"chrome"}, []string{"baselineserver"}, []string{"xxxxxxxxx"},
		"invalid canary format: \"xxxxxxxxx\"")

	test("Invalid canary due to unknown instance", []string{"chrome"}, []string{"baselineserver"}, []string{"foo:baselineserver"},
		"invalid canary - unknown Gold instance: \"foo:baselineserver\"")

	test("Invalid canary due to unknown service", []string{"chrome"}, []string{"baselineserver"}, []string{"chrome:foo"},
		"invalid canary - unknown Gold service: \"chrome:foo\"")

	test("Canary doesn't match --instances / --services", []string{"chrome"}, []string{"baselineserver"}, []string{"skia:diffcalculator"},
		"canary does not match any targeted services: \"skia:diffcalculator\"")

	test("All targeted services are canaried", []string{"chrome"}, []string{"baselineserver"}, []string{"chrome:baselineserver"},
		"all targeted services are marked for canarying",
	)
}

func TestParseAndValidateFlagsSuccess(t *testing.T) {
	unittest.SmallTest(t)

	// Deployments shared among test cases.
	angleBaselineServer := makeID(goldpushk.Angle, goldpushk.BaselineServer)
	angleFrontend := makeID(goldpushk.Angle, goldpushk.Frontend)
	battlestarBaselineServer := makeID(goldpushk.Battlestar, goldpushk.BaselineServer)
	battlestarFrontend := makeID(goldpushk.Battlestar, goldpushk.Frontend)
	chromeBaselineServer := makeID(goldpushk.Chrome, goldpushk.BaselineServer)
	chromeDiffCalculator := makeID(goldpushk.Chrome, goldpushk.DiffCalculator)
	chromeFrontend := makeID(goldpushk.Chrome, goldpushk.Frontend)
	chromeGitilesFollower := makeID(goldpushk.Chrome, goldpushk.GitilesFollower)
	chromeIngestion := makeID(goldpushk.Chrome, goldpushk.Ingestion)
	chromePeriodicTasks := makeID(goldpushk.Chrome, goldpushk.PeriodicTasks)
	chromePublicFrontend := makeID(goldpushk.ChromePublic, goldpushk.Frontend)
	chromiumTastBaselineServer := makeID(goldpushk.ChromiumOSTast, goldpushk.BaselineServer)
	chromiumTastFrontend := makeID(goldpushk.ChromiumOSTast, goldpushk.Frontend)
	eskiaBaselineServer := makeID(goldpushk.ESkia, goldpushk.BaselineServer)
	eskiaFrontend := makeID(goldpushk.ESkia, goldpushk.Frontend)
	flutterBaselineServer := makeID(goldpushk.Flutter, goldpushk.BaselineServer)
	flutterEngineBaselineServer := makeID(goldpushk.FlutterEngine, goldpushk.BaselineServer)
	flutterEngineFrontend := makeID(goldpushk.FlutterEngine, goldpushk.Frontend)
	flutterFrontend := makeID(goldpushk.Flutter, goldpushk.Frontend)
	lottieFrontend := makeID(goldpushk.Lottie, goldpushk.Frontend)
	lottieSpecFrontend := makeID(goldpushk.LottieSpec, goldpushk.Frontend)
	pdfiumBaselineServer := makeID(goldpushk.Pdfium, goldpushk.BaselineServer)
	pdfiumFrontend := makeID(goldpushk.Pdfium, goldpushk.Frontend)
	skiaBaselineServer := makeID(goldpushk.Skia, goldpushk.BaselineServer)
	skiaDiffCalculator := makeID(goldpushk.Skia, goldpushk.DiffCalculator)
	skiaFrontend := makeID(goldpushk.Skia, goldpushk.Frontend)
	skiaGitilesFollower := makeID(goldpushk.Skia, goldpushk.GitilesFollower)
	skiaPeriodicTasks := makeID(goldpushk.Skia, goldpushk.PeriodicTasks)
	skiaInfraBaselineServer := makeID(goldpushk.SkiaInfra, goldpushk.BaselineServer)
	skiaInfraFrontend := makeID(goldpushk.SkiaInfra, goldpushk.Frontend)
	skiaIngestion := makeID(goldpushk.Skia, goldpushk.Ingestion)
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
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks},
		nil)
	test("Single instance, all services, one canary",
		[]string{"chrome"}, []string{"all"}, []string{"chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestion, chromeGitilesFollower, chromePeriodicTasks},
		[]goldpushk.DeployableUnitID{chromeFrontend})
	test("Single instance, all services, multiple canaries",
		[]string{"chrome"}, []string{"all"}, []string{"chrome:ingestion", "chrome:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeGitilesFollower, chromePeriodicTasks},
		[]goldpushk.DeployableUnitID{chromeIngestion, chromeFrontend})
	test("Multiple instances, all services, no canary",
		[]string{"chrome", "skia"}, []string{"all"}, nil,
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks, skiaBaselineServer, skiaDiffCalculator, skiaIngestion, skiaFrontend, skiaGitilesFollower, skiaPeriodicTasks},
		nil)
	test("Multiple instances, all services, one canary",
		[]string{"chrome", "skia"}, []string{"all"}, []string{"skia:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks, skiaBaselineServer, skiaDiffCalculator, skiaIngestion, skiaGitilesFollower, skiaPeriodicTasks},
		[]goldpushk.DeployableUnitID{skiaFrontend})
	test("Multiple instances, all services, multiple canaries",
		[]string{"chrome", "skia"}, []string{"all"}, []string{"skia:ingestion", "skia:frontend"},
		[]goldpushk.DeployableUnitID{chromeBaselineServer, chromeDiffCalculator, chromeIngestion, chromeFrontend, chromeGitilesFollower, chromePeriodicTasks, skiaBaselineServer, skiaDiffCalculator, skiaGitilesFollower, skiaPeriodicTasks},
		[]goldpushk.DeployableUnitID{skiaIngestion, skiaFrontend})

	////////////////////////////////////////////////////////////////////////////////////////////////
	// Wildcard: --instance all                                                                   //
	////////////////////////////////////////////////////////////////////////////////////////////////
	test("All instances, single service, no canary",
		[]string{"all"}, []string{"frontend"}, nil,
		[]goldpushk.DeployableUnitID{angleFrontend, battlestarFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, eskiaFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, lottieSpecFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend, skiaPublicFrontend},
		nil)
	test("All instances, single service, one canary",
		[]string{"all"}, []string{"frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, battlestarFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, eskiaFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, lottieSpecFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend},
		[]goldpushk.DeployableUnitID{skiaPublicFrontend})
	test("All instances, single service, multiple canaries",
		[]string{"all"}, []string{"frontend"}, []string{"skia-infra:frontend", "skia-public:frontend"},
		[]goldpushk.DeployableUnitID{angleFrontend, battlestarFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, eskiaFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, lottieSpecFrontend, pdfiumFrontend, skiaFrontend},
		[]goldpushk.DeployableUnitID{skiaInfraFrontend, skiaPublicFrontend})
	test("All instances, multiple services, no canary",
		[]string{"all"}, []string{"baselineserver", "frontend"}, nil,
		[]goldpushk.DeployableUnitID{
			angleFrontend, battlestarFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, eskiaFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, lottieSpecFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend, skiaPublicFrontend,
			angleBaselineServer, battlestarBaselineServer, chromeBaselineServer, chromiumTastBaselineServer, eskiaBaselineServer, flutterBaselineServer, flutterEngineBaselineServer, pdfiumBaselineServer, skiaBaselineServer, skiaInfraBaselineServer},
		nil)
	test("All instances, multiple services, one canary",
		[]string{"all"}, []string{"baselineserver", "frontend"}, []string{"skia-public:frontend"},
		[]goldpushk.DeployableUnitID{
			angleFrontend, battlestarFrontend, chromeFrontend, chromePublicFrontend, chromiumTastFrontend, eskiaFrontend, flutterFrontend, flutterEngineFrontend, lottieFrontend, lottieSpecFrontend, pdfiumFrontend, skiaFrontend, skiaInfraFrontend,
			angleBaselineServer, battlestarBaselineServer, chromeBaselineServer, chromiumTastBaselineServer, eskiaBaselineServer, flutterBaselineServer, flutterEngineBaselineServer, pdfiumBaselineServer, skiaBaselineServer, skiaInfraBaselineServer},
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
