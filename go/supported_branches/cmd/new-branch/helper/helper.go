package helper

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/supported_branches"
	"go.skia.org/infra/go/util"
)

const DefaultMainStarFile = "main.star"

// AddSupportedBranch adds a new supported branch, optionally deleting an old
// supported branch.
func AddSupportedBranch(repoUrl, branch, owner, deleteBranch string, excludeTrybots []string, submit bool) error {
	newRef := git.FullyQualifiedBranchName(branch)
	excludeTrybotRegexp := make([]*regexp.Regexp, 0, len(excludeTrybots))
	for _, excludeTrybot := range excludeTrybots {
		re, err := regexp.Compile(excludeTrybot)
		if err != nil {
			return skerr.Wrapf(err, "failed to compile regular expression from %q", excludeTrybot)
		}
		excludeTrybotRegexp = append(excludeTrybotRegexp, re)
	}

	// Setup.
	wd, err := ioutil.TempDir("", "new-branch")
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.RemoveAll(wd)

	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_GERRIT)
	if err != nil {
		return skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gUrl := strings.Split(repoUrl, ".googlesource.com")[0] + "-review.googlesource.com"
	g, err := gerrit.NewGerrit(gUrl, client)
	if err != nil {
		return skerr.Wrap(err)
	}
	repo := gitiles.NewRepo(repoUrl, client)
	ctx := context.Background()
	baseCommitInfo, err := repo.Details(ctx, cq.CQ_CFG_REF)
	if err != nil {
		return skerr.Wrap(err)
	}
	baseCommit := baseCommitInfo.Hash
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.RemoveAll(tmp)

	// Download the CQ config file and modify it.
	mainStarFile := filepath.Join(tmp, DefaultMainStarFile)
	if err := repo.DownloadFileAtRef(ctx, DefaultMainStarFile, baseCommit, mainStarFile); err != nil {
		return skerr.Wrapf(err, "failed to download %q", DefaultMainStarFile)
	}
	if err := cq.WithUpdateCQConfig(ctx, mainStarFile, filepath.Join(tmp, "generated"), func(f *build.File) error {
		_, _, err := cq.FindExprForBranch(f, branch)
		if err == nil {
			_, _ = fmt.Fprintf(os.Stderr, "Already have %s in %s; not adding a duplicate.\n", newRef, cq.CQ_CFG_FILE)
		} else {
			if err := cq.CloneBranch(f, git.MasterBranch, git.BranchBaseName(branch), false, false, excludeTrybotRegexp); err != nil {
				return skerr.Wrap(err)
			}
		}
		if deleteBranch != "" {
			if _, _, err := cq.FindExprForBranch(f, deleteBranch); err == nil {
				if err := cq.DeleteBranch(f, deleteBranch); err != nil {
					return skerr.Wrap(err)
				}
			}
		}
		return nil
	}); err != nil {
		return skerr.Wrap(err)
	}

	// Download and modify the supported-branches.json file.
	branchesContents, err := repo.ReadFileAtRef(ctx, supported_branches.SUPPORTED_BRANCHES_FILE, baseCommit)
	if err != nil {
		return skerr.Wrap(err)
	}
	sbc, err := supported_branches.DecodeConfig(bytes.NewReader(branchesContents))
	if err != nil {
		return skerr.Wrap(err)
	}
	deleteRef := ""
	if deleteBranch != "" {
		deleteRef = git.FullyQualifiedBranchName(deleteBranch)
	}
	foundNewRef := false
	newBranches := make([]*supported_branches.SupportedBranch, 0, len(sbc.Branches)+1)
	for _, sb := range sbc.Branches {
		if deleteRef == "" || deleteRef != sb.Ref {
			newBranches = append(newBranches, sb)
		}
		if sb.Ref == newRef {
			foundNewRef = true
		}
	}
	if foundNewRef {
		_, _ = fmt.Fprintf(os.Stderr, "Already have %s in %s; not adding a duplicate.\n", newRef, supported_branches.SUPPORTED_BRANCHES_FILE)
	} else {
		newBranches = append(newBranches, &supported_branches.SupportedBranch{
			Ref:   newRef,
			Owner: owner,
		})
	}
	sbc.Branches = newBranches
	buf := bytes.Buffer{}
	if err := supported_branches.EncodeConfig(&buf, sbc); err != nil {
		return skerr.Wrap(err)
	}

	// Create the Gerrit CL.
	commitMsg := fmt.Sprintf("Add supported branch %s", branch)
	if deleteBranch != "" {
		commitMsg += fmt.Sprintf(", remove %s", deleteBranch)
	}
	repoSplit := strings.Split(repoUrl, "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	changes := map[string]string{
		supported_branches.SUPPORTED_BRANCHES_FILE: string(buf.Bytes()),
	}
	if err := filepath.Walk(tmp, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			relpath, err := filepath.Rel(tmp, path)
			if err != nil {
				return skerr.Wrap(err)
			}
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				return skerr.Wrap(err)
			}
			remoteContents, err := repo.ReadFileAtRef(ctx, relpath, cq.CQ_CFG_REF)
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve %q", relpath)
			}
			if string(contents) != string(remoteContents) {
				changes[relpath] = string(contents)
			}
		}
		return nil
	}); err != nil {
		return skerr.Wrap(err)
	}
	ci, err := gerrit.CreateCLWithChanges(ctx, g, project, cq.CQ_CFG_REF, commitMsg, baseCommit, changes, submit)
	if ci != nil {
		fmt.Println(fmt.Sprintf("Uploaded change %s", g.Url(ci.Issue)))
	}
	return skerr.Wrap(err)
}
