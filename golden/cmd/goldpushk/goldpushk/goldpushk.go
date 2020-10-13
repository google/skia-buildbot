// Package goldpushk contains the Goldpushk struct, which coordinates all the operations performed
// by goldpushk.
//
// Also included in this package is function ProductionDeployableUnits(), which returns a set with
// all the services goldpushk is able to manage.
//
// Function ProductionDeployableUnits is the source of truth of goldpushk, and should be updated to
// reflect any relevant changes in configuration.
//
// For testing, function TestingDeployableUnits should be used instead, which only contains
// information about testing services that can be deployed the public or corp clusters without
// disrupting any production services.
package goldpushk

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// Paths below are relative to $SKIA_INFRA_ROOT/golden.
	k8sConfigTemplatesDir = "k8s-config-templates"
	k8sInstancesDir       = "k8s-instances"

	// kubectl timestamp format as of September 30, 2019.
	//
	// $ kubectl version --short
	// Client Version: v1.16.0
	// Server Version: v1.14.6-gke.1
	kubectlTimestampLayout = "2006-01-02T15:04:05Z"

	// Time to wait between the push and monitoring steps, to give Kubernetes a chance to update the
	// status of the affected pods.
	delayBetweenPushAndMonitoring = 10 * time.Second

	// Kubernetes does not like colons in its strings, so we can't use time.RFC3999 (or any of the
	// provided formats) as is. This replaces the colons with underscores.
	rfc3999KubernetesSafe = "2006-01-02T15_04_05Z07_00"
)

// cluster represents a Kubernetes cluster on which to deploy DeployableUnits, and contains all the
// information necessary to switch between clusters with the "gcloud" command.
type cluster struct {
	name      string // e.g. "skia-corp".
	projectID string // e.g. "google.com:skia-corp".
}

var (
	clusterSkiaPublic = cluster{name: "skia-public", projectID: "skia-public"}
	clusterSkiaCorp   = cluster{name: "skia-corp", projectID: "google.com:skia-corp"}
)

// Goldpushk contains information about the deployment steps to be carried out.
type Goldpushk struct {
	// Input parameters (provided via flags or environment variables).
	deployableUnits            []DeployableUnit
	canariedDeployableUnits    []DeployableUnit
	goldSrcDir                 string // Path to the golden directory in the skia-infra checkout.
	dryRun                     bool
	noCommit                   bool
	minUptimeSeconds           int
	uptimePollFrequencySeconds int

	// Other constructor parameters.
	k8sConfigRepoUrl string
	verbose          bool

	// Checked out Git repository with k8s config files.
	k8sConfigCheckout *git.TempCheckout

	// The Kubernetes cluster that the kubectl command is currently configured to use.
	currentCluster cluster

	// Miscellaneous.
	unitTest bool // Disables confirmation prompt from unit tests.

	disableCopyingConfigsToCheckout bool

	// If set, will return this time from .now() instead of the actual time. Used for tests.
	fakeNow time.Time
}

// New is the Goldpushk constructor.
func New(deployableUnits []DeployableUnit, canariedDeployableUnits []DeployableUnit, skiaInfraRootPath string, dryRun, noCommit bool, minUptimeSeconds, uptimePollFrequencySeconds int, k8sConfigRepoUrl string, verbose bool) *Goldpushk {
	return &Goldpushk{
		deployableUnits:            deployableUnits,
		canariedDeployableUnits:    canariedDeployableUnits,
		goldSrcDir:                 filepath.Join(skiaInfraRootPath, "golden"),
		dryRun:                     dryRun,
		noCommit:                   noCommit,
		minUptimeSeconds:           minUptimeSeconds,
		uptimePollFrequencySeconds: uptimePollFrequencySeconds,
		k8sConfigRepoUrl:           k8sConfigRepoUrl,
		verbose:                    verbose,
	}
}

// Run carries out the deployment steps.
func (g *Goldpushk) Run(ctx context.Context) error {
	// Print out list of targeted deployable units, and ask for confirmation.
	if ok, err := g.printOutInputsAndAskConfirmation(); err != nil {
		return skerr.Wrap(err)
	} else if !ok {
		return nil
	}

	// Check out k8s-config.
	if err := g.checkOutK8sConfigRepo(ctx); err != nil {
		return skerr.Wrap(err)
	}
	defer g.k8sConfigCheckout.Delete()

	// Regenerate config files.
	if err := g.regenerateConfigFiles(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Commit config files, giving the user the option to abort.
	if ok, err := g.commitConfigFiles(ctx); err != nil {
		return skerr.Wrap(err)
	} else if !ok {
		return nil
	}

	// Deploy canaries.
	if err := g.pushCanaries(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Monitor canaries.
	if err := g.monitorCanaries(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Deploy remaining DeployableUnits.
	if err := g.pushServices(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Monitor remaining DeployableUnits.
	if err := g.monitorServices(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Give the user a chance to examine the generated files before exiting and cleaning up the Git
	// repository.
	if g.dryRun {
		fmt.Println("\nDry-run finished. Any generated files can be found in the k8s-config Git repository checkout below:")
		fmt.Printf("  %s\n", g.k8sConfigCheckout.GitDir)
		fmt.Println("Press enter to delete the checkout above and exit.")
		if _, err := fmt.Scanln(); err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

// printOutInputsAndAskConfirmation prints out a summary of the actions to be
// taken, then asks the user for confirmation.
func (g *Goldpushk) printOutInputsAndAskConfirmation() (bool, error) {
	// Skip if running from an unit test.
	if g.unitTest {
		return true, nil
	}

	// Print out a summary of the services to deploy.
	if len(g.canariedDeployableUnits) != 0 {
		fmt.Println("The following services will be canaried:")
		for _, d := range g.canariedDeployableUnits {
			fmt.Printf("  %s\n", d.CanonicalName())
		}
		fmt.Println()
	}
	fmt.Println("The following services will be deployed:")
	for _, d := range g.deployableUnits {
		fmt.Printf("  %s\n", d.CanonicalName())
	}

	// Ask for confirmation, ending execution by default.
	ok, err := prompt("\nProceed?")
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if !ok {
		fmt.Println("Aborting.")
		return false, nil
	}

	return true, nil
}

// prompt prints out a question to stdout and scans a y/n answer from stdin.
func prompt(question string) (bool, error) {
	fmt.Printf("%s (y/N): ", question)
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return false, skerr.Wrapf(err, "unable to read from standard input")
	}
	if input != "y" {
		return false, nil
	}
	return true, nil
}

// checkOutK8sConfigRepo checks out the k8s-config Git repository.
func (g *Goldpushk) checkOutK8sConfigRepo(ctx context.Context) error {
	fmt.Println()
	var err error
	g.k8sConfigCheckout, err = git.NewTempCheckout(ctx, g.k8sConfigRepoUrl)
	if err != nil {
		return skerr.Wrapf(err, "failed to check out %s", g.k8sConfigRepoUrl)
	}
	fmt.Printf("Cloned Git repository %s at %s.\n", g.k8sConfigRepoUrl, string(g.k8sConfigCheckout.GitDir))
	return nil
}

// regenerateConfigFiles regenerates the .yaml and .json5 files for each
// instance/service pair that will be deployed. Any generated files will be
// checked into the corresponding Git repository with configuration files.
func (g *Goldpushk) regenerateConfigFiles(ctx context.Context) error {
	// We keep track of which instance-specific configuration files have been copied in case
	// they are required by two or more DeployableUnits; it suffices to copy said files once.
	instanceConfigsCopied := map[Instance]bool{}

	// Iterate over all units to deploy (including canaries).
	return g.forAllDeployableUnits(func(unit DeployableUnit) error {
		// Path to the template file inside $SKIA_INFRA_ROOT/golden.
		tPath := unit.getDeploymentFileTemplatePath(g.goldSrcDir)

		// Path to the deployment file (.yaml) we will regenerate inside the k8s-config Git repository.
		oPath := g.getDeploymentFilePath(unit)

		// Regenerate .yaml file.
		if err := g.expandTemplate(ctx, unit, tPath, oPath); err != nil {
			return skerr.Wrapf(err, "error while regenerating %s", oPath)
		}

		if !instanceConfigsCopied[unit.Instance] {
			instanceConfigsCopied[unit.Instance] = true
			// Copy all configuration files from the appropriate instance directory into the
			// k8s-config repo so they can be checked in.
			instanceConfigDirectory := g.getInstanceSpecificConfigDir(unit.Instance)
			checkoutDirectory := g.getGitRepoSubdirPath(unit)
			err := g.copyConfigsToCheckout(instanceConfigDirectory, checkoutDirectory)
			if err != nil {
				return skerr.Wrap(err)
			}
		}
		return nil
	})
}

// copyConfigsToCheckout copies all JSON5 configurations from the provided directory
// into the given checkout directory. Upon copying, the files will have the prefix "gold-".
func (g *Goldpushk) copyConfigsToCheckout(configDir, checkoutDir string) error {
	if g.disableCopyingConfigsToCheckout {
		return nil
	}
	jsonFiles, err := ioutil.ReadDir(configDir)
	if err != nil {
		return skerr.Wrap(err)
	}

	for _, jf := range jsonFiles {
		if !strings.HasSuffix(jf.Name(), ".json5") {
			continue
		}
		// Bad things will happen if there are multiple configuration files with the same name,
		// as one will overwrite the other. If it becomes a problem, we could try to detect it.
		dstFile := filepath.Join(checkoutDir, "gold-"+jf.Name())
		srcFile := filepath.Join(configDir, jf.Name())
		b, err := ioutil.ReadFile(srcFile)
		if err != nil {
			return skerr.Wrapf(err, "reading %s", srcFile)
		}
		if err := ioutil.WriteFile(dstFile, b, 0644); err != nil {
			return skerr.Wrapf(err, "writing %s", dstFile)
		}
	}
	return nil
}

// getInstanceSpecificConfigDir returns the path to the JSON5 configuration files for a given
// instance. These are checked in to the infra repo.
func (g *Goldpushk) getInstanceSpecificConfigDir(inst Instance) string {
	return filepath.Join(g.goldSrcDir, k8sInstancesDir, string(inst))
}

// getDeploymentFilePath returns the path to the deployment file (.yaml) for the
// given DeployableUnit inside the k8s-config Git repository.
func (g *Goldpushk) getDeploymentFilePath(unit DeployableUnit) string {
	return filepath.Join(g.getGitRepoSubdirPath(unit), unit.CanonicalName()+".yaml")
}

// getGitRepoSubdirPath returns the path to the subdirectory inside the k8s-config
// repository checkout in which the config files for the given DeployableUnit
// should be checked in  (e.g. /path/to/k8s-config/skia-public-config).
func (g *Goldpushk) getGitRepoSubdirPath(unit DeployableUnit) string {
	subdir := clusterSkiaPublic.name
	if unit.internal {
		subdir = clusterSkiaCorp.name
	}
	return filepath.Join(string(g.k8sConfigCheckout.GitDir), subdir)
}

// expandTemplate executes the kube-conf-gen command with arguments sufficient to produce the
// templated yaml files that control a Kuberenetes deployment. It makes use of the instance
// specific configuration files.
func (g *Goldpushk) expandTemplate(ctx context.Context, unit DeployableUnit, templatePath, outputPath string) error {
	goldCommonJSON5 := filepath.Join(g.goldSrcDir, k8sConfigTemplatesDir, "gold-common.json5")

	instanceStr := string(unit.Instance)
	instanceJSON5 := fmt.Sprintf("%s.json5", unit.Instance)
	instanceJSON5 = filepath.Join(g.getInstanceSpecificConfigDir(unit.Instance), instanceJSON5)

	serviceJSON5 := fmt.Sprintf("%s-%s.json5", unit.Instance, unit.Service)
	serviceJSON5 = filepath.Join(g.getInstanceSpecificConfigDir(unit.Instance), serviceJSON5)

	err := g.execCmd(ctx, "kube-conf-gen", []string{
		// Notes on the kube-conf-gen arguments used:
		//   - Flag "-extra INSTANCE_ID:<instanceStr>" binds template variable
		//     INSTANCE_ID to instanceStr.
		//   - Flag "-strict" will make kube-conf-gen fail in the presence of
		//     unsupported types, missing data, etc.
		//   - Flag "-parse_conf=false" prevents the values read from the JSON5
		//     config files provided with -c <json5-file> from being converted to
		//     strings.
		"-c", goldCommonJSON5,
		"-c", instanceJSON5,
		"-c", serviceJSON5,
		"-extra", "INSTANCE_ID:" + instanceStr,
		"-extra", "NOW:" + g.now().Format(rfc3999KubernetesSafe),
		"-t", templatePath,
		"-parse_conf=false", "-strict",
		"-o", outputPath,
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Generated %s", outputPath)
	return nil
}

// commitConfigFiles prints out a summary of the changes to be committed to
// k8s-config, asks for confirmation and pushes those changes.
func (g *Goldpushk) commitConfigFiles(ctx context.Context) (bool, error) {
	// Print out summary of changes (git status -s).
	if err := g.printOutGitStatus(ctx); err != nil {
		return false, skerr.Wrap(err)
	}

	// Skip if --no-commit or --dryrun.
	if g.dryRun || g.noCommit {
		reason := "dry run"
		if g.noCommit {
			reason = "no commit"
		}
		fmt.Printf("\nSkipping commit step (%s).\n", reason)
		return true, nil
	}

	// Ask for confirmation.
	ok, err := prompt("\nCommit and push the above changes? Answering no will abort execution.")
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if !ok {
		return false, nil
	}
	fmt.Println()

	// Skip if the k8s-config checkout has no changes (i.e. if "git status -s" prints out nothing).
	stdout, err := g.k8sConfigCheckout.Git(ctx, "status", "-s")
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if len(stdout) == 0 {
		return true, nil
	}

	// Add, commit and push changes.
	fmt.Printf("Pushing changes to the k8s-config Git repository.\n")
	if _, err := g.k8sConfigCheckout.Git(ctx, "add", "."); err != nil {
		return false, skerr.Wrap(err)
	}
	if _, err := g.k8sConfigCheckout.Git(ctx, "commit", "-m", "Push"); err != nil {
		return false, skerr.Wrap(err)
	}
	if _, err := g.k8sConfigCheckout.Git(ctx, "push", git.DefaultRemote, git.DefaultBranch); err != nil {
		return false, skerr.Wrap(err)
	}

	return true, nil
}

// printOutGitStatus runs "git status -s" on the k8s-config checkout and prints its output to stdout.
func (g *Goldpushk) printOutGitStatus(ctx context.Context) error {
	stdout, err := g.k8sConfigCheckout.Git(ctx, "status", "-s")
	if err != nil {
		return skerr.Wrap(err)
	}
	if len(stdout) == 0 {
		fmt.Printf("\nNo changes to be pushed to the k8s-config Git repository.\n")
	} else {
		fmt.Printf("\nChanges to be pushed to the k8s-config Git repository:\n")
		fmt.Print(stdout)
	}
	return nil
}

// pushCanaries deploys the canaried DeployableUnits.
func (g *Goldpushk) pushCanaries(ctx context.Context) error {
	if len(g.canariedDeployableUnits) == 0 {
		return nil
	}

	fmt.Println("\nPushing canaried services.")
	if err := g.pushDeployableUnits(ctx, g.canariedDeployableUnits); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// monitorCanaries monitors the canaried DeployableUnits after they have been pushed to production.
func (g *Goldpushk) monitorCanaries(ctx context.Context) error {
	if len(g.canariedDeployableUnits) == 0 {
		return nil
	}
	if err := g.monitor(ctx, g.canariedDeployableUnits, g.getUptimes, time.Sleep); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// pushServices deploys the non-canaried DeployableUnits.
func (g *Goldpushk) pushServices(ctx context.Context) error {
	if len(g.canariedDeployableUnits) == 0 {
		fmt.Println("\nPushing services.")
	} else {
		fmt.Println("\nPushing remaining services.")
	}

	if err := g.pushDeployableUnits(ctx, g.deployableUnits); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// monitorServices monitors the non-canaried DeployableUnits after they have been pushed to
// production.
func (g *Goldpushk) monitorServices(ctx context.Context) error {
	if err := g.monitor(ctx, g.deployableUnits, g.getUptimes, time.Sleep); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// pushDeployableUnits takes a slice of DeployableUnits and pushes them to their corresponding
// clusters.
func (g *Goldpushk) pushDeployableUnits(ctx context.Context, units []DeployableUnit) error {
	if g.dryRun {
		fmt.Println("\nSkipping push step (dry run).")
		return nil
	}

	// We want to make sure we push configs for an instance only once on a given deploy command.
	instanceSpecificConfigMapsPushed := map[Instance]bool{}
	for _, unit := range units {
		if err := g.pushSingleDeployableUnit(ctx, unit, instanceSpecificConfigMapsPushed); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// pushSingleDeployableUnit pushes the given DeployableUnit to the corresponding cluster by running
// "kubectl apply -f path/to/config.yaml".
func (g *Goldpushk) pushSingleDeployableUnit(ctx context.Context, unit DeployableUnit, instanceSpecificConfigMapsPushed map[Instance]bool) error {
	// Get the cluster corresponding to the given DeployableUnit.
	cluster := clusterSkiaPublic
	if unit.internal {
		cluster = clusterSkiaCorp
	}

	// Switch clusters.
	if err := g.switchClusters(ctx, cluster); err != nil {
		return skerr.Wrap(err)
	}

	if !instanceSpecificConfigMapsPushed[unit.Instance] {
		instanceSpecificConfigMapsPushed[unit.Instance] = true
		if err := g.pushConfigurationJSON(ctx, unit.Instance); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Push DeployableUnit.
	path := g.getDeploymentFilePath(unit)
	fmt.Printf("%s: applying %s.\n", unit.CanonicalName(), path)
	if err := g.execCmd(ctx, "kubectl", []string{"apply", "-f", path}); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// pushConfigurationJSON pushes all the configuration files for a given instance. This includes
// all JSON5 files for all services. This is done because all services overlap with some common
// configuration.
func (g *Goldpushk) pushConfigurationJSON(ctx context.Context, instance Instance) error {
	configMapName := fmt.Sprintf("gold-%s-config", instance)
	instanceConfigDirectory := g.getInstanceSpecificConfigDir(instance)
	if err := g.pushConfigMap(ctx, instanceConfigDirectory, configMapName); err != nil {
		return skerr.Wrapf(err, "pushing the configuration files at %s", instanceConfigDirectory)
	}
	return nil
}

// pushConfigMap pushes the file(s) at a given path as a config map with the given name. It deletes
// any pre-existing map before, so as to overwrite it. The path for config maps may be to a single
// file or an entire directory.
func (g *Goldpushk) pushConfigMap(ctx context.Context, path, configMapName string) error {
	// Delete existing ConfigMap.
	if err := g.execCmd(ctx, "kubectl", []string{"delete", "configmap", configMapName}); err != nil {
		// Command "kubectl delete configmap" returns exit code 1 when the ConfigMap does not exist on
		// the cluster.
		var exitError *osexec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			sklog.Infof("Did not delete ConfigMap %s as it does not exist on the cluster.", configMapName)
		} else {
			return skerr.Wrap(err)
		}
	}

	// Create new ConfigMap.
	if err := g.execCmd(ctx, "kubectl", []string{"create", "configmap", configMapName, "--from-file", path}); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// switchClusters runs the "gcloud" command necessary to switch kubectl to the given cluster.
func (g *Goldpushk) switchClusters(ctx context.Context, cluster cluster) error {
	if g.currentCluster != cluster {
		sklog.Infof("Switching to cluster %s\n", cluster.name)
		if err := g.execCmd(ctx, "gcloud", []string{"container", "clusters", "get-credentials", cluster.name, "--zone", "us-central1-a", "--project", cluster.projectID}); err != nil {
			return skerr.Wrap(err)
		}
		g.currentCluster = cluster
	}
	return nil
}

// uptimesFn has the same signature as method Goldpushk.getUptimes(). To facilitate testing, method
// Goldpushk.monitor() takes an uptimesFn instance as a parameter instead of calling
// Goldpushk.getUptimes() directly.
type uptimesFn func(context.Context, []DeployableUnit, time.Time) (map[DeployableUnitID]time.Duration, error)

// sleepFn has the same signature as time.Sleep(). Its purpose is to enable mocking that function
// from tests.
type sleepFn func(time.Duration)

// monitor watches the state of the given DeployableUnits and returns as soon as they seem to be
// up and running on their respective clusters.
//
// It does so by polling the clusters via kubectl every N seconds, and it prints out a status table
// on each iteration.
func (g *Goldpushk) monitor(ctx context.Context, units []DeployableUnit, getUptimes uptimesFn, sleep sleepFn) error {
	fmt.Printf("\nMonitoring the following services until they all reach %d seconds of uptime (polling every %d seconds):\n", g.minUptimeSeconds, g.uptimePollFrequencySeconds)
	for _, unit := range units {
		fmt.Printf("  %s\n", unit.CanonicalName())
	}

	if g.dryRun {
		fmt.Println("\nSkipping monitoring step (dry run).")
		return nil
	}

	fmt.Printf("\nWaiting %d seconds before starting the monitoring step.\n", int(delayBetweenPushAndMonitoring.Seconds()))
	sleep(delayBetweenPushAndMonitoring)

	// Estimate the width of the status table to print at each monitoring iteration. This table
	// consists of three columns: UPTIME, READY and NAME. The first two are both 10 characters wide.
	statusTableWidth := 20 // UPTIME + READY.
	// We determine the width of column NAME by looking for the longest name among all DeployableUnits
	// to monitor (e.g. "gold-skia-diffserver").
	longestName := 0
	for _, unit := range units {
		if len(unit.CanonicalName()) > longestName {
			longestName = len(unit.CanonicalName())
		}
	}
	statusTableWidth += longestName

	// Print status table header.
	w := tabwriter.NewWriter(os.Stdout, 10, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "\nUPTIME\tREADY\tNAME"); err != nil {
		return skerr.Wrap(err)
	}
	if err := w.Flush(); err != nil {
		return skerr.Wrap(err)
	}

	// Monitoring loop.
	for {
		// Get uptimes.
		uptimes, err := getUptimes(ctx, units, time.Now())
		if err != nil {
			return skerr.Wrap(err)
		}

		// Print out uptimes for each DeployableUnit.
		for _, unit := range units {
			// First we assume the DeployableUnit has no uptime. If it's not yet ready, it won't show up
			// in the uptimes dictionary.
			uptimeStr := "<None>"
			ready := "No"

			// We now check if it does have an uptime, and update the variables above accordingly if so.
			if t, ok := uptimes[unit.DeployableUnitID]; ok {
				uptimeStr = fmt.Sprintf("%ds", int64(t.Seconds()))
				if int(t.Seconds()) >= g.minUptimeSeconds {
					ready = "Yes"
				}
			}

			// Print out a row in the status table for the current DeployableUnit.
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", uptimeStr, ready, unit.CanonicalName()); err != nil {
				return skerr.Wrap(err)
			}
		}

		// Have all DeployableUnits been in the "ready" state for at least minUptimeSeconds?
		done := true
		for _, unit := range units {
			if t, ok := uptimes[unit.DeployableUnitID]; !ok || int(t.Seconds()) < g.minUptimeSeconds {
				done = false
				break
			}
		}

		// If so, break out of the monitoring loop.
		if done {
			if err := w.Flush(); err != nil {
				return skerr.Wrap(err)
			}
			return nil
		}

		// Otherwise, print a horizontal line to separate the uptimes from this iteration and the next.
		for i := 0; i < statusTableWidth; i++ {
			if _, err := fmt.Fprintf(w, "-"); err != nil {
				return skerr.Wrap(err)
			}
		}
		if _, err := fmt.Fprintf(w, "\n"); err != nil {
			return skerr.Wrap(err)
		}
		if err := w.Flush(); err != nil {
			return skerr.Wrap(err)
		}

		// Wait before polling again.
		sleep(time.Duration(g.uptimePollFrequencySeconds) * time.Second)
	}
}

// getUptimes groups the given DeployableUnits by cluster, calls getUptimesSingleCluster once per
// cluster, and returns the union of the uptimes returned by both calls to getUptimesSingleCluster.
func (g *Goldpushk) getUptimes(ctx context.Context, units []DeployableUnit, now time.Time) (map[DeployableUnitID]time.Duration, error) {
	// Group units by cluster.
	publicUnits := []DeployableUnit{}
	corpUnits := []DeployableUnit{}
	for _, unit := range units {
		if unit.internal {
			corpUnits = append(corpUnits, unit)
		} else {
			publicUnits = append(publicUnits, unit)
		}
	}

	// This will hold the uptimes from both clusters.
	allUptimes := make(map[DeployableUnitID]time.Duration)

	// Once per cluster.
	for i := 0; i < 2; i++ {
		// Select the right cluster and DeployableUnits.
		cluster := clusterSkiaPublic
		units := publicUnits
		if i == 1 {
			cluster = clusterSkiaCorp
			units = corpUnits
		}

		// Skip if no units.
		if len(units) == 0 {
			continue
		}

		// Switch to the current cluster.
		if err := g.switchClusters(ctx, cluster); err != nil {
			return nil, skerr.Wrap(err)
		}

		// Get the uptimes for the current cluster.
		uptimes, err := g.getUptimesSingleCluster(ctx, units, now)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		// Add uptimes to the multicluster map.
		for unitID, uptime := range uptimes {
			allUptimes[unitID] = uptime
		}
	}

	return allUptimes, nil
}

// getUptimesSingleCluster takes a slice of DeployableUnits and returns a dictionary mapping
// DeployableUnitIDs to the time duration since all the pods corresponding to that unit have entered
// the "Ready" state.
//
// A DeployableUnit will not have a corresponding entry in the returned map if any of its pods are
// not yet ready, or if no matching pods are returned by kubectl.
//
// This method makes the following assumptions:
//   - All the given DeployableUnits belong to the same Kubernetes cluster.
//   - kubectl is already set up to operate on that cluster.
//   - A DeployableUnit may correspond to more than one pod (e.g. ReplicaSets).
func (g *Goldpushk) getUptimesSingleCluster(ctx context.Context, units []DeployableUnit, now time.Time) (map[DeployableUnitID]time.Duration, error) {
	// JSONPath expression to be passed to kubectl. Below is a sample fragment of what the output
	// looks like:
	//
	//   app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-9krrl  ready:True  readyLastTransitionTime:2019-10-03T16:45:48Z
	//   app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-hr86n  ready:True  readyLastTransitionTime:2019-09-30T13:20:39Z
	//   app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-l4lt5  ready:True  readyLastTransitionTime:2019-10-04T01:59:40Z
	//   app:gold-chrome-gpu-diffserver  podName:gold-chrome-gpu-diffserver-0  ready:True  readyLastTransitionTime:2019-10-09T18:43:08Z
	//   app:gold-chrome-gpu-ingestion-bt  podName:gold-chrome-gpu-ingestion-bt-f8b66844f-4969w  ready:True  readyLastTransitionTime:2019-10-04T01:54:54Z
	//   app:gold-chrome-gpu-skiacorrectness  podName:gold-chrome-gpu-skiacorrectness-67c547667d-cwt42  ready:True  readyLastTransitionTime:2019-10-04T02:01:11Z
	//
	// The output format should be fairly self explanatory, but to see an example of where those
	// are coming from, try running e.g. "kubectl get pod gold-skia-diffserver-0 -o json".
	//
	// Note: Field podName is not used, and is only included for debugging purposes. It will be
	// printed out to stdout if flag --logtostderr is passed.
	jsonPathExpr := `
{range .items[*]}
{'app:'}
{.metadata.labels.app}
{'  podName:'}
{.metadata.name}
{'  ready:'}
{.status.conditions[?(@.type == 'Ready')].status}
{'  readyLastTransitionTime:'}
{.status.conditions[?(@.type == 'Ready')].lastTransitionTime}
{'\n'}
{end}`
	jsonPathExpr = strings.ReplaceAll(jsonPathExpr, "\n", "")

	// Execute kubectl command that will return per-pod uptime.
	stdout, err := g.execCmdAndReturnStdout(ctx, "kubectl", []string{"get", "pods", "-o", fmt.Sprintf("jsonpath=%s", jsonPathExpr)})
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// This map will hold the uptimes parsed from the command output.
	uptime := make(map[DeployableUnitID]time.Duration)

	// If at least one of the pods corresponding to a DeployableUnit is not ready, then it will be
	// excluded from the returned dictionary.
	//
	// Take for example the fictitious "kubectl get pods ..." command output below:
	//   ...
	//   app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-9krrl  ready:True  readyLastTransitionTime:2019-10-03T16:45:48Z
	//   app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-hr86n  ready:False  readyLastTransitionTime:2019-09-30T13:20:39Z
	//   app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-l4lt5  ready:True  readyLastTransitionTime:2019-10-04T01:59:40Z
	//   ...
	// In this example, DeployableUnit "gold-chrome-gpu-baselineserver" will be excluded from the
	// returned dictionary because one of its pods is not yet ready.
	//
	// The dictionary below keeps track of which DeployableUnits to exclude from this method's output.
	excludeFromOutput := make(map[DeployableUnitID]bool)

	// We will parse each output line using this regular expression.
	re := regexp.MustCompile(`app:(?P<app>\S*)\s+podName:(?P<podName>\S+)\s+ready:(?P<ready>\S+)\s+readyLastTransitionTime:(?P<readyLastTransitionTime>\S+)`)

	// Iterate over all output lines.
	for _, line := range strings.Split(stdout, "\n") {
		// Skip empty line at the end.
		if line == "" {
			continue
		}

		// Parse line, e.g. "app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-9krrl  ready:True  readyLastTransitionTime:2019-10-03T16:45:48Z"
		matches := re.FindStringSubmatch(line)

		// If for whatever reason the regular expression does not match, skip to the next line.
		if len(matches) < 4 {
			continue
		}

		// Extract values from current line.
		app := matches[1]                     // e.g. "gold-chrome-gpu-baselineserver"
		ready := matches[3]                   // e.g. "True"
		readyLastTransitionTime := matches[4] // e.g. "2019-10-03T16:45:48Z"

		// Iterate over the given DeployableUnits; see if there is a DeployableUnit that matches the
		// current line.
		var unitID DeployableUnitID
		for _, unit := range units {
			if unit.CanonicalName() == app {
				unitID = unit.DeployableUnitID
			}
		}

		// If no DeployableUnit matches, skip to the next line. This is OK since "kubectl get pods"
		// returns information about all pods running on the cluster, and not just the ones we are
		// interested in.
		if unitID == (DeployableUnitID{}) {
			continue
		}

		// If the pod is not yet ready, we exclude its corresponding DeployableUnit from the method's
		// output.
		if ready != "True" {
			delete(uptime, unitID) // Delete it from the output if it was previously added.
			excludeFromOutput[unitID] = true
			continue
		}

		// If the DeployableUnit has been excluded from the output due to another of its pods not being
		// ready, skip to the next line.
		if _, ok := excludeFromOutput[unitID]; ok {
			continue
		}

		// Parse the timestamp, e.g. "2019-09-30T13:20:33Z".
		t, err := time.Parse(kubectlTimestampLayout, readyLastTransitionTime)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		// Compute the time duration since the pod corresponding to the current line has been ready.
		readyFor := now.Sub(t)

		// We'll report the uptime of the pod that became ready the most recently.
		if currentMin, ok := uptime[unitID]; !ok || (readyFor < currentMin) {
			uptime[unitID] = readyFor
		}
	}

	return uptime, nil
}

// forAllDeployableUnits applies all deployable units (including canaried units)
// to the given function.
func (g *Goldpushk) forAllDeployableUnits(f func(unit DeployableUnit) error) error {
	for _, unit := range g.deployableUnits {
		if err := f(unit); err != nil {
			return skerr.Wrap(err)
		}
	}
	for _, unit := range g.canariedDeployableUnits {
		if err := f(unit); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// now returns the current time in UTC or a mocked out time.
func (g *Goldpushk) now() time.Time {
	if g.fakeNow.IsZero() {
		return time.Now().UTC()
	}
	return g.fakeNow
}

// execCmd executes a command with the given arguments.
func (g *Goldpushk) execCmd(ctx context.Context, name string, args []string) error {
	cmd := makeExecCommand(name, args, g.verbose)
	if err := exec.Run(ctx, cmd); err != nil {
		return skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
	}
	return nil
}

// execCmdAndReturnStdout executes a command with the given arguments and returns its output.
func (g *Goldpushk) execCmdAndReturnStdout(ctx context.Context, name string, args []string) (string, error) {
	cmd := makeExecCommand(name, args, g.verbose)
	stdout, err := exec.RunCommand(ctx, cmd)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
	}
	return stdout, nil
}

// makeExecCommand returns an exec.Command for the given command and arguments.
func makeExecCommand(name string, args []string, debug bool) *exec.Command {
	cmd := &exec.Command{
		Name:        name,
		Args:        args,
		InheritPath: true,
		LogStderr:   true,
		LogStdout:   true,
	}
	if debug {
		cmd.Verbose = exec.Info
	}
	return cmd
}

// cmdToDebugStr returns a human-readable string representation of an *exec.Command.
func cmdToDebugStr(cmd *exec.Command) string {
	return fmt.Sprintf("%s %s", cmd.Name, strings.Join(cmd.Args, " "))
}
