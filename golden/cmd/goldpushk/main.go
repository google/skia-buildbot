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
	rootCmd.MarkFlagRequired("services")
	rootCmd.MarkFlagRequired("instances")

	if _, err := rootCmd.ExecuteC(); err != nil {
		os.Exit(1)
	}
}

func run() {
	// Build the services map. This is used as the source of truth for instance/service pairs.
	servicesMap := goldpushk.BuildServicesMap()

	// Parse and validate command line flags.
	deployments, canariedDeployments, err := parseAndValidateFlags(servicesMap, flagInstances, flagServices, flagCanaries)
	if err != nil {
		fmt.Printf("Error: %s.\n", err)
		os.Exit(1)
	}

	// Run goldpushk.
	gpk := goldpushk.New(deployments, canariedDeployments, flagDryRun)
	if err = gpk.Run(); err != nil {
		fmt.Printf("Error: %s.\n", err)
		os.Exit(1)
	}
}

// Determines whether or not a flag contains the "all" wildcard value.
func containsWildcardValue(flag []string) bool {
	for _, value := range flag {
		if value == all {
			return true
		}
	}
	return false
}

func parseAndValidateFlags(servicesMap goldpushk.GoldServicesMap, instances, services, canaries []string) (deployments, canariedDeployments []goldpushk.GoldServiceDeployment, err error) {
	// Deduplicate inputs.
	instances = util.SSliceDedup(instances)
	services = util.SSliceDedup(services)
	canaries = util.SSliceDedup(canaries)

	// If --instances or --services contain the special "all" value, they should
	// not contain any other values.
	if containsWildcardValue(instances) && len(instances) != 1 {
		return nil, nil, errors.New("flag --instances should contain either \"all\" or a list of Gold instances, but not both")
	}
	if containsWildcardValue(services) && len(services) != 1 {
		return nil, nil, errors.New("flag --services should contain either \"all\" or a list of Gold services, but not both")
	}

	// This slice will be populated with the subset of the cartesian product of
	// flags --instances and --services that is found in the services map.
	servicesToDeploy := make([]goldpushk.GoldInstanceServicePair, 0)

	// Determines whether or not an instance/service pair should be canaried.
	isMarkedForCanarying := make(map[goldpushk.GoldInstanceServicePair]bool)

	// Determine the set of instances over which to iterate to compute the
	// cartesian product of flags --instances and --services.
	instanceIterationSet := make([]goldpushk.GoldInstance, 0)
	if containsWildcardValue(instances) {
		// Handle the "all" value.
		instanceIterationSet = goldpushk.KnownGoldInstances
	} else {
		for _, instanceStr := range instances {
			instanceIterationSet = append(instanceIterationSet, goldpushk.GoldInstance(instanceStr))
		}
	}

	// Given an instance, determine the set of services over which to iterate to
	// compute the cartesian product of flags --instances and --services.
	serviceIterationSet := func(instance goldpushk.GoldInstance) []goldpushk.GoldService {
		if containsWildcardValue(services) {
			// Handle the "all" value.
			return goldpushk.KnownGoldServices
		}
		retval := make([]goldpushk.GoldService, 0)
		for _, serviceStr := range services {
			retval = append(retval, goldpushk.GoldService(serviceStr))
		}
		return retval
	}

	// Iterate over the cartesian product of flags --instances and --services.
	for _, instance := range instanceIterationSet {
		for _, service := range serviceIterationSet(instance) {
			// Validate inputs.
			if !goldpushk.IsKnownGoldInstance(instance) {
				return nil, nil, errors.New(fmt.Sprintf("unknown Gold instance: \"%s\"", instance))
			}
			if !goldpushk.IsKnownGoldService(service) {
				return nil, nil, errors.New(fmt.Sprintf("unknown Gold service: \"%s\"", service))
			}

			// Skip if the current instance/service combination is not found in the services map.
			if _, ok := servicesMap[instance]; !ok {
				continue
			}
			if _, ok := servicesMap[instance][service]; !ok {
				continue
			}

			// Save instance/service pair, which is not marked for canarying by default.
			instanceServicePair := goldpushk.GoldInstanceServicePair{instance, service}
			servicesToDeploy = append(servicesToDeploy, instanceServicePair)
			isMarkedForCanarying[instanceServicePair] = false
		}
	}

	// Fail if --instances and --services didn't match any services in the services map.
	if len(servicesToDeploy) == 0 {
		return nil, nil, errors.New("no known Gold service deployments match the values supplied with --instances and --services")
	}

	// Iterate over the --canaries flag.
	for _, canaryStr := range canaries {
		// Validate format and extract substrings.
		canaryStrSplit := strings.Split(canaryStr, ":")
		if len(canaryStrSplit) != 2 || len(canaryStrSplit[0]) == 0 || len(canaryStrSplit[1]) == 0 {
			return nil, nil, errors.New(fmt.Sprintf("invalid canary format: \"%s\"", canaryStr))
		}

		instance := goldpushk.GoldInstance(canaryStrSplit[0])
		service := goldpushk.GoldService(canaryStrSplit[1])
		instanceServicePair := goldpushk.GoldInstanceServicePair{instance, service}

		// Validate canary subcomponents.
		if !goldpushk.IsKnownGoldInstance(instance) {
			return nil, nil, errors.New(fmt.Sprintf("invalid canary - unknown Gold instance: \"%s\"", canaryStr))
		}
		if !goldpushk.IsKnownGoldService(service) {
			return nil, nil, errors.New(fmt.Sprintf("invalid canary - unknown Gold service: \"%s\"", canaryStr))
		}

		// Canaries should match the services provided with --instances and --services.
		if _, ok := isMarkedForCanarying[instanceServicePair]; !ok {
			return nil, nil, errors.New(fmt.Sprintf("canary does not match any targeted services: \"%s\"", canaryStr))
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
		if isMarkedForCanarying[instanceServicePair] {
			canariedDeployments = append(canariedDeployments, servicesMap[instanceServicePair.Instance][instanceServicePair.Service])
		} else {
			deployments = append(deployments, servicesMap[instanceServicePair.Instance][instanceServicePair.Service])
		}
	}

	// If all services to be deployed are marked for canarying, it probably
	// indicates a user error.
	if len(deployments) == 0 {
		return nil, nil, errors.New("all targeted services are marked for canarying")
	}

	return
}
