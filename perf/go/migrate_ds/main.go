// migrate_ds is a command line tool for migrating Perf data from MySQL to Cloud Datastore.
package main

import (
	"flag"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/activitylog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	idb "go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/ds"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut2"
)

// flags
var (
	gitRepoDir = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	namespace  = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'perf'.")
)

func main() {
	dbConf := idb.DBConfigFromFlags()
	common.Init()
	if *namespace == "" {
		sklog.Fatal("The --namespace flag must be specified.\n")
	}

	if err := ds.Init("google.com:skia-buildbots", *namespace); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			sklog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		sklog.Fatal(err)
	}

	// Migrate alerts.
	alerts.Init(false)
	alertStore := alerts.NewStore()
	allAlerts, err := alertStore.List(true)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Migrating %d alerts.", len(allAlerts))
	alerts.Init(true)
	for _, cfg := range allAlerts {
		if err := alertStore.Save(cfg); err != nil {
			sklog.Fatalf("Failed to write %#v: %s", *cfg, err)
		}
	}

	// Migrate shortcuts.
	shortcut2.Init(false)
	shortcuts, err := shortcut2.List()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Migrating %d shortcuts.", len(shortcuts))
	shortcut2.Init(true)
	for id, sh := range shortcuts {
		if err := shortcut2.Write(id, sh); err != nil {
			sklog.Fatal(err)
		}
	}

	// Migrate regressions.
	regression.Init(false)
	regStore := regression.NewStore()
	now := time.Now()
	begin := now.Add(-time.Hour * 24 * 365).Unix()
	end := now.Add(time.Hour).Unix()
	regs, err := regStore.Range(begin, end, regression.ALL_SUBSET)
	if err != nil {
		sklog.Fatal(err)
	}
	git, err := gitinfo.CloneOrUpdate(*gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}
	cidl := cid.New(git, nil, *gitRepoURL)
	lookup := func(c *cid.CommitID) (*cid.CommitDetail, error) {
		details, err := cidl.Lookup([]*cid.CommitID{c})
		return details[0], err
	}

	sklog.Infof("Migrating %d regressions.", len(regs))
	regression.Init(true)
	err = regStore.Write(regs, lookup)
	if err != nil {
		sklog.Fatal(err)
	}

	// Migrate activity log.
	activitylog.Init(false)
	ac, err := activitylog.GetRecent(1000)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Migrating %d activities.", len(ac))
	activitylog.Init(true)
	for _, a := range ac {
		if err := activitylog.Write(a); err != nil {
			sklog.Fatal(err)
		}
	}
}
