package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/statusv2/go/failure_parser"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

var (
	// Flags.
	local              = flag.Bool("local", false, "Whether or not we're running locally")
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
)

func manualTest() {
	failures := failure_parser.ParseFailures(`
 USER: chrome-bot

# go.skia.org/infra
./dump_autoroll_state_machine.go:11: undefined: "go.skia.org/infra/autoroll/go/state_machine".DumpGraphviz
./run_unittests.go:305: main redeclared in this block
	previous declaration at ./dump_autoroll_state_machine.go:9
step returned non-zero exit code: 2
`)
	if len(failures) == 0 {
		sklog.Fatal("no failures")
	}
	for i, f := range failures {
		sklog.Infof("#%d", i)
		sklog.Info(f.StrippedMessage)
	}
}

func main() {
	common.Init()

	if *local {
		manualTest()
	} else {
		db, err := remote_db.NewClient(*taskSchedulerDbUrl)
		if err != nil {
			sklog.Fatal(err)
		}

		fp, err := failure_parser.New(db)
		if err != nil {
			sklog.Fatal(err)
		}
		if err := fp.Tick(); err != nil {
			sklog.Fatal(err)
		}
	}
}
