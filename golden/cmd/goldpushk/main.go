// goldpushk pushes Gold services to production. See go/goldpushk.
//
// Sample usage:
//
//   Deployment of a specific service:
//     $ goldpushk --service diffcalculator --instance chrome-gpu
//     $ goldpushk -s diffcalculator -i chrome-gpu
//
//   Deployment of a specific service across multiple instances:
//     $ goldpushk --service diffcalculator --instance chrome-gpu,skia
//     $ goldpushk -s diffcalculator -i chrome-gpu,skia
//
//   Deployment of all instances of a given service across all Gold instances:
//     $ goldpushk --service diffcalculator --instance all
//     $ goldpushk -s diffcalculator -i all
//
//   Deployment of all services corresponding to a specific Gold instance:
//     $ goldpushk --service all --instance chrome-gpu
//     $ goldpushk -s all -i chrome-gpu
//
//   Deployment of all instances of a given service, designating one of them as the canary:
//     $ goldpushk --service diffcalculator --instance all --canary skia:diffcalculator
//
//   Print out all Gold instances and services goldpushk is able to manage:
//     $ goldpushk --list

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/nooplogging"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/cmd/goldpushk/goldpushk"
)

const (
	// Wildcard value for command line arguments.
	all = "all"

	// Environment variable with path to buildbot repository checkout directory.
	skiaInfraRootEnvVar = "SKIA_INFRA_ROOT"

	// Git repository with k8s configuration files in YAML format.
	k8sConfigRepoUrl = "https://skia.googlesource.com/k8s-config"
)

var (
	// Required flags.
	flagInstances []string
	flagServices  []string
	flagCanaries  []string

	// Optional flags.
	flagList                       bool
	flagDryRun                     bool
	flagNoCommit                   bool
	flagMinUptimeSeconds           int
	flagUptimePollFrequencySeconds int

	// Flags for debugging.
	flagLogToStdErr bool
	flagVerbose     bool
	flagTesting     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:  "goldpushk",
		Long: "goldpushk pushes Gold services to production.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if flagLogToStdErr {
				sklogimpl.SetLogger(stdlogging.New(os.Stderr))
			} else {
				sklogimpl.SetLogger(nooplogging.New())
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			run(cmd)
		},
	}

	rootCmd.Flags().SortFlags = false
	rootCmd.Flags().BoolVar(&flagList, "list", false, "List known Gold instances and services (tip: try combining this flag with --testing).")
	rootCmd.Flags().StringSliceVarP(&flagInstances, "instances", "i", []string{}, "[REQUIRED] Comma-delimited list of Gold instances to target (e.g. \"skia,flutter\"), or \""+all+"\" to target all instances.")
	rootCmd.Flags().StringSliceVarP(&flagServices, "services", "s", []string{}, "[REQUIRED] Comma-delimited list of services to target (e.g. \"frontend,diffcalculator\"), or \""+all+"\" to target all services.")
	rootCmd.Flags().StringSliceVarP(&flagCanaries, "canaries", "c", []string{}, "Comma-delimited subset of Gold services to use as canaries, written as instance:service pairs (e.g. \"skia:diffcalculator,flutter:frontend\")")
	rootCmd.Flags().BoolVar(&flagDryRun, "dryrun", false, "Do everything except applying the new configuration to Kubernetes and committing changes to Git.")
	rootCmd.Flags().BoolVar(&flagNoCommit, "no-commit", false, "Do not commit configuration changes to the k8s-config repository.")
	rootCmd.Flags().IntVar(&flagMinUptimeSeconds, "min-uptime", 30, "Minimum uptime in seconds required for all services before exiting the monitoring step.")
	rootCmd.Flags().IntVar(&flagUptimePollFrequencySeconds, "poll-freq", 3, "How often to poll Kubernetes for service uptimes, in seconds.")
	rootCmd.Flags().BoolVar(&flagLogToStdErr, "logtostderr", false, "Log debug information to stderr. No logs will be produced if this flag is not set.")
	rootCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Verbose logs. This will log the commands executed and their command-line parameters.")

	// Fail with exit code 1 in the presence of invalid flags.
	if _, err := rootCmd.ExecuteC(); err != nil {
		sklog.Errorf("Failed to execute Cobra command: %s", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command) {
	// Get set of deployable units. Used as the source of truth across goldpushk.
	deployableUnitSet := goldpushk.ProductionDeployableUnits()

	// If --list is passed, print known services and exit. This takes into account flag --testing.
	if flagList {
		if err := listKnownServices(deployableUnitSet); err != nil {
			sklog.Fatalf("Error while printing list of known services: %s", err)
		}
		return
	}

	// If --list was not provided, validate presence of flags --services and --instances.
	if len(flagInstances) == 0 {
		fmt.Println("Error: flag \"instances\" is required.")
		if err := cmd.Usage(); err != nil {
			sklog.Fatalf("Error while printing usage: %s", err)
		}
		os.Exit(1)
	}
	if len(flagServices) == 0 {
		fmt.Println("Error: flag \"services\" is required.")
		if err := cmd.Usage(); err != nil {
			sklog.Fatalf("Error while printing usage: %s", err)
		}
		os.Exit(1)
	}

	// Parse and validate command line flags.
	deployableUnits, canariedDeployableUnits, err := parseAndValidateFlags(deployableUnitSet, flagInstances, flagServices, flagCanaries)
	if err != nil {
		fmt.Printf("Error: %s.\n", err)
		os.Exit(1)
	}

	// Read environment variables.
	skiaInfraRoot, ok := os.LookupEnv(skiaInfraRootEnvVar)
	if !ok {
		fmt.Printf("Error: environment variable %s not set.\n", skiaInfraRootEnvVar)
		os.Exit(1)
	}

	// Build goldpushk instance.
	gpk := goldpushk.New(deployableUnits, canariedDeployableUnits, skiaInfraRoot, flagDryRun, flagNoCommit, flagMinUptimeSeconds, flagUptimePollFrequencySeconds, k8sConfigRepoUrl, flagVerbose)

	// Run goldpushk.
	if err = gpk.Run(context.Background()); err != nil {
		fmt.Printf("Error: %s.\n", err)
		os.Exit(1)
	}
}

// listKnownServices prints out a table of known services.
func listKnownServices(deployableUnitSet goldpushk.DeployableUnitSet) error {
	mode := "production"
	if flagTesting {
		mode = "testing"
	}
	fmt.Printf("Known Gold instances and services (%s):\n", mode)

	// Print out table header.
	w := tabwriter.NewWriter(os.Stdout, 10, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "\nINSTANCE\tSERVICE\tCANONICAL NAME"); err != nil {
		return skerr.Wrap(err)
	}

	// Print out table body.
	for _, instance := range deployableUnitSet.KnownInstances() {
		for _, service := range deployableUnitSet.KnownServices() {
			unit, ok := deployableUnitSet.Get(goldpushk.DeployableUnitID{Instance: instance, Service: service})
			if ok {
				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", instance, service, unit.CanonicalName()); err != nil {
					return skerr.Wrap(err)
				}
			}
		}
	}

	// Flush output and return.
	if err := w.Flush(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
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
			if !deployableUnitSet.IsKnownInstance(goldpushk.Instance(instanceStr)) {
				return nil, nil, fmt.Errorf("unknown Gold instance: \"%s\"", instanceStr)
			}
		}
	}

	// Validate services.
	if !allServices {
		for _, serviceStr := range services {
			if !deployableUnitSet.IsKnownService(goldpushk.Service(serviceStr)) {
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
		instanceIterationSet = deployableUnitSet.KnownInstances()
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
		serviceIterationSet = deployableUnitSet.KnownServices()
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
		if !deployableUnitSet.IsKnownInstance(instance) {
			return nil, nil, fmt.Errorf("invalid canary - unknown Gold instance: \"%s\"", canaryStr)
		}
		if !deployableUnitSet.IsKnownService(service) {
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
