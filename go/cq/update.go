package cq

import (
	"context"
	"io"
	"io/ioutil"
	osexec "os/exec"
	"path/filepath"
	"regexp"

	"github.com/bazelbuild/buildtools/build"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// WithUpdateCQConfig reads the given Starlark config file, runs the given
// function to modify it, then writes it back to disk and runs lucicfg to
// generate the proto config files in the given directory. Expects lucicfg to
// be in PATH and "lucicfg auth-login" to have already been run. generatedDir
// must be a descendant of the directory which contains starlarkConfigFile.
func WithUpdateCQConfig(ctx context.Context, starlarkConfigFile, generatedDir string, fn func(*build.File) error) error {
	// Make the generatedDir relative to the directory containing
	// starlarkConfigFile.
	parentDir := filepath.Dir(starlarkConfigFile)
	relConfigFile := filepath.Base(starlarkConfigFile)
	relGenDir, err := filepath.Rel(parentDir, generatedDir)
	if err != nil {
		return skerr.Wrapf(err, "could not make %s relative to %s", generatedDir, parentDir)
	}

	// Check presence and auth status of lucicfg.
	if lucicfg, err := osexec.LookPath("lucicfg"); err != nil || lucicfg == "" {
		return skerr.Fmt("unable to find lucicfg in PATH; do you have depot tools installed?")
	}
	if _, err := exec.RunCwd(ctx, ".", "lucicfg", "auth-info"); err != nil {
		return skerr.Wrapf(err, "please run `lucicfg auth-login`")
	}

	// Read the config file.
	oldCfgBytes, err := ioutil.ReadFile(starlarkConfigFile)
	if err != nil {
		return skerr.Wrapf(err, "failed to read %s", starlarkConfigFile)
	}

	// Update the config.
	newCfgBytes, err := WithUpdateCQConfigBytes(starlarkConfigFile, oldCfgBytes, fn)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Write the new config.
	if err := util.WithWriteFile(starlarkConfigFile, func(w io.Writer) error {
		_, err := w.Write(newCfgBytes)
		return err
	}); err != nil {
		return skerr.Wrapf(err, "failed to write config file")
	}

	// Run lucicfg to generate the proto config files.
	cmd := []string{"lucicfg", "generate", "-validate", "-config-dir", relGenDir, relConfigFile}
	if _, err := exec.RunCwd(ctx, parentDir, cmd...); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// WithUpdateCQConfigBytes parses the given bytes as a Config, runs the given
// function to modify the Config, then returns the updated bytes.
func WithUpdateCQConfigBytes(filename string, oldCfgBytes []byte, fn func(*build.File) error) ([]byte, error) {
	// Parse the Config.
	f, err := build.ParseDefault(filename, oldCfgBytes)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse config file")
	}

	// Run the passed-in func.
	if err := fn(f); err != nil {
		return nil, skerr.Wrapf(err, "config modification failed")
	}

	// Write the new config bytes.
	return build.Format(f), nil
}

// FindAssignExpr finds the AssignExpr with the given identifier.
func FindAssignExpr(callExpr *build.CallExpr, identifier string) (int, *build.AssignExpr, error) {
	for idx, expr := range callExpr.List {
		if assignExpr, ok := expr.(*build.AssignExpr); ok {
			if ident, ok := assignExpr.LHS.(*build.Ident); ok {
				if ident.Name == identifier {
					return idx, assignExpr, nil
				}
			}
		}
	}
	return 0, nil, skerr.Fmt("no AssignExpr found for %q", identifier)
}

// FindExprForBranch finds the CallExpr for the given branch.
func FindExprForBranch(f *build.File, branch string) (int, *build.CallExpr, error) {
	for idx, expr := range f.Stmt {
		if callExpr, ok := expr.(*build.CallExpr); ok {
			_, assignExpr, err := FindAssignExpr(callExpr, "name")
			if err != nil {
				continue
			}
			if stringExpr, ok := assignExpr.RHS.(*build.StringExpr); ok {
				if stringExpr.Value == branch {
					return idx, callExpr, nil
				}
			}
		}
	}
	return 0, nil, skerr.Fmt("no config group found for %q", branch)
}

// CopyExprSlice returns a shallow copy of the []Expr.
func CopyExprSlice(slice []build.Expr) []build.Expr {
	cp := make([]build.Expr, 0, len(slice))
	for _, expr := range slice {
		cp = append(cp, expr)
	}
	return cp
}

// CloneBranch updates the given CQ config to create a config for a new
// branch based on a given existing branch. Optionally, include experimental
// tryjobs, include the tree-is-open check, and exclude trybots matching regular
// expressions.
func CloneBranch(f *build.File, oldBranch, newBranch string, includeExperimental, includeTreeCheck bool, excludeTrybotRegexp []*regexp.Regexp) error {
	// Find the CQ config for the old branch.
	_, oldBranchExpr, err := FindExprForBranch(f, oldBranch)
	if err != nil {
		return err
	}

	// Copy the old branch and modify it for the new branch.
	newBranchExpr, ok := oldBranchExpr.Copy().(*build.CallExpr)
	if !ok {
		return skerr.Fmt("expected CallExpr for cq_group")
	}
	newBranchExpr.List = CopyExprSlice(oldBranchExpr.List)

	// CQ group name.
	nameExprIndex, nameExpr, err := FindAssignExpr(newBranchExpr, "name")
	if err != nil {
		return skerr.Wrap(err)
	}
	nameExprCopy := nameExpr.Copy().(*build.AssignExpr)
	nameStr, ok := nameExprCopy.RHS.Copy().(*build.StringExpr)
	if !ok {
		return skerr.Fmt("expected StringExpr for name")
	}
	nameStr.Value = newBranch
	nameExprCopy.RHS = nameStr
	newBranchExpr.List[nameExprIndex] = nameExprCopy

	// Ref match.
	refsetExprIndex, refsetExpr, err := FindAssignExpr(newBranchExpr, "watch")
	if err != nil {
		return skerr.Wrap(err)
	}
	refsetExprCopy := refsetExpr.Copy().(*build.AssignExpr)
	newBranchExpr.List[refsetExprIndex] = refsetExprCopy

	refsetCallExpr, ok := refsetExprCopy.RHS.(*build.CallExpr)
	if !ok {
		return skerr.Fmt("expected CallExpr for refset")
	}
	refsetCallExprCopy := refsetCallExpr.Copy().(*build.CallExpr)
	refsetExprCopy.RHS = refsetCallExprCopy
	refsetCallExprCopy.List = CopyExprSlice(refsetCallExprCopy.List)

	refsExprIndex, refsExpr, err := FindAssignExpr(refsetCallExprCopy, "refs")
	if err != nil {
		return skerr.Wrap(err)
	}
	refsExprCopy := refsExpr.Copy().(*build.AssignExpr)
	refsetCallExprCopy.List[refsExprIndex] = refsExprCopy

	refsListExpr, ok := refsExprCopy.RHS.(*build.ListExpr)
	if !ok {
		return skerr.Fmt("expected ListExpr for refs")
	}
	refsListExprCopy := refsListExpr.Copy().(*build.ListExpr)
	refsExprCopy.RHS = refsListExprCopy

	if len(refsListExprCopy.List) != 1 {
		return skerr.Fmt("expected a single ref but got %d", len(refsListExprCopy.List))
	}
	refExpr, ok := refsListExprCopy.List[0].(*build.StringExpr)
	if !ok {
		return skerr.Fmt("expected StringExpr for ref")
	}
	refExprCopy := refExpr.Copy().(*build.StringExpr)
	refExprCopy.Value = "refs/heads/" + newBranch
	refsListExprCopy.List = []build.Expr{refExprCopy}

	// Tryjobs.
	verifiersExprIndex, verifiersExpr, err := FindAssignExpr(newBranchExpr, "verifiers")
	if err != nil {
		return skerr.Wrap(err)
	}
	verifiersExprCopy := verifiersExpr.Copy().(*build.AssignExpr)
	newBranchExpr.List[verifiersExprIndex] = verifiersExprCopy

	verifiersListExpr, ok := verifiersExprCopy.RHS.(*build.ListExpr)
	if !ok {
		return skerr.Fmt("expected ListExpr for verifiers")
	}
	verifiersListExprCopy := verifiersListExpr.Copy().(*build.ListExpr)
	verifiersExprCopy.RHS = verifiersListExprCopy

	verifiersListExprCopy.List = make([]build.Expr, 0, len(verifiersListExpr.List))
	for _, expr := range verifiersListExpr.List {
		verifierCallExpr, ok := expr.(*build.CallExpr)
		if !ok {
			return skerr.Fmt("expected CallExpr for verifier")
		}
		// Include experimental builders?
		_, _, err := FindAssignExpr(verifierCallExpr, "experiment_percentage")
		if err == nil && !includeExperimental {
			continue
		}

		// Is this builder excluded based on a regex?
		_, builder, err := FindAssignExpr(verifierCallExpr, "builder")
		if err != nil {
			return skerr.Wrap(err)
		}
		builderStringExpr, ok := builder.RHS.(*build.StringExpr)
		if !ok {
			return skerr.Fmt("expected StringExpr for builder name")
		}
		include := true
		for _, regex := range excludeTrybotRegexp {
			if regex.MatchString(builderStringExpr.Value) {
				include = false
				break
			}
		}
		if include {
			// No need to copy, since we're not modifying the verifier
			// expression itself.
			verifiersListExprCopy.List = append(verifiersListExprCopy.List, verifierCallExpr)
		}
	}

	// Tree status.
	if !includeTreeCheck {
		treeCheckIndex, _, err := FindAssignExpr(newBranchExpr, "tree_status_host")
		if err == nil {
			cp := make([]build.Expr, 0, len(newBranchExpr.List))
			for idx, expr := range newBranchExpr.List {
				if idx != treeCheckIndex {
					cp = append(cp, expr)
				}
			}
			newBranchExpr.List = cp
		}
	}

	// Add the new branch config.
	f.Stmt = append(f.Stmt, newBranchExpr)
	return nil
}

// DeleteBranch updates the given CQ config to delete the config matching the
// given branch.
func DeleteBranch(f *build.File, branch string) error {
	branchExprIndex, _, err := FindExprForBranch(f, branch)
	if err != nil {
		return skerr.Wrap(err)
	}
	newStmt := make([]build.Expr, 0, len(f.Stmt)-1)
	for idx, expr := range f.Stmt {
		if idx != branchExprIndex {
			newStmt = append(newStmt, expr)
		}
	}
	f.Stmt = newStmt
	return nil
}
