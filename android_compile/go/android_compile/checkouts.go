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
	checkouts          = []string{}
	availableCheckouts = make(chan string)

	bucketHandle *storage.BucketHandle
)

func CheckoutsInit(numCheckouts int, workdir string) error {
	availableCheckouts = make(chan string, numCheckouts)
	// Create directories for all checkouts. Do nothing if they already exist.
	for i := 1; i <= numCheckouts; i++ {
		checkoutPath := filepath.Join(workdir, CHECKOUTS_TOPLEVEL_DIR, fmt.Sprintf("%s_%d", CHECKOUT_DIR_PREFIX, i))
		// EnsureDirPathExists ??
		if _, err := fileutil.EnsureDirExists(checkoutPath); err != nil {
			return fmt.Errorf("Error creating %s: %s", checkoutPath, err)
		}
		checkouts = append(checkouts, checkoutPath)
		addToChan(checkoutPath)
	}
	// Sync all checkouts simultaneously.
	// TODO(rmistry): Uncomment this...
	// UpdateCheckoutsInParallel()

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

// Also build here so that next run will do incremental builds?
// Purposely not throwing error here for now.
func UpdateCheckoutsInParallel() {
	fmt.Println("Updating all checkouts from the init thingy")
	var wg sync.WaitGroup
	for _, checkout := range checkouts {
		// Increment the WaitGroup counter.
		wg.Add(1)
		// Create and run a goroutine closure that updates the checkout.
		go func(c string) {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()
			if err := updateCheckout(c); err != nil {
				sklog.Errorf("Error when updating checkout in %s: %s", c, err)
			}
		}(checkout)
	}
	// Wait for all spawned goroutines to complete.
	wg.Wait()
	fmt.Println("Done updating all CHECKOUTS in parallel!")
}

//func UpdateCheckouts(numCheckouts int, workdir string) error {
//	for i := 1; i <= numCheckouts; i++ {
//		checkoutPath := filepath.Join(workdir, CHECKOUTS_TOPLEVEL_DIR, fmt.Sprintf("%s_%d", CHECKOUT_DIR_PREFIX, i))
//		// EnsureDirPathExists ??
//		if _, err := fileutil.EnsureDirExists(checkoutPath); err != nil {
//			return fmt.Errorf("Error creating %s: %s", checkoutPath, err)
//		}
//		UpdateCheckout(checkoutPath)
//	}
//	return nil
//}

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
		// MAKE ALL THESE ERRORS DESCRIPTIVE!!!
		return err
	}
	// Sync the current branch.
	if _, err := sk_exec.RunCwd(ctx, checkoutPath, repoToolPath, "sync", "-c", "-j32"); err != nil {
		return err
	}

	return nil
}

// TODO(rmistyr):L Use SkiaCHeckout
func ApplyPatch(checkoutPath string, issue, patchset int) error {
	// Should be common everywhere!
	ctx := context.Background()
	// Pass it in from one place or something?
	skiaInAndroid := filepath.Join(checkoutPath, "external", "skia")
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
	output, err := sk_exec.RunCwd(ctx, skiaInAndroid, "git", "fetch", common.REPO_SKIA, fmt.Sprintf("refs/changes/%02d/%d/%d", issuePostfix, issue, patchset))
	if err != nil {
		return fmt.Errorf("Failed to execute in %s Git %q in %s: %s", checkoutPath, output, err)
	}
	_, err = sk_exec.RunCwd(ctx, skiaInAndroid, "git", "reset", "--hard", "FETCH_HEAD")
	if err != nil {
		return fmt.Errorf("Failed to checkout FETCH_HEAD in %s: %s", checkoutPath, err)
	}
	_, err = sk_exec.RunCwd(ctx, skiaInAndroid, "git", "rebase")
	if err != nil {
		return fmt.Errorf("Failed to rebase in %s: %s", checkoutPath, err)
	}
	return nil
}

func ResetSkiaCheckout(ctx context.Context, skiaCheckout *git.Checkout) error {
	if _, err := skiaCheckout.Git(ctx, "clean", "-d", "-f"); err != nil {
		return err
	}
	if _, err := skiaCheckout.Git(ctx, "reset", "--hard", "origin/master"); err != nil {
		return err
	}
	return nil
}

func cleanCheckout(ctx context.Context, skiaCheckout *git.Checkout, androidSkiaHash string) error {
	// Clean the checkout and go back to the state before checking out origin/master.
	if err := ResetSkiaCheckout(ctx, skiaCheckout); err != nil {
		return fmt.Errorf("Error when resetting Skia checkout: %s", err)
	}
	if _, err := skiaCheckout.Git(ctx, "checkout", androidSkiaHash); err != nil {
		return fmt.Errorf("Failed to checkout %s: %s", androidSkiaHash, err)
	}
	if _, err := skiaCheckout.Git(ctx, "branch", "-D", TRY_BRANCH_NAME); err != nil {
		return fmt.Errorf("Failed to delete branch s: %s", TRY_BRANCH_NAME, err)
	}
	return nil
}

func PrepareSkiaCheckoutForCompile(ctx context.Context, userConfigContent []byte, skiaCheckout *git.Checkout) error {
	skiaPath := skiaCheckout.Dir()
	if err := android_skia_checkout.RunGnToBp(ctx, skiaPath); err != nil {
		return fmt.Errorf("Error when running gn_to_bp: %s", err)
	}

	// TODO(rmistry): How do I sync only buildtools? bin/sync brings everything in.
	if err := os.RemoveAll(filepath.Join(skiaPath, "third_party", "externals")); err != nil {
		// go back to previous state.
		return fmt.Errorf("Error when deleting third_party/externals: %s", err)
	}

	// Copy over the contents of the thingy now.
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

func addToChan(checkout string) {
	availableCheckouts <- checkout
}

// If compile fails then it does a compile again with the patch removed.
func ApplyAndCompilePatch(issue, patchset int, task *CompileTask, datastoreKey *datastore.Key) error {
	// Blocking call to wait for an available checkout.
	fmt.Println("WAITNG WAITING WAITING")
	checkoutPath := <-availableCheckouts
	defer addToChan(checkoutPath)
	fmt.Println("DONE DONE DONE DONE DONE")

	ctx := context.Background()

	skiaPath := filepath.Join(checkoutPath, "external", "skia")
	skiaCheckout, err := git.NewCheckout(ctx, ANDROID_SKIA_URL, path.Dir(skiaPath))
	if err != nil {
		return fmt.Errorf("Failed to create GitDir from %s: %s", skiaPath, err)
	}
	// Get the hash currently in Skia's checkout. We will checkout to this hash
	// after we are done updating to Skia ToT and compiling.
	androidSkiaHash, err := skiaCheckout.GitDir.FullHash(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("Failed to get hash from %s: %s", skiaPath, err)
	}

	// Copy SkUserConfigManual.h to a temp location to use after pointing Skia
	// to ToT.
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

	// EVERYWHERE AFTER THIS!!

	// Create a branch and have it track origin/master.
	if _, err := skiaCheckout.Git(ctx, "checkout", "-b", TRY_BRANCH_NAME, "-t", "origin/master"); err != nil {
		util.LogErr(cleanCheckout(ctx, skiaCheckout, androidSkiaHash))
		return fmt.Errorf("Error when creating %s in %s: %s", TRY_BRANCH_NAME, skiaPath, err)
	}

	// Apply Patch
	if err := ApplyPatch(checkoutPath, issue, patchset); err != nil {
		util.LogErr(cleanCheckout(ctx, skiaCheckout, androidSkiaHash))
		return fmt.Errorf("Could not apply the patch with issue %d and patchset %d: %s", issue, patchset, err)
	}

	if err := PrepareSkiaCheckoutForCompile(ctx, userConfigContent, skiaCheckout); err != nil {
		util.LogErr(cleanCheckout(ctx, skiaCheckout, androidSkiaHash))
		return fmt.Errorf("Could not prepare Skia checkout for compile: %s", err)
	}

	withPatchSuccess, gsWithPatchLink, err := CompileCheckout(ctx, checkoutPath, fmt.Sprintf("%d_withpatch_", datastoreKey.ID))
	if err != nil {
		return fmt.Errorf("Error when compiling checkout withpatch at %s: %s", checkoutPath, err)
	}
	task.WithPatchSucceeded = withPatchSuccess
	task.NoPatchLog = gsWithPatchLink
	if _, err := UpdateDSTask(datastoreKey, task); err != nil {
		return fmt.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
	}
	if !withPatchSuccess {
		// If this failed then check to see if a build without the patch will succeed.
		if err := ResetSkiaCheckout(ctx, skiaCheckout); err != nil {
			util.LogErr(cleanCheckout(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Error when resetting Skia checkout: %s", err)
		}
		// Checkout origin/master.
		if _, err := skiaCheckout.Git(ctx, "checkout", "origin/master"); err != nil {
			util.LogErr(cleanCheckout(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Failed to checkout origin/master: %s", err)
		}
		if err := PrepareSkiaCheckoutForCompile(ctx, userConfigContent, skiaCheckout); err != nil {
			util.LogErr(cleanCheckout(ctx, skiaCheckout, androidSkiaHash))
			return fmt.Errorf("Could not prepare Skia checkout for compile: %s", err)
		}

		noPatchSuccess, gsNoPatchLink, err := CompileCheckout(ctx, checkoutPath, fmt.Sprintf("%d_nopatch_", datastoreKey.ID))
		if err != nil {
			return fmt.Errorf("Error when compiling checkout nopatch at %s: %s", checkoutPath, err)
		}
		task.NoPatchSucceeded = noPatchSuccess
		task.WithPatchLog = gsNoPatchLink
		if _, err := UpdateDSTask(datastoreKey, task); err != nil {
			return fmt.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
		}
	}

	if err := cleanCheckout(ctx, skiaCheckout, androidSkiaHash); err != nil {
		return fmt.Errorf("Could not clean checkout: %s", err)
	}

	// NEED TODO TONS OF CLEANUPS!!
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")

	// Go back to previous state with:
	// rm -rf .deps_sha1 buildtools bin/gn third_party/externals; git reset --hard origin/master; git clean -f -d; git checkout a12b2369d34fb3d557ed1b3563d8f0935b93b173; git branch -D try_branch;

	return nil
}

// TODO(rmistry): Record how long it took to compile the different checkouts both with and without??
// TODO(rmistry): Have them pass in the output file name prefix!
// CompileCheckout runs compile.sh which compiles the provided Android checkout. Compile logs are
// then stored in the provided Google storage location with the specified prefix.
// Returns a link to the logs in Google Storage.
func CompileCheckout(ctx context.Context, checkoutPath, logFilePrefix string) (bool, string, error) {
	checkoutBase := path.Base(checkoutPath)
	sklog.Infof("Started compiling %s", checkoutBase)
	// Create metric and send it to a timer.
	compileTimesMetric := metrics2.GetFloat64Metric(fmt.Sprintf("android_compile_time_%s", checkoutBase))
	defer timer.NewWithMetric(fmt.Sprintf("Time taken to compile %s:", checkoutBase), compileTimesMetric).Stop()

	//ctx := context.Background()

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
