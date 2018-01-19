package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/android_skia_checkout"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	sk_exec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
)

const (
	CHECKOUTS_TOPLEVEL_DIR = "checkouts"
	CHECKOUT_DIR_PREFIX    = "checkout"

	ANDROID_MANIFEST_URL = "https://googleplex-android.googlesource.com/a/platform/manifest"
	ANDROID_SKIA_URL     = "https://googleplex-android.googlesource.com/a/platform/external/skia"

	TRY_BRANCH_NAME = "try_branch"

	COMPILE_TASK_LOGS_BUCKET = "android-compile-logs"
)

var (
	availableCheckoutsChan = make(chan string)

	bucketHandle *storage.BucketHandle
)

func CheckoutsInit(numCheckouts int, workdir string) error {
	availableCheckouts := []string{}
	availableCheckoutsChan = make(chan string, numCheckouts)
	// Populate the channel with available checkouts.
	for i := 1; i <= numCheckouts; i++ {
		checkoutPath := filepath.Join(workdir, CHECKOUTS_TOPLEVEL_DIR, fmt.Sprintf("%s_%d", CHECKOUT_DIR_PREFIX, i))
		if _, err := fileutil.EnsureDirExists(checkoutPath); err != nil {
			return fmt.Errorf("Error creating %s: %s", checkoutPath, err)
		}
		availableCheckouts = append(availableCheckouts, checkoutPath)
		addToCheckoutsChannel(checkoutPath)
	}

	// Sync all checkouts simultaneously.
	updateCheckoutsInParallel(availableCheckouts)

	// Get a handle to the Google Storage bucket logs will be stored in.
	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		return fmt.Errorf("Problem setting up client OAuth: %s", err)
	}
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("Failed to create a Google Storage API client: %s", err)
	}
	bucketHandle = storageClient.Bucket(COMPILE_TASK_LOGS_BUCKET)
	return nil
}

// UpdateCheckoutsInParallel updates all Android checkouts in parallel.
func updateCheckoutsInParallel(checkouts []string) {
	sklog.Info("Updating all checkouts in parallel.")
	var wg sync.WaitGroup
	for _, checkout := range checkouts {
		wg.Add(1)
		// Create and run a goroutine closure that updates the checkout.
		go func(c string) {
			defer wg.Done()
			if err := updateCheckout(c); err != nil {
				sklog.Errorf("Error when updating checkout in %s: %s", c, err)
			}
		}(checkout)
	}
	wg.Wait()
	sklog.Info("Done updating all checkouts in parallel.")
}

// updateCheckout updates the Android checkout using the repo tool at the
// specified path.
func updateCheckout(checkoutPath string) error {
	checkoutBase := path.Base(checkoutPath)
	sklog.Infof("Started updating %s", checkoutBase)
	// Create metric and send it to a timer.
	syncTimesMetric := metrics2.GetFloat64Metric(fmt.Sprintf("android_sync_time_%s", checkoutBase))
	defer timer.NewWithMetric(fmt.Sprintf("Time taken to update %s:", checkoutBase), syncTimesMetric).Stop()

	user, err := user.Current()
	if err != nil {
		return err
	}
	repoToolPath := path.Join(user.HomeDir, "bin", "repo")
	ctx := context.Background()
	// Run repo init and sync commands.
	if _, err := sk_exec.RunCwd(ctx, checkoutPath, repoToolPath, "init", "-u", ANDROID_MANIFEST_URL, "-g", "all,-notdefault,-darwin", "-b", "master"); err != nil {
		return fmt.Errorf("Failed to init the repo at %s: %s", checkoutBase, err)
	}
	// Sync the current branch.
	if _, err := sk_exec.RunCwd(ctx, checkoutPath, repoToolPath, "sync", "-c", "-j32"); err != nil {
		return fmt.Errorf("Failed to sync the repo at %s: %s", checkoutBase, err)
	}

	return nil
}

// applyPatch applies a patch from the specified issue and patchset to the
// specified Skia checkout.
func applyPatch(ctx context.Context, skiaCheckout *git.Checkout, issue, patchset int) error {
	// Run a git fetch for the branch where gerrit stores patches.
	//
	//  refs/changes/46/4546/1
	//                |  |   |
	//                |  |   +-> Patch set.
	//                |  |
	//                |  +-> Issue ID.
	//                |
	//                +-> Last two digits of Issue ID.
	issuePostfix := issue % 100
	if err := skiaCheckout.FetchRefFromRepo(ctx, common.REPO_SKIA, fmt.Sprintf("refs/changes/%02d/%d/%d", issuePostfix, issue, patchset)); err != nil {
		return fmt.Errorf("Failed to fetch ref in %s: %s", skiaCheckout.Dir(), err)
	}
	if _, err := skiaCheckout.Git(ctx, "reset", "--hard", "FETCH_HEAD"); err != nil {
		return fmt.Errorf("Failed to checkout FETCH_HEAD in %s: %s", skiaCheckout.Dir(), err)
	}
	if _, err := skiaCheckout.Git(ctx, "rebase"); err != nil {
		return fmt.Errorf("Failed to rebase in %s: %s", skiaCheckout.Dir(), err)
	}
	return nil
}

func deleteTryBranch(ctx context.Context, skiaCheckout *git.Checkout) error {
	if _, err := skiaCheckout.Git(ctx, "branch", "-D", TRY_BRANCH_NAME); err != nil {
		return fmt.Errorf("Failed to delete branch s: %s", TRY_BRANCH_NAME, err)
	}
	return nil
}

func resetSkiaCheckout(ctx context.Context, skiaCheckout *git.Checkout, resetCommit string) error {
	if _, err := skiaCheckout.Git(ctx, "clean", "-d", "-f"); err != nil {
		return err
	}
	resetCommand := []string{"reset", "--hard"}
	if resetCommit != "" {
		resetCommand = append(resetCommand, resetCommit)
	}
	if _, err := skiaCheckout.Git(ctx, resetCommand...); err != nil {
		return err
	}
	return nil
}

//  undoes all changes to make the Skia checkout go back to the
// state it was in in the Androd repo.
func goBackToOriginalSkiaState(ctx context.Context, skiaCheckout *git.Checkout, androidSkiaHash string) error {
	// Clean the checkout and go back to the state before checking out origin/master.
	if err := resetSkiaCheckout(ctx, skiaCheckout, "origin/master"); err != nil {
		return fmt.Errorf("Error when resetting Skia checkout: %s", err)
	}
	if _, err := skiaCheckout.Git(ctx, "checkout", androidSkiaHash); err != nil {
		return fmt.Errorf("Failed to checkout %s: %s", androidSkiaHash, err)
	}
	if err := deleteTryBranch(ctx, skiaCheckout); err != nil {
		return fmt.Errorf("Failed to delete branch s: %s", TRY_BRANCH_NAME, err)
	}
	return nil
}

func prepareSkiaCheckoutForCompile(ctx context.Context, userConfigContent []byte, skiaCheckout *git.Checkout) error {
	skiaPath := skiaCheckout.Dir()
	if err := android_skia_checkout.RunGnToBp(ctx, skiaPath); err != nil {
		return fmt.Errorf("Error when running gn_to_bp: %s", err)
	}
	// TODO(rmistry): Cannot compile Android with third_party/externals. Find
	// a less hackier way around this.
	if err := os.RemoveAll(filepath.Join(skiaPath, "third_party", "externals")); err != nil {
		// go back to previous state.
		return fmt.Errorf("Error when deleting third_party/externals: %s", err)
	}
	// Create SkUserConfigManual file since it is required for compilation.
	f, err := os.Create(filepath.Join(skiaPath, android_skia_checkout.SkUserConfigManualRelPath))
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", android_skia_checkout.SkUserConfigManualRelPath, err)
	}
	defer util.Close(f)
	if _, err := f.Write(userConfigContent); err != nil {
		return fmt.Errorf("Could not write to %s: %s", android_skia_checkout.SkUserConfigManualRelPath, err)
	}

	return nil
}

func addToCheckoutsChannel(checkout string) {
	availableCheckoutsChan <- checkout
}

// If compile fails then it does a compile again with the patch removed.
// Need lots of documentaiton there.. explaning all git commands this is doing
// and what it does for patches and hashes....
func RunCompileTask(task *CompileTask, datastoreKey *datastore.Key) error {
	// Blocking call to wait for an available checkout.
	checkoutPath := <-availableCheckoutsChan
	defer addToCheckoutsChannel(checkoutPath)

	// Update task with the checkout it is in for display in the UI and for
	// easier debugging.
	task.Checkout = path.Base(checkoutPath)
	if _, err := UpdateDSTask(datastoreKey, task); err != nil {
		return fmt.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
	}

	ctx := context.Background()
	skiaPath := filepath.Join(checkoutPath, "external", "skia")
	skiaCheckout, err := git.NewCheckout(ctx, ANDROID_SKIA_URL, path.Dir(skiaPath))
	if err != nil {
		return fmt.Errorf("Failed to create GitDir from %s: %s", skiaPath, err)
	}

	// Remove any leftover artifacts before updating the checkout.
	if err := resetSkiaCheckout(ctx, skiaCheckout, ""); err != nil {
		return fmt.Errorf("Error when resetting Skia checkout: %s", err)
	}
	// Checkout Android's built in remote "goog/master" incase the checkout is
	// tracking something else.
	if _, err := skiaCheckout.Git(ctx, "checkout", "goog/master"); err != nil {
		return fmt.Errorf("Failed to checkout goog/master in %s: %s", checkoutPath, err)
	}
	if err := deleteTryBranch(ctx, skiaCheckout); err != nil {
		// Do nothing. The branch does not exist which is ok.
	}
	if err := updateCheckout(checkoutPath); err != nil {
		return fmt.Errorf("Error when updating checkout in %s: %s", checkoutPath, err)
	}

	// Get the Android hash currently in Skia's checkout. We will checkout to
	// this hash after we are done updating to Skia master/origin and compiling.
	androidSkiaHash, err := skiaCheckout.GitDir.FullHash(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("Failed to get hash from %s: %s", skiaPath, err)
	}

	// Get contents of SkUserConfigManual.h. We will use this after updating
	// Skia to master/origin and before compiling.
	userConfigContent, err := ioutil.ReadFile(filepath.Join(skiaPath, android_skia_checkout.SkUserConfigManualRelPath))
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", android_skia_checkout.SkUserConfigManualRelPath, err)
	}

	// Add origin remote that points to the Skia repo.
	if err := skiaCheckout.AddRemote(ctx, "origin", common.REPO_SKIA); err != nil {
		return fmt.Errorf("Error when adding origin remote: %s", err)
	}
	// Fetch origin without updating checkout
	if err := skiaCheckout.Fetch(ctx); err != nil {
		return fmt.Errorf("Error when fetching origin: %s", err)
	}

	// Create a branch and have it track origin/master.
	if _, err := skiaCheckout.Git(ctx, "checkout", "-b", TRY_BRANCH_NAME, "-t", "origin/master"); err != nil {
		util.LogErr(goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash))
		return fmt.Errorf("Error when creating %s in %s: %s", TRY_BRANCH_NAME, skiaPath, err)
	}

	trybotRun := (task.Hash == "")
	if trybotRun {
		// Apply Patch
		if err := applyPatch(ctx, skiaCheckout, task.Issue, task.PatchSet); err != nil {
			util.LogErr(goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Could not apply the patch with issue %d and patchset %d: %s", task.Issue, task.PatchSet, err)
		}
	} else {
		// Checkout the specified Skia hash.
		if _, err := skiaCheckout.Git(ctx, "checkout", task.Hash); err != nil {
			return fmt.Errorf("Failed to checkout Skia hash %s: %s", task.Hash, err)
		}
	}

	if err := prepareSkiaCheckoutForCompile(ctx, userConfigContent, skiaCheckout); err != nil {
		util.LogErr(goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash))
		return fmt.Errorf("Could not prepare Skia checkout for compile: %s", err)
	}

	// Do the with patch compilation.
	withPatchSuccess, gsWithPatchLink, err := compileCheckout(ctx, checkoutPath, fmt.Sprintf("%d_withpatch_", datastoreKey.ID))
	if err != nil {
		return fmt.Errorf("Error when compiling checkout withpatch at %s: %s", checkoutPath, err)
	}
	task.WithPatchSucceeded = withPatchSuccess
	task.NoPatchLog = gsWithPatchLink
	if _, err := UpdateDSTask(datastoreKey, task); err != nil {
		return fmt.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
	}

	if !withPatchSuccess && trybotRun {
		// If this failed then check to see if a build without the patch will succeed.
		if err := resetSkiaCheckout(ctx, skiaCheckout, "origin/master"); err != nil {
			util.LogErr(goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Error when resetting Skia checkout: %s", err)
		}
		// Checkout origin/master.
		if _, err := skiaCheckout.Git(ctx, "checkout", "origin/master"); err != nil {
			util.LogErr(goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Failed to checkout origin/master: %s", err)
		}
		if err := prepareSkiaCheckoutForCompile(ctx, userConfigContent, skiaCheckout); err != nil {
			util.LogErr(goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Could not prepare Skia checkout for compile: %s", err)
		}

		// Do the no patch compilation.
		noPatchSuccess, gsNoPatchLink, err := compileCheckout(ctx, checkoutPath, fmt.Sprintf("%d_nopatch_", datastoreKey.ID))
		if err != nil {
			return fmt.Errorf("Error when compiling checkout nopatch at %s: %s", checkoutPath, err)
		}
		task.NoPatchSucceeded = noPatchSuccess
		task.WithPatchLog = gsNoPatchLink
		if _, err := UpdateDSTask(datastoreKey, task); err != nil {
			return fmt.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
		}
	}

	if err := goBackToOriginalSkiaState(ctx, skiaCheckout, androidSkiaHash); err != nil {
		return fmt.Errorf("Could not clean checkout: %s", err)
	}

	return nil
}

// compileCheckout runs compile.sh which compiles the provided Android checkout.
// Compile logs are then stored in the provided Google storage location with the
// specified prefix.
// Returns whether the compilation was successful and a link to the compilation
// logs in Google Storage.
func compileCheckout(ctx context.Context, checkoutPath, logFilePrefix string) (bool, string, error) {
	checkoutBase := path.Base(checkoutPath)
	sklog.Infof("Started compiling %s", checkoutBase)
	// Create metric and send it to a timer.
	compileTimesMetric := metrics2.GetFloat64Metric(fmt.Sprintf("android_compile_time_%s", checkoutBase))
	defer timer.NewWithMetric(fmt.Sprintf("Time taken to compile %s:", checkoutBase), compileTimesMetric).Stop()

	// Find the compile script.
	_, currentFile, _, _ := runtime.Caller(0)
	compileScript := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "compile.sh")
	// Execute the compile script pointing it to the checkout.
	command := exec.Command(compileScript, checkoutPath)
	logFile, err := ioutil.TempFile(*workdir, logFilePrefix)
	defer util.Remove(logFile.Name())
	if err != nil {
		return false, "", fmt.Errorf("Could not create log file")
	}
	command.Stdout = io.MultiWriter(logFile, os.Stdout)
	command.Stderr = command.Stdout

	// Execute the command and determine if it was successful
	compileSuccess := (command.Run() == nil)

	// Put the log file in Google Storage.
	target := bucketHandle.Object(filepath.Base(logFile.Name()))
	writer := target.NewWriter(ctx)
	writer.ObjectAttrs.ContentType = "text/plain"
	// Make uploaded logs readable by google.com domain.
	writer.ObjectAttrs.ACL = []storage.ACLRule{{Entity: "domain-google.com", Role: storage.RoleReader}}
	defer util.Close(writer)

	f, err := os.Open(logFile.Name())
	defer f.Close()
	// Write the actual data.
	if _, err := io.Copy(writer, f); err != nil {
		return compileSuccess, "", fmt.Errorf("Could not write %s to google storage: %s", logFile.Name(), err)
	}
	return compileSuccess, fmt.Sprintf("https://storage.cloud.google.com/%s/%s", COMPILE_TASK_LOGS_BUCKET, filepath.Base(logFile.Name())), nil
}
