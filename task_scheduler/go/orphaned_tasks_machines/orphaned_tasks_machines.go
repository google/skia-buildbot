package orphaned_tasks_machines

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
)

type Report struct {
	NoMatchingMachines []*Group  `json:"no_matching_machines"`
	NoMatchingTasks    []*Group  `json:"no_matching_tasks"`
	Timestamp          time.Time `json:"timestamp"`
}

type Group struct {
	Tasks      []string `json:"tasks"`
	Machines   []string `json:"machines"`
	Dimensions []string `json:"dimensions"`
	LastTaskID string   `json:"last_task_id"`
}

type Machine struct {
	ID         string
	Dimensions []string
}

func GenerateReport(ctx context.Context, tasksCfg *specs.TasksCfg, swarm swarmingv2.SwarmingV2Client) (*Report, error) {
	// Extract the list of Swarming pools to query and the list of dimension
	// keys used by all tasks.
	pools := util.StringSet{}
	allowedDimKeys := util.StringSet{}
	for taskName, taskSpec := range tasksCfg.Tasks {
		for _, dim := range taskSpec.Dimensions {
			split := strings.SplitN(dim, ":", 2)
			if len(split) != 2 {
				return nil, skerr.Fmt("Task %s has invalid dimension %s; expected `key:value`", taskName, dim)
			}
			allowedDimKeys[split[0]] = true
			if split[0] == "pool" {
				pools[split[1]] = true
			}
		}
	}
	machines, err := loadAllMachines(ctx, swarm, pools.Keys())
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Match machines to tasks.

	// Create a machines-by-swarming-dimension mapping.
	machinesByDim := map[string]util.StringSet{}
	for _, b := range machines {
		for _, dim := range b.Dimensions {
			if _, ok := machinesByDim[dim]; !ok {
				machinesByDim[dim] = util.StringSet{}
			}
			machinesByDim[dim][b.ID] = true
		}
	}

	groups := map[string]*Group{}
	matchedMachine := make(map[string]bool, len(machines))
	for name, taskSpec := range tasksCfg.Tasks {
		sort.Strings(taskSpec.Dimensions)
		key := strings.Join(taskSpec.Dimensions, ";")
		group, ok := groups[key]
		if !ok {
			group = &Group{}
			groups[key] = group
		}
		group.Tasks = append(group.Tasks, name)
		if ok {
			// We already have the dimensions and matching machines for this
			// dimension set.
			continue
		}

		group.Dimensions = taskSpec.Dimensions

		matches := util.StringSet{}
		for i, d := range taskSpec.Dimensions {
			if i == 0 {
				matches = matches.Union(machinesByDim[d])
			} else {
				matches = matches.Intersect(machinesByDim[d])
			}
		}
		group.Machines = matches.Keys()
		sort.Strings(group.Machines)
		for _, machine := range group.Machines {
			matchedMachine[machine] = true
		}
	}

	// Separate the groups which had no matching machines.
	noMatchingMachines := map[string]*Group{}
	for key, group := range groups {
		if len(group.Machines) == 0 {
			noMatchingMachines[key] = group
			delete(groups, key)
			lastTask, err := findLastTask(ctx, swarm, group.Tasks)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if lastTask != nil {
				group.LastTaskID = lastTask.TaskId
			}
		}
	}

	// Find unused machines.
	unusedMachines := map[string][]string{}
	unusedDims := map[string][]string{}
	for _, machine := range machines {
		if matchedMachine[machine.ID] {
			continue
		}
		dimensions := make([]string, 0, len(allowedDimKeys))
		for _, dim := range machine.Dimensions {
			split := strings.SplitN(dim, ":", 2)
			if len(split) != 2 {
				return nil, skerr.Fmt("machine %s has invalid dimension %q; expected `key:value`", machine.ID, dim)
			}
			if !allowedDimKeys[split[0]] {
				continue
			}
			dimensions = append(dimensions, dim)
		}
		sort.Strings(dimensions)
		key := strings.Join(dimensions, ";")
		unusedMachines[key] = append(unusedMachines[key], machine.ID)
		if _, ok := unusedDims[key]; !ok {
			unusedDims[key] = dimensions
		}
	}
	noMatchingTasks := map[string]*Group{}
	for key, machines := range unusedMachines {
		noMatchingTasks[key] = &Group{
			Machines:   machines,
			Dimensions: unusedDims[key],
		}
	}

	report := &Report{
		NoMatchingMachines: make([]*Group, 0, len(noMatchingMachines)),
		NoMatchingTasks:    make([]*Group, 0, len(noMatchingTasks)),
		Timestamp:          time.Now(),
	}
	for _, group := range noMatchingMachines {
		report.NoMatchingMachines = append(report.NoMatchingMachines, group)
	}
	for _, group := range noMatchingTasks {
		report.NoMatchingTasks = append(report.NoMatchingTasks, group)
	}
	return report, nil
}

func loadAllMachines(ctx context.Context, swarm swarmingv2.SwarmingV2Client, pools []string) ([]*Machine, error) {
	var rv []*Machine
	for _, pool := range pools {
		machines, err := swarmingv2.ListBotsForPool(ctx, swarm, pool)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for _, item := range machines {
			rv = append(rv, &Machine{
				ID:         item.BotId,
				Dimensions: swarmingv2.BotDimensionsToStringSlice(item.Dimensions),
			})
		}
	}
	return rv, nil
}

func findLastTask(ctx context.Context, swarm swarmingv2.SwarmingV2Client, taskNames []string) (*apipb.TaskResultResponse, error) {
	var lastTask *apipb.TaskResultResponse
	for _, taskName := range taskNames {
		resp, err := swarm.ListTasks(ctx, &apipb.TasksWithPerfRequest{
			Limit: 1,
			Tags:  []string{fmt.Sprintf("name:%s", taskName)},
		})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for _, task := range resp.Items {
			if lastTask == nil || task.CreatedTs.AsTime().After(lastTask.CreatedTs.AsTime()) {
				lastTask = task
			}
		}
	}
	return lastTask, nil
}
