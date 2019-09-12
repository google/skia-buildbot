// goldpushk pushes Gold services to production. See go/goldpushk.
//
// Sample usage:
//
//   Deployment of a specific service:
//     $ goldpushk --service diffserver --instance chrome-gpu
//     $ goldpushk -s diffserver -i chrome-gpu
//
//   Deployment of a specific service across multiple instances:
//     $ goldpushk --service diffserver --instance chrome-gpu,skia
//     $ goldpushk -s diffserver -i chrome-gpu,skia
//
//   Deployment of all instances of a given service across all Gold instances:
//     $ goldpushk --service diffserver --instance all
//     $ goldpushk -s diffserver -i all
//
//   Deployment of all services corresponding to a specific Gold instance:
//     $ goldpushk --service all --instance chrome-gpu
//     $ goldpushk -s all -i chrome-gpu
//
//   Deployment of all instances of a given service, designating one of them as the canary:
//     $ goldpushk --service diffserver --instance all --canary skia:diffserver
//
//   Print out all Gold instances and services goldpushk is able to manage:
//     $ goldpushk --list
//     $ goldpushk -l

package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/cmd/goldpushk/goldpushk"
)

const (
	all = "all" // Wildcard value for command line arguments.
)

var (
	flagInstances []string
	flagServices  []string
	flagCanaries  []string
	flagDryRun    bool
)

func main() {
	// TODO(lovisolo): Add --list flag.
	rootCmd := &cobra.Command{
		Use:  "goldpushk",
		Long: "goldpushk pushes Gold services to production.",
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}
	rootCmd.Flags().SortFlags = false
	rootCmd.Flags().StringSliceVarP(&flagInstances, "instances", "i", []string{}, "[REQUIRED] Comma-delimited list of Gold instances to target (e.g. \"skia,flutter\"), or \""+all+"\" to target all instances.")
	rootCmd.Flags().StringSliceVarP(&flagServices, "services", "s", []string{}, "[REQUIRED] Comma-delimited list of services to target (e.g. \"skiacorrectness,diffserver\"), or \""+all+"\" to target all services.")
	rootCmd.Flags().StringSliceVarP(&flagCanaries, "canaries", "c", []string{}, "Comma-delimited subset of Gold services to use as canaries, written as instance:service pairs (e.g. \"skia:diffserver,flutter:skiacorrectness\")")
	rootCmd.Flags().BoolVarP(&flagDryRun, "dryrun", "d", false, "Do everything except applying the new configuration to Kubernetes and committing changes to Git.")
	if err := rootCmd.MarkFlagRequired("services"); err != nil {
		sklog.Fatalf("Error while setting up command line flags: %s", err)
	}
	if err := rootCmd.MarkFlagRequired("instances"); err != nil {
		sklog.Fatalf("Error while setting up command line flags: %s", err)
	}
	if _, err := rootCmd.ExecuteC(); err != nil {
		sklog.Fatalf("Error while running Cobra command: %s", err)
	}
}

func run() {
	// Get set of deployable units. Used as the source of truth across goldpushk.
	deployableUnitSet := goldpushk.BuildDeployableUnitSet()

	// Parse and validate command line flags.
	deployableUnits, canariedDeployableUnits, err := parseAndValidateFlags(deployableUnitSet, flagInstances, flagServices, flagCanaries)
	if err != nil {
		fmt.Printf("Error: %s.\n", err)
		os.Exit(1)
	}

	// Run goldpushk.
	gpk := goldpushk.New(deployableUnits, canariedDeployableUnits, flagDryRun)
	if err = gpk.Run(); err != nil {
		fmt.Printf("Error: %s.\n", err)
		os.Exit(1)
	}
}

// containsWildcardValue determines whether or not a flag contains the special
// "all" wildcard value.
func containsWildcardValue(flag []string) bool {
	for _, value := range flag {
		if value == all {
			return true
		}
	}
	return false
}

// parseAndValidateFlags validates the given command line flags, retrieves the
// corresponding DeployableUnits from the given DeployableUnitSet and returns
// them as two separate slices according to whether or not they were marked for
// canarying.
func parseAndValidateFlags(deployableUnitSet goldpushk.DeployableUnitSet, instances, services, canaries []string) (deployableUnits, canariedDeployableUnits []goldpushk.DeployableUnit, err error) {
	// Deduplicate inputs.
	instances = util.SSliceDedup(instances)
	services = util.SSliceDedup(services)
	canaries = util.SSliceDedup(canaries)

	// Determine whether --instances or --services are set to "all".
	allInstances := containsWildcardValue(instances)
	allServices := containsWildcardValue(services)

	// If --instances or --services contain the special "all" value, they should
	// not contain any other values.
	if allInstances && len(instances) != 1 {
		return nil, nil, errors.New("flag --instances should contain either \"all\" or a list of Gold instances, but not both")
	}
	if allServices && len(services) != 1 {
		return nil, nil, errors.New("flag --services should contain either \"all\" or a list of Gold services, but not both")
	}
	if allInstances && allServices {
		return nil, nil, errors.New("cannot set both --instances and --services to \"all\"")
	}

	// Validate instances.
	if !allInstances {
		for _, instanceStr := range instances {
			if !goldpushk.IsKnownInstance(goldpushk.Instance(instanceStr)) {
				return nil, nil, fmt.Errorf("unknown Gold instance: \"%s\"", instanceStr)
			}
		}
	}

	// Validate services.
	if !allServices {
		for _, serviceStr := range services {
			if !goldpushk.IsKnownService(goldpushk.Service(serviceStr)) {
				return nil, nil, fmt.Errorf("unknown Gold service: \"%s\"", serviceStr)
			}
		}
	}

	// This slice will be populated with the subset of the cartesian product of
	// flags --instances and --services that is found in the services map.
	var servicesToDeploy []goldpushk.DeployableUnitID

	// Determines whether or not an instance/service pair should be canaried.
	isMarkedForCanarying := map[goldpushk.DeployableUnitID]bool{}

	// Determine the set of instances over which to iterate to compute the
	// cartesian product of flags --instances and --services.
	var instanceIterationSet []goldpushk.Instance
	if containsWildcardValue(instances) {
		// Handle the "all" value.
		instanceIterationSet = goldpushk.KnownInstances
	} else {
		for _, instanceStr := range instances {
			instanceIterationSet = append(instanceIterationSet, goldpushk.Instance(instanceStr))
		}
	}

	// Determine the set of services over which to iterate to compute the
	// cartesian product of flags --instances and --services.
	var serviceIterationSet []goldpushk.Service
	if containsWildcardValue(services) {
		// Handle the "all" value.
		serviceIterationSet = goldpushk.KnownServices
	} else {
		for _, serviceStr := range services {
			serviceIterationSet = append(serviceIterationSet, goldpushk.Service(serviceStr))
		}
	}

	// Iterate over the cartesian product of flags --instances and --services.
	for _, instance := range instanceIterationSet {
		for _, service := range serviceIterationSet {
			id := goldpushk.DeployableUnitID{
				Instance: instance,
				Service:  service,
			}

			// Skip if the current instance/service combination is not found in the services map.
			if _, ok := deployableUnitSet.Get(id); !ok {
				continue
			}

			// Save instance/service pair, which is not marked for canarying by default.
			servicesToDeploy = append(servicesToDeploy, id)
			isMarkedForCanarying[id] = false
		}
	}

	// Fail if --instances and --services didn't match any services in the services map.
	if len(servicesToDeploy) == 0 {
		return nil, nil, errors.New("no known Gold services match the values supplied with --instances and --services")
	}

	// Iterate over the --canaries flag.
	for _, canaryStr := range canaries {
		// Validate format and extract substrings.
		canaryStrSplit := strings.Split(canaryStr, ":")
		if len(canaryStrSplit) != 2 || len(canaryStrSplit[0]) == 0 || len(canaryStrSplit[1]) == 0 {
			return nil, nil, fmt.Errorf("invalid canary format: \"%s\"", canaryStr)
		}

		instance := goldpushk.Instance(canaryStrSplit[0])
		service := goldpushk.Service(canaryStrSplit[1])
		instanceServicePair := goldpushk.DeployableUnitID{
			Instance: instance,
			Service:  service,
		}

		// Validate canary subcomponents.
		if !goldpushk.IsKnownInstance(instance) {
			return nil, nil, fmt.Errorf("invalid canary - unknown Gold instance: \"%s\"", canaryStr)
		}
		if !goldpushk.IsKnownService(service) {
			return nil, nil, fmt.Errorf("invalid canary - unknown Gold service: \"%s\"", canaryStr)
		}

		// Canaries should match the services provided with --instances and --services.
		if _, ok := isMarkedForCanarying[instanceServicePair]; !ok {
			return nil, nil, fmt.Errorf("canary does not match any targeted services: \"%s\"", canaryStr)
		}

		// Mark instance/service pair for canarying.
		isMarkedForCanarying[instanceServicePair] = true
	}

	// Sort services to deploy to generate a deterministic output.
	sort.Slice(servicesToDeploy, func(i, j int) bool {
		a := servicesToDeploy[i]
		b := servicesToDeploy[j]
		return a.Instance < b.Instance || (a.Instance == b.Instance && a.Service < b.Service)
	})

	// Build outputs.
	for _, instanceServicePair := range servicesToDeploy {
		deployment, ok := deployableUnitSet.Get(instanceServicePair)
		if !ok {
			sklog.Fatalf("DeployableUnit \"%s\" not found in deployableUnitSet", deployment.CanonicalName())
		}

		if isMarkedForCanarying[instanceServicePair] {
			canariedDeployableUnits = append(canariedDeployableUnits, deployment)
		} else {
			deployableUnits = append(deployableUnits, deployment)
		}
	}

	// If all services to be deployed are marked for canarying, it probably
	// indicates a user error.
	if len(deployableUnits) == 0 {
		return nil, nil, errors.New("all targeted services are marked for canarying")
	}

	return deployableUnits, canariedDeployableUnits, nil
}
