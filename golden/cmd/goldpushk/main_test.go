package main

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/cmd/goldpushk/goldpushk"
)

func TestParseAndValidateFlags(t *testing.T) {
	unittest.SmallTest(t)

	makeDeployableUnit := func(instance goldpushk.Instance, service goldpushk.Service) goldpushk.DeployableUnit {
		return goldpushk.DeployableUnit{
			DeployableUnitID: goldpushk.DeployableUnitID{
				Instance: instance,
				Service:  service,
			},
		}
	}

	// Deployments shared among test cases.
	chromeBaselineServer := makeDeployableUnit(goldpushk.Chrome, goldpushk.BaselineServer)
	chromeDiffServer := makeDeployableUnit(goldpushk.Chrome, goldpushk.DiffServer)
	chromeSkiaCorrectness := makeDeployableUnit(goldpushk.Chrome, goldpushk.SkiaCorrectness)
	skiaDiffServer := makeDeployableUnit(goldpushk.Skia, goldpushk.DiffServer)
	skiaSkiaCorrectness := makeDeployableUnit(goldpushk.Skia, goldpushk.SkiaCorrectness)
	skiaPublicDiffServer := makeDeployableUnit(goldpushk.SkiaPublic, goldpushk.DiffServer)

	// Source of truth for all test cases.
	deployableUnitSet := goldpushk.DeployableUnitSet{}
	deployableUnitSet.Add(goldpushk.Chrome, goldpushk.BaselineServer)
	deployableUnitSet.Add(goldpushk.Chrome, goldpushk.DiffServer)
	deployableUnitSet.Add(goldpushk.Chrome, goldpushk.SkiaCorrectness)
	deployableUnitSet.Add(goldpushk.Skia, goldpushk.DiffServer)
	deployableUnitSet.Add(goldpushk.Skia, goldpushk.SkiaCorrectness)
	deployableUnitSet.Add(goldpushk.SkiaPublic, goldpushk.DiffServer)

	testCases := []struct {
		message string // Test case name.

		// Inputs.
		flagInstances []string
		flagServices  []string
		flagCanaries  []string

		// Expected outputs.
		expectedDeployableUnits         []goldpushk.DeployableUnit
		expectedCanariedDeployableUnits []goldpushk.DeployableUnit
		hasError                        bool
		errorMsg                        string
	}{

		////////////////////////////////////////////////////////////////////////////
		// Errors                                                                 //
		////////////////////////////////////////////////////////////////////////////

		{
			message:       "Error: --instances all,chrome",
			flagInstances: []string{"all", "chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{},
			hasError:      true,
			errorMsg:      "flag --instances should contain either \"all\" or a list of Gold instances, but not both",
		},

		{
			message:       "Error: --services all,baselineserver",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"all", "baselineserver"},
			flagCanaries:  []string{},
			hasError:      true,
			errorMsg:      "flag --services should contain either \"all\" or a list of Gold services, but not both",
		},

		{
			message:       "Error: --instances and --services both set to \"all\"",
			flagInstances: []string{"all"},
			flagServices:  []string{"all"},
			flagCanaries:  []string{},
			hasError:      true,
			errorMsg:      "cannot set both --instances and --services to \"all\"",
		},

		{
			message:       "Error: Unknown instance",
			flagInstances: []string{"foo"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{},
			hasError:      true,
			errorMsg:      "unknown Gold instance: \"foo\"",
		},

		{
			message:       "Error: Unknown service",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"foo"},
			flagCanaries:  []string{},
			hasError:      true,
			errorMsg:      "unknown Gold service: \"foo\"",
		},

		{
			message:       "Error: No instances/services matched.",
			flagInstances: []string{"skia"},
			flagServices:  []string{"baselineserver"},
			hasError:      true,
			errorMsg:      "no known Gold services match the values supplied with --instances and --services",
		},

		{
			message:       "Error: Invalid canary format",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"xxxxxxxxx"},
			hasError:      true,
			errorMsg:      "invalid canary format: \"xxxxxxxxx\"",
		},

		{
			message:       "Error: Invalid canary due to unknown instance",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"foo:baselineserver"},
			hasError:      true,
			errorMsg:      "invalid canary - unknown Gold instance: \"foo:baselineserver\"",
		},

		{
			message:       "Error: Invalid canary due to unknown service",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"chrome:foo"},
			hasError:      true,
			errorMsg:      "invalid canary - unknown Gold service: \"chrome:foo\"",
		},

		{
			message:       "Error: Canary doesn't match --instances / --services",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"skia:diffserver"},
			hasError:      true,
			errorMsg:      "canary does not match any targeted services: \"skia:diffserver\"",
		},

		{
			message:       "Error: All targeted services are canaried",
			flagInstances: []string{"chrome"},
			flagServices:  []string{"baselineserver"},
			flagCanaries:  []string{"chrome:baselineserver"},
			hasError:      true,
			errorMsg:      "all targeted services are marked for canarying",
		},

		////////////////////////////////////////////////////////////////////////////
		// No wildcards                                                           //
		////////////////////////////////////////////////////////////////////////////

		{
			message:                 "Single instance, single service, no canary",
			flagInstances:           []string{"chrome"},
			flagServices:            []string{"baselineserver"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeBaselineServer},
		},

		{
			message:                 "Single instance, multiple services, no canary",
			flagInstances:           []string{"chrome"},
			flagServices:            []string{"baselineserver", "diffserver"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer},
		},

		{
			message:                         "Single instance, multiple services, one canary",
			flagInstances:                   []string{"chrome"},
			flagServices:                    []string{"baselineserver", "diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"chrome:skiacorrectness"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{chromeSkiaCorrectness},
		},

		{
			message:                         "Single instance, multiple services, multiple canaries",
			flagInstances:                   []string{"chrome"},
			flagServices:                    []string{"baselineserver", "diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"chrome:diffserver", "chrome:skiacorrectness"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeBaselineServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:                 "Multiple instances, single service, no canary",
			flagInstances:           []string{"chrome", "skia", "skia-public"},
			flagServices:            []string{"diffserver"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeDiffServer, skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                         "Multiple instances, single service, one canary",
			flagInstances:                   []string{"chrome", "skia", "skia-public"},
			flagServices:                    []string{"diffserver"},
			flagCanaries:                    []string{"skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, skiaDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaPublicDiffServer},
		},

		{
			message:                         "Multiple instances, single service, multiple canaries",
			flagInstances:                   []string{"chrome", "skia", "skia-public"},
			flagServices:                    []string{"diffserver"},
			flagCanaries:                    []string{"skia:diffserver", "skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                 "Multiple instances, multiple services, no canary",
			flagInstances:           []string{"chrome", "skia", "skia-public"},
			flagServices:            []string{"diffserver", "skiacorrectness"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		{
			message:                         "Multiple instances, multiple services, one canary",
			flagInstances:                   []string{"chrome", "skia", "skia-public"},
			flagServices:                    []string{"diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaPublicDiffServer},
		},

		{
			message:                         "Multiple instances, multiple services, multiple canaries",
			flagInstances:                   []string{"chrome", "skia", "skia-public"},
			flagServices:                    []string{"diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"skia:diffserver", "skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaDiffServer, skiaPublicDiffServer},
		},

		////////////////////////////////////////////////////////////////////////////
		// Wildcard: --service all                                                //
		////////////////////////////////////////////////////////////////////////////

		{
			message:                 "Single instance, all services, no canary",
			flagInstances:           []string{"chrome"},
			flagServices:            []string{"all"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:                         "Single instance, all services, one canary",
			flagInstances:                   []string{"chrome"},
			flagServices:                    []string{"all"},
			flagCanaries:                    []string{"chrome:skiacorrectness"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{chromeSkiaCorrectness},
		},

		{
			message:                         "Single instance, all services, multiple canaries",
			flagInstances:                   []string{"chrome"},
			flagServices:                    []string{"all"},
			flagCanaries:                    []string{"chrome:diffserver", "chrome:skiacorrectness"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeBaselineServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:                 "Multiple instances, all services, no canary",
			flagInstances:           []string{"chrome", "skia"},
			flagServices:            []string{"all"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
		},

		{
			message:                         "Multiple instances, all services, one canary",
			flagInstances:                   []string{"chrome", "skia"},
			flagServices:                    []string{"all"},
			flagCanaries:                    []string{"skia:skiacorrectness"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaSkiaCorrectness},
		},

		{
			message:                         "Multiple instances, all services, multiple canaries",
			flagInstances:                   []string{"chrome", "skia"},
			flagServices:                    []string{"all"},
			flagCanaries:                    []string{"skia:diffserver", "skia:skiacorrectness"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaDiffServer, skiaSkiaCorrectness},
		},

		////////////////////////////////////////////////////////////////////////////
		// Wildcard: --instance all                                               //
		////////////////////////////////////////////////////////////////////////////

		{
			message:                 "All instances, single service, no canary",
			flagInstances:           []string{"all"},
			flagServices:            []string{"diffserver"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeDiffServer, skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                         "All instances, single service, one canary",
			flagInstances:                   []string{"all"},
			flagServices:                    []string{"diffserver"},
			flagCanaries:                    []string{"skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, skiaDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaPublicDiffServer},
		},

		{
			message:                         "All instances, single service, multiple canaries",
			flagInstances:                   []string{"all"},
			flagServices:                    []string{"diffserver"},
			flagCanaries:                    []string{"skia:diffserver", "skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                 "All instances, multiple services, no canary",
			flagInstances:           []string{"all"},
			flagServices:            []string{"diffserver", "skiacorrectness"},
			flagCanaries:            []string{},
			expectedDeployableUnits: []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		{
			message:                         "All instances, multiple services, one canary",
			flagInstances:                   []string{"all"},
			flagServices:                    []string{"diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaPublicDiffServer},
		},

		{
			message:                         "All instances, multiple services, multiple canaries",
			flagInstances:                   []string{"all"},
			flagServices:                    []string{"diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"skia:skiacorrectness", "skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		////////////////////////////////////////////////////////////////////////////
		// Miscellaneous                                                          //
		////////////////////////////////////////////////////////////////////////////

		{
			message:                         "Repeated inputs are ignored",
			flagInstances:                   []string{"chrome", "chrome", "skia", "chrome", "skia", "skia-public", "skia-public"},
			flagServices:                    []string{"diffserver", "skiacorrectness", "diffserver", "skiacorrectness"},
			flagCanaries:                    []string{"skia:diffserver", "skia-public:diffserver", "skia:diffserver", "skia-public:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                         "Outputs sorted by instance, then service",
			flagInstances:                   []string{"skia-public", "chrome", "skia"},
			flagServices:                    []string{"skiacorrectness", "diffserver"},
			flagCanaries:                    []string{"skia-public:diffserver", "skia:diffserver"},
			expectedDeployableUnits:         []goldpushk.DeployableUnit{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployableUnits: []goldpushk.DeployableUnit{skiaDiffServer, skiaPublicDiffServer},
		},
	}

	for _, tc := range testCases {
		deployableUnits, canariedDeployableUnits, err := parseAndValidateFlags(deployableUnitSet, tc.flagInstances, tc.flagServices, tc.flagCanaries)

		if tc.hasError {
			assert.Error(t, err, tc.message)
			assert.Contains(t, err.Error(), tc.errorMsg, tc.message)
		} else {
			assert.NoError(t, err, tc.message)
			assert.Equal(t, tc.expectedDeployableUnits, deployableUnits, tc.message)
			assert.Equal(t, tc.expectedCanariedDeployableUnits, canariedDeployableUnits, tc.message)
		}
	}
}
