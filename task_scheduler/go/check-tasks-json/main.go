package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/task_scheduler/go/orphaned_tasks_machines"
	"go.skia.org/infra/task_scheduler/go/specs"
	"golang.org/x/oauth2/google"
)

var (
	swarmingServer = flag.String("swarming", "chromium-swarm.appspot.com", "Swarming server to use.")
	tasksJSONFile  = flag.String("tasks-json", "", "tasks.json file to check. If not provided, attempts to find it in the current working directory.")
)

func main() {
	ctx := context.Background()
	flag.Parse()
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		sklog.Fatal(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	swarm := swarmingv2.NewDefaultClient(c, *swarmingServer)

	var tasksCfg *specs.TasksCfg
	if *tasksJSONFile != "" {
		contents, err := os.ReadFile(*tasksJSONFile)
		if err != nil {
			sklog.Fatal(err)
		}
		tasksCfg, err = specs.ParseTasksCfg(string(contents))
	} else {
		checkoutRoot, err := specs.GetCheckoutRoot()
		if err != nil {
			sklog.Fatal(err)
		}
		tasksCfg, err = specs.ReadTasksCfg(checkoutRoot)
	}
	if err != nil {
		sklog.Fatal(err)
	}

	report, err := orphaned_tasks_machines.GenerateReport(ctx, tasksCfg, swarm)
	if err != nil {
		sklog.Fatal(err)
	}
	if len(report.NoMatchingMachines) > 0 {
		fmt.Println("## The following tasks have no matching machines:")
		for _, g := range report.NoMatchingMachines {
			fmt.Println(printGroup(g))
		}
	}
	if len(report.NoMatchingTasks) > 0 {
		fmt.Println("## The following machines have no matching tasks:")
		for _, g := range report.NoMatchingTasks {
			fmt.Println(printGroup(g))
		}
	}
}

func printList(w io.Writer, name string, items []string) {
	_, _ = fmt.Fprintf(w, "%s:\n", name)
	for _, item := range items {
		_, _ = fmt.Fprintf(w, "  - %s\n", item)
	}
}

func printGroup(g *orphaned_tasks_machines.Group) string {
	var sb strings.Builder
	_, _ = fmt.Fprintln(&sb, "==================================")
	printList(&sb, "Dimensions", g.Dimensions)
	if g.LastTaskID != "" {
		_, _ = fmt.Fprintf(&sb, "Last task: https://%s/task?id=%s", *swarmingServer, g.LastTaskID)
	}
	printList(&sb, "Tasks", g.Tasks)
	_, _ = fmt.Fprintf(&sb, "Swarming search: https://%s/machinelist?f=%s\n", *swarmingServer, strings.Join(g.Dimensions, "&f="))
	printList(&sb, "Machines", g.Machines)
	_, _ = fmt.Fprintln(&sb, "==================================")
	return sb.String()
}
