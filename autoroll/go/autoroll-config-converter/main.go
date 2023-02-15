package main

import (
	"encoding/json"
	"flag"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config/conversion"
	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

func main() {
	// Flags.
	src := flag.String("src", "", "Source directory.")
	dst := flag.String("dst", "", "Destination directory. Outputs will mimic the structure of the source.")
	privacySandboxAndroidRepoURL := flag.String("privacy_sandbox_android_repo_url", "", "Repo URL for privacy sandbox on Android.")
	privacySandboxAndroidVersionsPath := flag.String("privacy_sandbox_android_versions_path", "", "Path to the file containing the versions of privacy sandbox on Android.")
	createCL := flag.Bool("create-cl", false, "If true, creates a CL if any changes were made.")
	srcRepo := flag.String("source-repo", "", "URL of the repo which triggered this run.")
	srcCommit := flag.String("source-commit", "", "Commit hash which triggered this run.")
	louhiExecutionID := flag.String("louhi-execution-id", "", "Execution ID of the Louhi flow.")
	louhiPubsubProject := flag.String("louhi-pubsub-project", "", "GCP project used for sending Louhi pub/sub notifications.")
	local := flag.Bool("local", false, "True if running locally.")

	flag.Parse()

	// We're using the task driver framework because it provides logging and
	// helpful insight into what's occurring as the program runs.
	fakeProjectId := ""
	fakeTaskId := ""
	fakeTaskName := ""
	output := "-"
	tdLocal := true
	ctx := td.StartRun(&fakeProjectId, &fakeTaskId, &fakeTaskName, &output, &tdLocal)
	defer td.EndRun(ctx)

	if *src == "" {
		td.Fatalf(ctx, "--src is required.")
	}
	if *dst == "" {
		td.Fatalf(ctx, "--dst is required.")
	}

	// Set up auth, load config variables.
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if !*local {
		srv, err := oauth2.NewService(ctx, option.WithTokenSource(ts))
		if err != nil {
			td.Fatal(ctx, err)
		}
		info, err := srv.Userinfo.V2.Me.Get().Do()
		if err != nil {
			td.Fatal(ctx, err)
		}
		sklog.Infof("Authenticated as %s", info.Email)
		if _, err := gitauth.New(ts, "/tmp/.gitcookies", true, info.Email); err != nil {
			td.Fatal(ctx, err)
		}
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	vars, err := conversion.CreateTemplateVars(ctx, client, *privacySandboxAndroidRepoURL, *privacySandboxAndroidVersionsPath)
	if err != nil {
		td.Fatal(ctx, err)
	}

	b, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		td.Fatal(ctx, err)
	}
	sklog.Infof("Using variables: %s", string(b))

	// Walk through the autoroller config directory. Create roller configs from
	// templates and convert roller configs to k8s configs.
	fsys := os.DirFS(*src)
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".cfg") {
			srcPath := filepath.Join(*src, path)
			sklog.Infof("Converting %s", srcPath)
			cfgBytes, err := ioutil.ReadFile(srcPath)
			if err != nil {
				return skerr.Wrapf(err, "failed to read roller config %s", srcPath)
			}

			if err := conversion.ConvertConfig(ctx, cfgBytes, path, *dst); err != nil {
				return skerr.Wrapf(err, "failed to convert config %s", path)
			}
		} else if strings.HasSuffix(d.Name(), ".tmpl") {
			tmplPath := filepath.Join(*src, path)
			sklog.Infof("Processing %s", tmplPath)
			tmplContents, err := ioutil.ReadFile(tmplPath)
			if err != nil {
				return skerr.Wrapf(err, "failed to read template file %s", tmplPath)
			}
			generatedConfigs, err := conversion.ProcessTemplate(ctx, client, path, string(tmplContents), vars)
			if err != nil {
				return skerr.Wrapf(err, "failed to process template file %s", path)
			}
			for path, cfgBytes := range generatedConfigs {
				if err := conversion.ConvertConfig(ctx, cfgBytes, path, *dst); err != nil {
					return skerr.Wrapf(err, "failed to convert config %s", path)
				}
			}
		}
		return nil
	}); err != nil {
		td.Fatalf(ctx, "Failed to read configs: %s", err)
	}

	// "git add" the directory.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, *dst, gitExec, "add", "-A"); err != nil {
		td.Fatal(ctx, err)
	}

	// Upload a CL.
	if *createCL {
		commitSubject := "Update autoroll k8s configs"
		if err := cd.MaybeUploadCL(ctx, *dst, commitSubject, *srcRepo, *srcCommit, *louhiPubsubProject, *louhiExecutionID); err != nil {
			td.Fatalf(ctx, "Failed to create CL: %s", err)
		}
	}
}
