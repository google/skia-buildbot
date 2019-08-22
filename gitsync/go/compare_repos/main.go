package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/pmezard/go-difflib/difflib"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	aProject        = flag.String("a_project", "skia-public", "Project ID.")
	aInstance       = flag.String("a_instance", "production", "BigTable instance.")
	aTable          = flag.String("a_table", "git-repos", "BigTable table.")
	aRepo           = flag.String("a_repo", "", "Repo URL.")
	bProject        = flag.String("b_project", "skia-public", "Project ID.")
	bInstance       = flag.String("b_instance", "production", "BigTable instance.")
	bTable          = flag.String("b_table", "git-repos", "BigTable table.")
	bRepo           = flag.String("b_repo", "", "Repo URL.")
	exitAtFirstDiff = flag.Bool("exit_at_first_diff", true, "If true, exit at the first diff we find.")
)

// TODO(borenet): Stole this from go/deepequal/deep_equals.go
var spewConfig = spew.ConfigState{
	Indent:                  "  ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	SortKeys:                true,
}

func compare(a, b interface{}, args ...interface{}) error {
	if !deepequal.DeepEqual(a, b) {
		aDump := spewConfig.Sdump(a)
		bDump := spewConfig.Sdump(b)
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(aDump),
			B:        difflib.SplitLines(bDump),
			FromFile: "A",
			ToFile:   "B",
			Context:  2,
		})
		msg := ""
		if len(args) > 0 {
			msg = fmt.Sprintf(args[0].(string), args[1:]...)
		}
		return skerr.Fmt(msg + "\n" + diff)
	}
	return nil
}

func compareBranch(ctx context.Context, a, b gitstore.GitStore, branch string, endIdx int) error {
	// Compare IndexCommits in chunks.
	return util.ChunkIter(endIdx+1, 100, func(start, end int) error {
		aCommits, err := a.RangeN(ctx, start, end, branch)
		if err != nil {
			return err
		}
		bCommits, err := b.RangeN(ctx, start, end, branch)
		if err != nil {
			return err
		}
		return compare(aCommits, bCommits, "Branch %s differs:", branch)
	})
}

func compareRepos(ctx context.Context, a, b gitstore.GitStore) error {
	// Compare branch heads.
	aBranches, err := a.GetBranches(ctx)
	if err != nil {
		return err
	}
	bBranches, err := b.GetBranches(ctx)
	if err != nil {
		return err
	}
	if err := compare(aBranches, bBranches); err != nil {
		if *exitAtFirstDiff {
			return err
		} else {
			sklog.Error(err.Error())
		}
	}

	// Compare IndexCommits on common branches.
	branches := map[string]int{}
	for name, aPtr := range aBranches {
		if bPtr, ok := bBranches[name]; ok {
			if aPtr.Index > bPtr.Index {
				branches[name] = aPtr.Index
			} else {
				branches[name] = bPtr.Index
			}
		}
	}
	for name, idx := range branches {
		if err := compareBranch(ctx, a, b, name, idx); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	common.Init()

	ctx := context.Background()

	// Create the GitStore instances.
	aCfg := &bt_gitstore.BTConfig{
		ProjectID:  *aProject,
		InstanceID: *aInstance,
		TableID:    *aTable,
	}
	a, err := bt_gitstore.New(ctx, aCfg, *aRepo)
	if err != nil {
		sklog.Fatal(err)
	}
	bCfg := &bt_gitstore.BTConfig{
		ProjectID:  *bProject,
		InstanceID: *bInstance,
		TableID:    *bTable,
	}
	b, err := bt_gitstore.New(ctx, bCfg, *bRepo)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := compareRepos(ctx, a, b); err != nil {
		sklog.Error(err)
		os.Exit(1)
	}
}
