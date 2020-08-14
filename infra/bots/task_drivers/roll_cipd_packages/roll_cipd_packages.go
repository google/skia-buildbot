package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/lib/rotations"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	// Tag indicating the most recently uploaded version of a CIPD package.
	TAG_LATEST = "latest"

	// Tag prefixes.
	TAG_PREFIX_VERSION  = "version:"
	TAG_PREFIX_REPO     = "git_repository:"
	TAG_PREFIX_REVISION = "git_revision:"
)

var (
	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workdir       = flag.String("workdir", ".", "Working directory")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	rs, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if *gerritProject == "" {
		td.Fatalf(ctx, "--gerrit_project is required.")
	}
	if *gerritUrl == "" {
		td.Fatalf(ctx, "--gerrit_url is required.")
	}

	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Check out the code.
	co, err := checkout.EnsureGitCheckout(ctx, path.Join(wd, "repo"), rs)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Read packages from cipd.ensure.
	ensureFile := filepath.Join(co.Dir(), "cipd.ensure")
	var pkgs []*cipd.Package
	if err := td.Do(ctx, td.Props("Read cipd.ensure").Infra(), func(ctx context.Context) error {
		pkgs, err = cipd.ParseEnsureFile(ensureFile)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	sort.Sort(cipd.PackageSlice(pkgs))

	// Find the latest versions of the desired packages.
	c, err := auth_steps.InitHttpClient(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		td.Fatal(ctx, err)
	}
	var cc *cipd.Client
	if err := td.Do(ctx, td.Props("Create CIPD client").Infra(), func(ctx context.Context) error {
		cc, err = cipd.NewClient(c, *workdir)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	newVersions := make(map[*cipd.Package]string, len(pkgs))
	if err := td.Do(ctx, td.Props("Get latest package versions").Infra(), func(ctx context.Context) error {
		for _, pkg := range pkgs {
			// Fill in the placeholders.
			name := pkg.Name
			for placeholder, val := range map[string]string{
				"arch":     "amd64",
				"os":       "linux",
				"platform": "linux-amd64",
			} {
				name = strings.ReplaceAll(name, fmt.Sprintf("${%s}", placeholder), val)
			}
			if err := td.Do(ctx, td.Props(fmt.Sprintf("Find latest %s", name)).Infra(), func(ctx context.Context) error {
				// Find the latest version of the package.
				pin, err := cc.ResolveVersion(ctx, name, TAG_LATEST)
				if err != nil {
					return err
				}
				// Retrieve details of the package instance, including the full
				// set of refs and tags.
				desc, err := cc.Describe(ctx, name, pin.InstanceID)
				if err != nil {
					return err
				}
				newVersionTag := ""
				tags := make([]string, 0, len(desc.Tags))
				var repos []string
				var revs []string
				for _, tag := range desc.Tags {
					tags = append(tags, tag.Tag)

					// First preference: "version"
					if strings.HasPrefix(tag.Tag, TAG_PREFIX_VERSION) {
						newVersionTag = tag.Tag
						break
					}
					// Fall back to choosing the most recent
					// tagged commit based on repo+revision.
					if strings.HasPrefix(tag.Tag, TAG_PREFIX_REPO) {
						repos = append(repos, strings.TrimPrefix(tag.Tag, TAG_PREFIX_REPO))
					}
					if strings.HasPrefix(tag.Tag, TAG_PREFIX_REVISION) {
						revs = append(revs, strings.TrimPrefix(tag.Tag, TAG_PREFIX_REVISION))
					}
				}
				if newVersionTag == "" {
					// If more than one repo is listed, we need to match the
					// git_revision to the correct repo in order to obtain the
					// timestamp.
					// TODO(borenet): Is there ever more than one git_repository?
					commits := make([]*vcsinfo.LongCommit, 0, len(revs))
					for _, repo := range repos {
						r := gitiles.NewRepo(repo, c)
						for _, rev := range revs {
							// Ignore any error, in case we're looking
							// at the wrong repo.
							details, err := r.Details(ctx, rev)
							if err == nil {
								// Sanity check.
								if details.Hash == rev {
									commits = append(commits, details)
								} else {
									sklog.Errorf("Retrieved commit details do not match git_revision tag: expect %q but got %q", rev, details.Hash)
								}
							}
						}
					}
					if len(commits) > 0 {
						// Sort by timestamp, most recent first.
						sort.Sort(vcsinfo.LongCommitSlice(commits))
						newVersionTag = TAG_PREFIX_REVISION + commits[0].Hash
					}
				}
				if newVersionTag == "" {
					return fmt.Errorf("Unable to find a valid version tag in %+v", tags)
				}
				newVersions[pkg] = newVersionTag
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Write the new ensure file; we read the original and find-and-replace
	// the package versions.
	ensureBytes, err := os_steps.ReadFile(ctx, ensureFile)
	if err != nil {
		td.Fatal(ctx, err)
	}
	oldLines := strings.Split(string(ensureBytes), "\n")
	newLines := make([]string, 0, len(oldLines))
	for _, line := range oldLines {
		for _, pkg := range pkgs {
			if strings.HasPrefix(line, pkg.Name) {
				line = strings.ReplaceAll(line, pkg.Version, newVersions[pkg])
				break
			}
		}
		newLines = append(newLines, line)
	}
	if err := os_steps.WriteFile(ctx, ensureFile, []byte(strings.Join(newLines, "\n")), os.ModePerm); err != nil {
		td.Fatal(ctx, err)
	}

	// Run "go generate".
	if err := golang.InstallCommonDeps(ctx, co.Dir()); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := golang.Go(ctx, co.Dir(), "generate", "./..."); err != nil {
		td.Fatal(ctx, err)
	}

	// Regenerate tasks.json.
	if _, err := golang.Go(ctx, co.Dir(), "run", "./infra/bots/gen_tasks.go"); err != nil {
		td.Fatal(ctx, err)
	}

	// If we changed anything, upload a CL.
	reviewers, err := rotations.GetCurrentTrooper(ctx, c)
	if err != nil {
		td.Fatal(ctx, err)
	}
	g, err := gerrit_steps.Init(ctx, *local, *gerritUrl)
	if err != nil {
		td.Fatal(ctx, err)
	}
	isTryJob := *local || rs.Issue != ""
	if isTryJob {
		var i int64
		if err := td.Do(ctx, td.Props(fmt.Sprintf("Parse %q as int", rs.Issue)).Infra(), func(ctx context.Context) error {
			var err error
			i, err = strconv.ParseInt(rs.Issue, 10, 64)
			return err
		}); err != nil {
			td.Fatal(ctx, err)
		}
		ci, err := gerrit_steps.GetIssueProperties(ctx, g, i)
		if err != nil {
			td.Fatal(ctx, err)
		}
		if !util.In(ci.Owner.Email, reviewers) {
			reviewers = append(reviewers, ci.Owner.Email)
		}
	}
	if err := gerrit_steps.UploadCL(ctx, g, co, *gerritProject, git.DefaultBranch, rs.Revision, "Update CIPD Packages", reviewers, isTryJob); err != nil {
		td.Fatal(ctx, err)
	}
}
