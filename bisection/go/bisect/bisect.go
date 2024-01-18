package main

import (
	"context"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/bisection/go/read_values"
	"go.skia.org/infra/bisection/go/run_benchmark"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

// executable for local testing and experimentation
func main() {
	ctx := context.Background()

	swarmingClient, err := run_benchmark.DialSwarming(ctx)
	if err != nil {
		sklog.Errorf("swarming client error: %v", err)
	}

	// based off of commit 1224894 in Pinpoint job:
	// https://pinpoint-dot-chromeperf.appspot.com/job/1226ecbef60000
	rbe := swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "projects/chrome-swarming/instances/default_instance",
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "52cfada1f142268bbcb90a827170f9d6f0d46232755f0de83b78cd60763f0aa5",
			SizeBytes: 1294,
		},
	}

	job := "3"
	req := run_benchmark.RunBenchmarkRequest{
		JobID: job,
		Build: &rbe,
	}

	for i := 0; i < 10; i++ {
		taskID, err := run_benchmark.Run(ctx, swarmingClient, req)
		if err != nil {
			fmt.Printf("error: %v", err)
		}
		status, err := run_benchmark.GetStatus(ctx, swarmingClient, taskID)
		fmt.Printf("task %s status: %s", taskID, status)
	}

	// minor time delay is necessary to ensure all tasks are enqueued
	time.Sleep(10 * time.Second)

	tasks, err := run_benchmark.ListPinpointTasks(ctx, swarmingClient, req)
	if err != nil {
		fmt.Printf("list tasks error: %v", err)
	}
	fmt.Printf("\nswarming tasks:%v\n\n", tasks)

	flag := true
	for flag {
		flag = false
		states, err := run_benchmark.GetStates(ctx, swarmingClient, tasks)
		if err != nil {
			sklog.Errorf("failed to retrieve swarming tasks %v\n due to error: %s", tasks, err)
			return
		}
		for i, s := range states {
			if s == swarming.TASK_STATE_PENDING || s == swarming.TASK_STATE_RUNNING {
				fmt.Printf("task [%d]: %s is still %s\n", i, tasks[i], s)
				flag = true
			}
		}
		time.Sleep(10 * time.Second)
	}

	fmt.Printf("\nSwarming tasks completed. Retrieving values.\n")
	casOutputs := make([]*swarmingV1.SwarmingRpcsCASReference, len(tasks))
	states, err := run_benchmark.GetStates(ctx, swarmingClient, tasks)
	for i, t := range tasks {
		if states[i] == "COMPLETED" {
			cas, err := run_benchmark.GetCASOutput(ctx, swarmingClient, t)
			if err != nil {
				fmt.Printf("error retrieving cas outputs: %s", err)
			}
			casOutputs[i] = cas
		}
	}

	rbeClient, err := read_values.DialRBECAS(ctx, rbe.CasInstance)
	if err != nil {
		fmt.Printf("failed to dial rbe client: %s\n", err)
	}
	// based off of Pinpoint job https://pinpoint-dot-chromeperf.appspot.com/job/1226ecbef60000
	values := read_values.ReadValuesByChart(ctx, rbeClient, "blink_perf.bindings", "node-type", casOutputs)
	spew.Dump(values)

	fmt.Printf("\nfinished\n")
}
