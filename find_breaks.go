package main

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/status/go/find_breaks"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

// flags
var (
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
)

// Find groups of failing tasks which constitute breakages.

// Start by merging tasks into contiguous slices of successes and failures
// within a column (task spec). Note that we're assuming all failures
// within a column have the same cause. A more intelligent system would
// be able to distinguish *what* failed and only merge the failures if
// they matched.

// Then merge columns into breakages. For each column:
// - Find the slice of commits which may have caused the break.
// - Find the slice of commits which may have fixed the break.
//
// Two columns may be merged if:
// - The intersection of their break slices is non-empty.
// - The intersection of their fix slices is non-empty.
// - The intersection of the break slice of one column and the fix slice of
//   the other is empty.
//
// However, in merging columns we may reduce the size of the break and
// fix slices. We want this behavior in some sense, because the goal is to
// narrow down the blame to a single commit. But we need to make sure
// that we don't include a failure which then causes us to exclude
// others. For example:
//
//           col1  col2  col3
//            S     S     S
//            F     S     F
//            S     F     F
//            S     S     S
//
// In this case, we can merge col1 with col3 or col2 with col3 but not
// both. We need to make sure that the algorithm gives us both slices:
// [col1, col3] and [col2, col3].
//
// Edge cases. A failure may extend up to the most recent task, or back
// to before our time window, such that we can't actually determine what
// the break or fix slice should be. We can handle this by making the
// break or fix slice empty (or contain a single special value) and let it
// behave as a wildcard which matches anything.
func main() {
	common.Init()

	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	wd, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	reposDir := path.Join(wd, "repos")
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		sklog.Fatal(err)
	}
	repo, err := repograph.NewGraph(common.REPO_SKIA, reposDir)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("Checkout complete")

	end := time.Now()
	start := end.Add(-24 * time.Hour)
	g, err := find_breaks.FindFailureGroups(repo, taskDb, start, end)
	if err != nil {
		sklog.Fatal(err)
	}
	for _, group := range g {
		sklog.Errorf("Failure group:")
		sklog.Errorf("IDs:")
		for _, id := range group.Ids {
			sklog.Errorf("\t%s", id)
		}
		sklog.Errorf("Broke in:")
		for _, c := range group.BrokeIn {
			sklog.Errorf("\t%s", c)
		}
		sklog.Errorf("Failing: ")
		for _, c := range group.Failing {
			sklog.Errorf("\t%s", c)
		}
		sklog.Errorf("Fixed in:")
		for _, c := range group.FixedIn {
			sklog.Errorf("\t%s", c)
		}
		sklog.Errorf("")
	}
}
