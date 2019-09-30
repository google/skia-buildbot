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
// information about dummy services that can be deployed the public or corp clusters without
// disrupting any production services.
package goldpushk

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// Paths below are relative to $SKIA_INFRA_ROOT.
	k8sConfigTemplatesDir = "golden/k8s-config-templates"
	k8sInstancesDir       = "golden/k8s-instances"
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
	deployableUnits         []DeployableUnit
	canariedDeployableUnits []DeployableUnit
	rootPath                string // Path to the buildbot checkout.
	dryRun                  bool
	noCommit                bool

	// Other constructor parameters.
	skiaPublicConfigRepoUrl string
	skiaCorpConfigRepoUrl   string

	// Checked out Git repositories.
	skiaPublicConfigCheckout *git.TempCheckout
	skiaCorpConfigCheckout   *git.TempCheckout

	// The Kubernetes cluster that the kubectl command is currently configured to use.
	currentCluster cluster

	// Miscellaneous.
	unitTest bool // Disables confirmation prompt from unit tests.
}

// New is the Goldpushk constructor.
func New(deployableUnits []DeployableUnit, canariedDeployableUnits []DeployableUnit, skiaInfraRootPath string, dryRun, noCommit bool, skiaPublicConfigRepoUrl, skiaCorpConfigRepoUrl string) *Goldpushk {
	return &Goldpushk{
		deployableUnits:         deployableUnits,
		canariedDeployableUnits: canariedDeployableUnits,
		rootPath:                skiaInfraRootPath,
		dryRun:                  dryRun,
		noCommit:                noCommit,
		skiaPublicConfigRepoUrl: skiaPublicConfigRepoUrl,
		skiaCorpConfigRepoUrl:   skiaCorpConfigRepoUrl,
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

	// Check out Git repositories.
	if err := g.checkOutGitRepositories(ctx); err != nil {
		return skerr.Wrap(err)
	}
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

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
	if err := g.monitorCanaries(); err != nil {
		return skerr.Wrap(err)
	}

	// Deploy remaining DeployableUnits.
	if err := g.pushServices(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Monitor remaining DeployableUnits.
	if err := g.monitorServices(); err != nil {
		return skerr.Wrap(err)
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

// checkOutGitRepositories checks out the skia-public-config and
// skia-corp-config Git repositories.
func (g *Goldpushk) checkOutGitRepositories(ctx context.Context) error {
	fmt.Println()
	var err error
	if g.skiaPublicConfigCheckout, err = checkOutSingleGitRepository(ctx, g.skiaPublicConfigRepoUrl); err != nil {
		return skerr.Wrap(err)
	}
	if g.skiaCorpConfigCheckout, err = checkOutSingleGitRepository(ctx, g.skiaCorpConfigRepoUrl); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// checkOutSingleGitRepository checks out the Git repository at the given URL.
func checkOutSingleGitRepository(ctx context.Context, url string) (*git.TempCheckout, error) {
	checkout, err := git.NewTempCheckout(ctx, url)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to check out %s", url)
	}
	fmt.Printf("Cloned Git repository %s at %s.\n", url, string(checkout.GitDir))
	return checkout, nil
}

// regenerateConfigFiles regenerates the .yaml and .json5 files for each
// instance/service pair that will be deployed. Any generated files will be
// checked into the corresponding Git repository with configuration files.
func (g *Goldpushk) regenerateConfigFiles(ctx context.Context) error {
	// Iterate over all units to deploy (including canaries).
	return g.forAllDeployableUnits(func(unit DeployableUnit) error {
		// Path to the template file inside $SKIA_INFRA_ROOT.
		tPath := unit.getDeploymentFileTemplatePath(g.rootPath)

		// Path to the deployment file (.yaml) we will regenerate inside the
		// corresponding skia-{public,corp}-config Git repository.
		oPath := g.getDeploymentFilePath(unit)

		// Regenerate .yaml file.
		if err := g.expandTemplate(ctx, unit.Instance, tPath, oPath); err != nil {
			return skerr.Wrapf(err, "error while regenerating %s", oPath)
		}

		// If the DeployableUnit has a ConfigMap template (as opposed to a ConfigMap
		// file that already exists in $SKIA_INFRA_ROOT)), the template must be
		// expanded and saved to the corresponding skia-{public,corp}-config Git
		// repository.
		if unit.configMapTemplate != "" {
			// Path to the template file inside $SKIA_INFRA_ROOT.
			tPath = unit.getConfigMapFileTemplatePath(g.rootPath)

			// Path to the ConfigMap file (.json5) to be regenerated inside the
			// corresponding skia-{public,corp}-config repository.
			oPath, ok := g.getConfigMapFilePath(unit)
			if !ok {
				return fmt.Errorf("goldpushk.getConfigMapFilePath() failed for %s; this is probably a bug", unit.CanonicalName())
			}

			// Regenerate .json5 file.
			if err := g.expandTemplate(ctx, unit.Instance, tPath, oPath); err != nil {
				return skerr.Wrapf(err, "error while regenerating %s", oPath)
			}
		}

		return nil
	})
}

// getDeploymentFilePath returns the path to the deployment file (.yaml) for the
// given DeployableUnit inside the corresponding skia-{public,corp}-config Git
// repository.
func (g *Goldpushk) getDeploymentFilePath(unit DeployableUnit) string {
	return filepath.Join(g.getGitRepoRootPath(unit), unit.CanonicalName()+".yaml")
}

// getConfigMapFilePath returns the path to the ConfigFile (.json5) for the
// given DeployableUnit.
//
// If the DeployableUnit has a ConfigMap file (e.g. gold-skiapublic-skiacorrectness)
// this will be a path inside $SKIA_INFRA_ROOT.
//
// If the DeployableUnit has a ConfigMap template (e.g. gold-skia-ingestion-bt)
// this will be a path inside the corresponding skia-{public,corp}-config Git
// repository pointing to the expanded ConfigMap template.
//
// If neither is true, it will return ("", false).
func (g *Goldpushk) getConfigMapFilePath(unit DeployableUnit) (string, bool) {
	if unit.configMapFile != "" {
		return filepath.Join(g.rootPath, unit.configMapFile), true
	} else if unit.configMapName != "" && unit.configMapTemplate != "" {
		return filepath.Join(g.getGitRepoRootPath(unit), unit.configMapName+".json5"), true
	} else {
		return "", false
	}
}

// getGitRepoRoothPath returns the path to the checked out Git repository in
// which the config files for the given DeployableUnit should be checked in
// (i.e. one of skia-{public,corp}-config.
func (g *Goldpushk) getGitRepoRootPath(unit DeployableUnit) string {
	if unit.internal {
		return string(g.skiaCorpConfigCheckout.GitDir)
	}
	return string(g.skiaPublicConfigCheckout.GitDir)
}

// expandTemplate executes the kube-conf-gen command with the given arguments in
// a fashion similar to gen-k8s-config.sh.
func (g *Goldpushk) expandTemplate(ctx context.Context, instance Instance, templatePath, outputPath string) error {
	goldCommonJson5 := filepath.Join(g.rootPath, k8sConfigTemplatesDir, "gold-common.json5")
	instanceStr := string(instance)
	instanceJson5 := filepath.Join(g.rootPath, k8sInstancesDir, instanceStr+"-instance.json5")

	cmd := &exec.Command{
		Name: "kube-conf-gen",
		// Notes on the kube-conf-gen arguments used:
		//   - Flag "-extra INSTANCE_ID:<instanceStr>" binds template variable
		//     INSTANCE_ID to instanceStr.
		//   - Flag "-strict" will make kube-conf-gen fail in the presence of
		//     unsupported types, missing data, etc.
		//   - Flag "-parse_conf=false" prevents the values read from the JSON5
		//     config files provided with -c <json5-file> from being converted to
		//     strings.
		Args:        []string{"-c", goldCommonJson5, "-c", instanceJson5, "-extra", "INSTANCE_ID:" + instanceStr, "-t", templatePath, "-parse_conf=false", "-strict", "-o", outputPath},
		InheritPath: true,
		LogStderr:   true,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, cmd); err != nil {
		return skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
	}
	sklog.Infof("Generated %s", outputPath)
	return nil
}

// commitConfigFiles prints out a summary of the changes to be committed to
// skia-{public,corp}-config, asks for confirmation and pushes those changes.
func (g *Goldpushk) commitConfigFiles(ctx context.Context) (bool, error) {
	// Print out summary of changes (git status -s).
	err := g.forAllGitRepos(func(repo *git.TempCheckout, name string) error {
		return printOutGitStatus(ctx, repo, name)
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}

	// Skip if noCommit is true.
	if g.noCommit {
		fmt.Println("\nSkipping commit step.")
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

	// Add, commit and push changes.
	fmt.Println()
	err = g.forAllGitRepos(func(repo *git.TempCheckout, name string) error {
		fmt.Printf("Pushing changes to %s.\n", name)
		if _, err := repo.Git(ctx, "add", "."); err != nil {
			return skerr.Wrap(err)
		}
		if _, err := repo.Git(ctx, "commit", "-m", "Push"); err != nil {
			return skerr.Wrap(err)
		}
		if _, err := repo.Git(ctx, "push", "origin", "master"); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}

	return true, nil
}

// printOutGitStatus runs "git status -s" on the given checkout and prints its output to stdout.
func printOutGitStatus(ctx context.Context, checkout *git.TempCheckout, repoName string) error {
	msg, err := checkout.Git(ctx, "status", "-s")
	if err != nil {
		return skerr.Wrap(err)
	}
	if len(msg) == 0 {
		fmt.Printf("\nNo changes to be pushed to %s.\n", repoName)
	} else {
		fmt.Printf("\nChanges to be pushed to %s:\n", repoName)
		fmt.Print(msg)
	}
	return nil
}

// pushCanaries deploys the canaried deployable units.
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

func (g *Goldpushk) monitorCanaries() error {
	// TODO(lovisolo)
	fmt.Println("\nMonitoring not yet implemented.")
	return nil
}

// pushServices deploys the non-canaried deployable units.
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

func (g *Goldpushk) monitorServices() error {
	// TODO(lovisolo)
	fmt.Println("\nMonitoring not yet implemented.")
	return nil
}

// pushDeployableUnits takes a slice of DeployableUnits and pushes them to their corresponding
// clusters.
func (g *Goldpushk) pushDeployableUnits(ctx context.Context, units []DeployableUnit) error {
	for _, unit := range units {
		if err := g.pushSingleDeployableUnit(ctx, unit); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// pushSingleDeployableUnit pushes the given DeployableUnit to the corresponding cluster by running
// "kubectl apply -f path/to/config.yaml".
func (g *Goldpushk) pushSingleDeployableUnit(ctx context.Context, unit DeployableUnit) error {
	// Get the cluster corresponding to the given DeployableUnit.
	cluster := clusterSkiaPublic
	if unit.internal {
		cluster = clusterSkiaCorp
	}

	// Switch clusters.
	if err := g.switchClusters(ctx, cluster); err != nil {
		return skerr.Wrap(err)
	}

	// Push ConfigMap if the DeployableUnit requires one.
	if err := g.maybePushConfigMap(ctx, unit); err != nil {
		return skerr.Wrap(err)
	}

	// Push DeployableUnit.
	path := g.getDeploymentFilePath(unit)
	fmt.Printf("%s: applying %s.\n", unit.CanonicalName(), path)
	cmd := &exec.Command{
		Name:        "kubectl",
		Args:        []string{"apply", "-f", path},
		InheritPath: true,
		LogStderr:   true,
		LogStdout:   true,
	}
	if err := exec.Run(ctx, cmd); err != nil {
		return skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
	}

	return nil
}

// maybePushConfigMap pushes the ConfigMap required by the given DeployableUnit, if it needs one.
func (g *Goldpushk) maybePushConfigMap(ctx context.Context, unit DeployableUnit) error {
	// If the DeployableUnit requires a ConfigMap, delete and recreate it.
	if path, ok := g.getConfigMapFilePath(unit); ok {
		fmt.Printf("%s: creating ConfigMap named \"%s\" from file %s.\n", unit.CanonicalName(), unit.configMapName, path)

		// Delete existing ConfigMap.
		cmd := &exec.Command{
			Name:        "kubectl",
			Args:        []string{"delete", "configmap", unit.configMapName},
			InheritPath: true,
			LogStderr:   true,
			LogStdout:   true,
		}
		if err := exec.Run(ctx, cmd); err != nil {
			// TODO(lovisolo): Figure out a less brittle way to detect exit status 1.
			if strings.HasPrefix(err.Error(), "Command exited with exit status 1") {
				sklog.Infof("Did not delete ConfigMap %s as it does not exist on the cluster.", unit.configMapName)
			} else {
				return skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
			}
		}

		// Create new ConfigMap.
		cmd = &exec.Command{
			Name:        "kubectl",
			Args:        []string{"create", "configmap", unit.configMapName, "--from-file", path},
			InheritPath: true,
			LogStderr:   true,
			LogStdout:   true,
		}
		if err := exec.Run(ctx, cmd); err != nil {
			return skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
		}
	}
	return nil
}

// switchClusters runs the "gcloud" command necessary to switch kubectl to the given cluster.
func (g *Goldpushk) switchClusters(ctx context.Context, cluster cluster) error {
	if g.currentCluster != cluster {
		sklog.Infof("Switching to cluster %s\n", cluster.name)
		cmd := &exec.Command{
			Name:        "gcloud",
			Args:        []string{"container", "clusters", "get-credentials", cluster.name, "--zone", "us-central1-a", "--project", cluster.projectID},
			InheritPath: true,
			LogStderr:   true,
			LogStdout:   true,
		}
		if err := exec.Run(ctx, cmd); err != nil {
			return skerr.Wrapf(err, "failed to run %s", cmdToDebugStr(cmd))
		}
		g.currentCluster = cluster
	}
	return nil
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

// forAllGitRepos applies the *git.TempCheckouts for skia-{public,corp}-config to the given
// function.
func (g *Goldpushk) forAllGitRepos(f func(repo *git.TempCheckout, name string) error) error {
	if err := f(g.skiaPublicConfigCheckout, "skia-public-config"); err != nil {
		return skerr.Wrap(err)
	}
	if err := f(g.skiaCorpConfigCheckout, "skia-corp-config"); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// cmdToDebugStr returns a human-readable string representation of an *exec.Command.
func cmdToDebugStr(cmd *exec.Command) string {
	return fmt.Sprintf("%s %s", cmd.Name, strings.Join(cmd.Args, " "))
}
