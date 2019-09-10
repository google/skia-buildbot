package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/cmd/goldpushk/goldpushk"
)

func TestParseAndValidateFlags(t *testing.T) {
	unittest.SmallTest(t)

	// Deployments shared among test cases.
	chromeBaselineServer := goldpushk.NewGoldServiceDeployment(goldpushk.Chrome, goldpushk.BaselineServer)
	chromeDiffServer := goldpushk.NewGoldServiceDeployment(goldpushk.Chrome, goldpushk.DiffServer)
	chromeSkiaCorrectness := goldpushk.NewGoldServiceDeployment(goldpushk.Chrome, goldpushk.SkiaCorrectness)
	skiaDiffServer := goldpushk.NewGoldServiceDeployment(goldpushk.Skia, goldpushk.DiffServer)
	skiaSkiaCorrectness := goldpushk.NewGoldServiceDeployment(goldpushk.Skia, goldpushk.SkiaCorrectness)
	skiaPublicDiffServer := goldpushk.NewGoldServiceDeployment(goldpushk.SkiaPublic, goldpushk.DiffServer)

	// Services map the test cases will run against.
	servicesMap := goldpushk.GoldServicesMap{
		goldpushk.Chrome: {
			goldpushk.BaselineServer:  chromeBaselineServer,
			goldpushk.DiffServer:      chromeDiffServer,
			goldpushk.SkiaCorrectness: chromeSkiaCorrectness,
		},
		goldpushk.Skia: {
			goldpushk.DiffServer:      skiaDiffServer,
			goldpushk.SkiaCorrectness: skiaSkiaCorrectness,
		},
		goldpushk.SkiaPublic: {
			goldpushk.DiffServer: skiaPublicDiffServer,
		},
	}

	testCases := []struct {
		message string // Test case name.

		// Inputs.
		flagInstances []string
		flagServices  []string
		flagCanaries  []string

		// Expected outputs.
		expectedDeployments         []goldpushk.GoldServiceDeployment
		expectedCanariedDeployments []goldpushk.GoldServiceDeployment
		hasError                    bool
		errorMsg                    string
	}{

		// Errors //////////////////////////////////////////////////////////////////

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
			errorMsg:      "no known Gold service deployments match the values supplied with --instances and --services",
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

		// No wilcards /////////////////////////////////////////////////////////////

		{
			message:             "Single instance, single service, no canary",
			flagInstances:       []string{"chrome"},
			flagServices:        []string{"baselineserver"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeBaselineServer},
		},

		{
			message:             "Single instance, multiple services, no canary",
			flagInstances:       []string{"chrome"},
			flagServices:        []string{"baselineserver", "diffserver"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer},
		},

		{
			message:                     "Single instance, multiple services, one canary",
			flagInstances:               []string{"chrome"},
			flagServices:                []string{"baselineserver", "diffserver", "skiacorrectness"},
			flagCanaries:                []string{"chrome:skiacorrectness"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{chromeSkiaCorrectness},
		},

		{
			message:                     "Single instance, multiple services, multiple canaries",
			flagInstances:               []string{"chrome"},
			flagServices:                []string{"baselineserver", "diffserver", "skiacorrectness"},
			flagCanaries:                []string{"chrome:diffserver", "chrome:skiacorrectness"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:             "Multiple instances, single service, no canary",
			flagInstances:       []string{"chrome", "skia", "skia-public"},
			flagServices:        []string{"diffserver"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeDiffServer, skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                     "Multiple instances, single service, one canary",
			flagInstances:               []string{"chrome", "skia", "skia-public"},
			flagServices:                []string{"diffserver"},
			flagCanaries:                []string{"skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, skiaDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaPublicDiffServer},
		},

		{
			message:                     "Multiple instances, single service, multiple canaries",
			flagInstances:               []string{"chrome", "skia", "skia-public"},
			flagServices:                []string{"diffserver"},
			flagCanaries:                []string{"skia:diffserver", "skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:             "Multiple instances, multiple services, no canary",
			flagInstances:       []string{"chrome", "skia", "skia-public"},
			flagServices:        []string{"diffserver", "skiacorrectness"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		{
			message:                     "Multiple instances, multiple services, one canary",
			flagInstances:               []string{"chrome", "skia", "skia-public"},
			flagServices:                []string{"diffserver", "skiacorrectness"},
			flagCanaries:                []string{"skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaPublicDiffServer},
		},

		{
			message:                     "Multiple instances, multiple services, multiple canaries",
			flagInstances:               []string{"chrome", "skia", "skia-public"},
			flagServices:                []string{"diffserver", "skiacorrectness"},
			flagCanaries:                []string{"skia:diffserver", "skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaDiffServer, skiaPublicDiffServer},
		},

		// Wildcard: --service all /////////////////////////////////////////////////

		{
			message:             "Single instance, all services, no canary",
			flagInstances:       []string{"chrome"},
			flagServices:        []string{"all"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:                     "Single instance, all services, one canary",
			flagInstances:               []string{"chrome"},
			flagServices:                []string{"all"},
			flagCanaries:                []string{"chrome:skiacorrectness"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{chromeSkiaCorrectness},
		},

		{
			message:                     "Single instance, all services, multiple canaries",
			flagInstances:               []string{"chrome"},
			flagServices:                []string{"all"},
			flagCanaries:                []string{"chrome:diffserver", "chrome:skiacorrectness"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness},
		},

		{
			message:             "Multiple instances, all services, no canary",
			flagInstances:       []string{"chrome", "skia"},
			flagServices:        []string{"all"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
		},

		{
			message:                     "Multiple instances, all services, one canary",
			flagInstances:               []string{"chrome", "skia"},
			flagServices:                []string{"all"},
			flagCanaries:                []string{"skia:skiacorrectness"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaSkiaCorrectness},
		},

		{
			message:                     "Multiple instances, all services, multiple canaries",
			flagInstances:               []string{"chrome", "skia"},
			flagServices:                []string{"all"},
			flagCanaries:                []string{"skia:diffserver", "skia:skiacorrectness"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaDiffServer, skiaSkiaCorrectness},
		},

		// Wildcard: --instance all ////////////////////////////////////////////////

		{
			message:             "All instances, single service, no canary",
			flagInstances:       []string{"all"},
			flagServices:        []string{"diffserver"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeDiffServer, skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                     "All instances, single service, one canary",
			flagInstances:               []string{"all"},
			flagServices:                []string{"diffserver"},
			flagCanaries:                []string{"skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, skiaDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaPublicDiffServer},
		},

		{
			message:                     "All instances, single service, multiple canaries",
			flagInstances:               []string{"all"},
			flagServices:                []string{"diffserver"},
			flagCanaries:                []string{"skia:diffserver", "skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:             "All instances, multiple services, no canary",
			flagInstances:       []string{"all"},
			flagServices:        []string{"diffserver", "skiacorrectness"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		{
			message:                     "All instances, multiple services, one canary",
			flagInstances:               []string{"all"},
			flagServices:                []string{"diffserver", "skiacorrectness"},
			flagCanaries:                []string{"skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaPublicDiffServer},
		},

		{
			message:                     "All instances, multiple services, multiple canaries",
			flagInstances:               []string{"all"},
			flagServices:                []string{"diffserver", "skiacorrectness"},
			flagCanaries:                []string{"skia:skiacorrectness", "skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		// Wildcards: --instance all --services all ////////////////////////////////

		{
			message:             "All instances, all services, no canary",
			flagInstances:       []string{"all"},
			flagServices:        []string{"all"},
			flagCanaries:        []string{},
			expectedDeployments: []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		{
			message:                     "All instances, all services, one canary",
			flagInstances:               []string{"all"},
			flagServices:                []string{"all"},
			flagCanaries:                []string{"skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer, skiaSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaPublicDiffServer},
		},

		{
			message:                     "All instances, all services, multiple canaries",
			flagInstances:               []string{"all"},
			flagServices:                []string{"all"},
			flagCanaries:                []string{"skia:skiacorrectness", "skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeBaselineServer, chromeDiffServer, chromeSkiaCorrectness, skiaDiffServer},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaSkiaCorrectness, skiaPublicDiffServer},
		},

		// Miscellaneous ///////////////////////////////////////////////////////////

		{
			message:                     "Repeated inputs are ignored",
			flagInstances:               []string{"chrome", "chrome", "skia", "chrome", "skia", "skia-public", "skia-public"},
			flagServices:                []string{"diffserver", "skiacorrectness", "diffserver", "skiacorrectness"},
			flagCanaries:                []string{"skia:diffserver", "skia-public:diffserver", "skia:diffserver", "skia-public:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaDiffServer, skiaPublicDiffServer},
		},

		{
			message:                     "Outputs sorted by instance, then service",
			flagInstances:               []string{"skia-public", "chrome", "skia"},
			flagServices:                []string{"skiacorrectness", "diffserver"},
			flagCanaries:                []string{"skia-public:diffserver", "skia:diffserver"},
			expectedDeployments:         []goldpushk.GoldServiceDeployment{chromeDiffServer, chromeSkiaCorrectness, skiaSkiaCorrectness},
			expectedCanariedDeployments: []goldpushk.GoldServiceDeployment{skiaDiffServer, skiaPublicDiffServer},
		},
	}

	for _, tc := range testCases {
		deployments, canariedDeployments, err := parseAndValidateFlags(servicesMap, tc.flagInstances, tc.flagServices, tc.flagCanaries)

		if tc.hasError {
			assert.Nil(t, deployments, tc.message)
			assert.Nil(t, canariedDeployments, tc.message)
			assert.Error(t, err, tc.message)
			assert.Contains(t, err.Error(), tc.errorMsg, tc.message)
		} else {
			assert.NoError(t, err, tc.message)
			assert.Equal(t, tc.expectedDeployments, deployments, tc.message)
			assert.Equal(t, tc.expectedCanariedDeployments, canariedDeployments, tc.message)
		}
	}
}
