// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/flag/stringmapflag"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/maruel/subcommands"
)

var cmdTrigger = &subcommands.Command{
	UsageLine: "trigger hash",
	ShortDesc: "Triggers a Swarming task",
	LongDesc:  "Triggers a Swarming task.",
	CommandRun: func() subcommands.CommandRun {
		r := &triggerRun{}
		r.Init()
		return r
	},
}

type array []*swarming.SwarmingRpcsStringPair

func (a array) Len() int { return len(a) }
func (a array) Less(i, j int) bool {
	return (a[i].Key < a[j].Key) ||
		(a[i].Key == a[j].Key && a[i].Value < a[j].Value)
}
func (a array) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// mapToArray converts a stringmapflag.Value into an array of
// swarming.SwarmingRpcsStringPair, sorted by key and then value.
func mapToArray(m stringmapflag.Value) []*swarming.SwarmingRpcsStringPair {
	a := make([]*swarming.SwarmingRpcsStringPair, 0, len(m))
	for k, v := range m {
		a = append(a, &swarming.SwarmingRpcsStringPair{Key: k, Value: v})
	}

	sort.Sort(array(a))
	return a
}

// namePartFromDimensions creates a string from a map of dimensions that can
// be used as part of the task name.  The dimensions are first sorted as
// described in mapToArray().
func namePartFromDimensions(m stringmapflag.Value) string {
	a := mapToArray(m)
	pairs := make([]string, 0, len(a))
	for i := 0; i < len(a); i++ {
		pairs = append(pairs, fmt.Sprintf("%s=%s", a[i].Key, a[i].Value))
	}
	return strings.Join(pairs, "_")
}

type triggerRun struct {
	commonFlags

	// TODO(rogerta): movce these flags to swarming/common.go once other commands
	// are written and I see what parts are common.

	// Isolate server.
	isolateServer string
	namespace     string

	// Task group.
	isolated    string
	dimensions  stringmapflag.Value
	env         stringmapflag.Value
	priority    int64
	taskName    string
	tags        common.Strings
	user        string
	idempotent  bool
	expiration  int
	deadline    int
	hardTimeout int64
	ioTimeout   int64
	rawCmd      bool
	dumpJSON    string

	// Used to authenticate requests to server.
	authFlags      authcli.Flags
	parsedAuthOpts auth.Options
}

func (c *triggerRun) Init() {
	c.commonFlags.Init()

	// Isolate server.
	c.Flags.StringVar(&c.isolateServer, "isolate-server", "", "URL of the Isolate Server to use.")
	c.Flags.StringVar(&c.namespace, "namespace", "default-zip", "The namespace to use on the Isolate Server.")

	// Task group.
	c.Flags.StringVar(&c.isolated, "isolated", "", "Hash of the .isolated to grab from the isolate server.")
	c.Flags.Var(&c.dimensions, "dimension", "Dimension to filter slaves on.")
	c.Flags.Var(&c.env, "env", "Environment variables to set.")
	c.Flags.Int64Var(&c.priority, "priority", 100, "The lower value, the more important the task.")
	c.Flags.StringVar(&c.taskName, "task-name", "", "Display name of the task. Defaults to <base_name>/<dimensions>/<isolated hash>/<timestamp> if an  isolated file is provided, if a hash is provided, it defaults to <user>/<dimensions>/<isolated hash>/<timestamp>")
	c.Flags.Var(&c.tags, "tag", "Tags to assign to the task.")
	c.Flags.StringVar(&c.user, "user", "", "User associated with the task. Defaults to authenticated user on the server.")
	c.Flags.BoolVar(&c.idempotent, "idempotent", false, "When set, the server will actively try to find a previous task with the same parameter and return this result instead if possible.")
	c.Flags.IntVar(&c.expiration, "expiration", 6*60*60, "Seconds to allow the task to be pending for a bot to run before this task request expires.")
	c.Flags.IntVar(&c.deadline, "deadline", 0, "TODO(rogerta)")
	c.Flags.Int64Var(&c.hardTimeout, "hard-timeout", 60*60, "Seconds to allow the task to complete.")
	c.Flags.Int64Var(&c.ioTimeout, "io-timeout", 20*60, "Seconds to allow the task to be silent.")
	c.Flags.BoolVar(&c.rawCmd, "raw-cmd", false, "When set, the command after -- is used as-is without run_isolated. In this case, no isolated hash is expected.")
	c.Flags.StringVar(&c.dumpJSON, "dump-json", "", "Dump details about the triggered task(s) to this file as json.")

	c.authFlags.Register(&c.Flags, auth.Options{
		Method: auth.UserCredentialsMethod, // disable GCE service account for now
	})
}

func (c *triggerRun) createAuthClient() (*http.Client, error) {
	ctx := gologger.StdConfig.Use(context.Background())
	return auth.NewAuthenticator(ctx, auth.OptionalLogin,
		c.parsedAuthOpts).Client()
}

func (c *triggerRun) Parse(args []string) error {
	var err error
	if err := c.commonFlags.Parse(); err != nil {
		return err
	}
	c.parsedAuthOpts, err = c.authFlags.Options()

	// Validate options and args.
	if c.dimensions == nil {
		return errors.New("please at least specify one dimension")
	}

	if len(c.isolated) == 0 {
		return errors.New("please use -isolated to specify hash")
	}

	if len(c.isolated) != 40 {
		return errors.New("invalid hash")
	}

	if c.rawCmd {
		if len(args) == 0 {
			return errors.New("arguments with -raw-cmd should be passed after -- as command delimiter")
		}

		if len(c.isolateServer) > 0 {
			return errors.New("can't use both -raw-cmd and -isolate-server")
		}
	}

	if len(c.user) == 0 {
		c.user = os.Getenv("USER")
	}

	return err
}

func (c *triggerRun) Run(a subcommands.Application, args []string) int {
	if err := c.Parse(args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	cl, err := c.defaultFlags.StartTracing()
	if err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	defer cl.Close()

	if err := c.main(a, args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *triggerRun) main(a subcommands.Application, args []string) error {
	start := time.Now()
	request, err := c.processTriggerOptions(args)
	if err != nil {
		return err
	}

	result, err := c.createNewTask(request)
	if err != nil {
		return err
	}

	fmt.Printf("Triggered task: %s\n\n", result.TaskId)
	viewURL := fmt.Sprintf("%s/user/task/%s", c.serverURL, result.TaskId)

	if len(c.dumpJSON) > 0 {
		dump, err := os.Create(c.dumpJSON)
		if err != nil {
			return err
		}
		defer dump.Close()

		taskInfo := map[string]interface{}{
			"shard_index": 0,
			"task_id":     result.TaskId,
			"view_url":    viewURL,
		}

		tasks := map[string]interface{}{
			request.Name: taskInfo,
		}

		data := map[string]interface{}{
			"base_task_name": c.taskName,
			"tasks":          tasks,
			"request":        request,
		}

		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return errors.New("could not marshal data")
		}

		_, err = dump.Write(b)
		if err != nil {
			return errors.New("could not dump response to json file")
		}

		if !c.defaultFlags.Quiet {
			fmt.Println("To collect results use:")
			fmt.Printf("  swarming collect -server %s -json %s\n", c.serverURL, c.dumpJSON)
		}
	} else if !c.defaultFlags.Quiet {
		fmt.Println("To collect results use:")
		fmt.Printf("  swarming collect -server %s %s\n", c.serverURL, result.TaskId)
	}

	if !c.defaultFlags.Quiet {
		fmt.Printf("or visit: %s\n", viewURL)
	}

	duration := time.Since(start)
	log.Printf("Duration: %s\n", units.Round(duration, time.Millisecond))
	return err
}

func (c *triggerRun) processTriggerOptions(args []string) (*swarming.SwarmingRpcsNewTaskRequest, error) {
	var inputsRefs *swarming.SwarmingRpcsFilesRef
	var commands []string
	var extraArgs []string

	if c.rawCmd {
		commands = args
		inputsRefs = nil
		if len(c.taskName) == 0 {
			c.taskName = fmt.Sprintf("%s/%s", c.user, namePartFromDimensions(c.dimensions))
		}
	} else {
		separator := -1
		for i, v := range args {
			if v == "--" {
				separator = i
				break
			}
		}

		if len(args) > 0 {
			if separator != -1 {
				commands = args[separator+1:]
			} else {
				commands = args
			}
		}

		if len(c.taskName) == 0 {
			c.taskName = fmt.Sprintf("%s/%s/%s", c.user, namePartFromDimensions(c.dimensions), c.isolated)
		}

		inputsRefs = &swarming.SwarmingRpcsFilesRef{
			Isolated:       c.isolated,
			Isolatedserver: c.isolateServer,
			Namespace:      c.namespace,
		}
		extraArgs = commands
		commands = nil
	}

	properties := swarming.SwarmingRpcsTaskProperties{
		Command:              commands,
		Dimensions:           mapToArray(c.dimensions),
		Env:                  mapToArray(c.env),
		ExecutionTimeoutSecs: c.hardTimeout,
		ExtraArgs:            extraArgs,
		GracePeriodSecs:      30,
		Idempotent:           c.idempotent,
		InputsRef:            inputsRefs,
		IoTimeoutSecs:        c.ioTimeout,
	}

	request := swarming.SwarmingRpcsNewTaskRequest{
		ExpirationSecs: c.hardTimeout,
		Name:           c.taskName,
		ParentTaskId:   os.Getenv("SWARMING_TASK_ID"),
		Priority:       c.priority,
		Properties:     &properties,
		Tags:           c.tags,
		User:           c.user,
	}
	return &request, nil
}

func (c *triggerRun) createNewTask(request *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	client, err := c.createAuthClient()
	if err != nil {
		return &swarming.SwarmingRpcsTaskRequestMetadata{}, err
	}

	s, err := swarming.New(client)
	if err != nil {
		return &swarming.SwarmingRpcsTaskRequestMetadata{}, err
	}
	s.BasePath = c.commonFlags.serverURL + "/api/swarming/v1/"

	call := s.Tasks.New(request).Fields("task_result")
	result, err := call.Do()
	if err != nil {
		return &swarming.SwarmingRpcsTaskRequestMetadata{}, err
	}

	// Recursively look at error and log.

	return result, nil
}
