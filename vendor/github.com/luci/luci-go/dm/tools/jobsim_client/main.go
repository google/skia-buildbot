// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Jobsim client is a self-contained binary that implements various toy job
// algorithms for use in testing DM with live distributors (like swarming).
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"

	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/data/rand/cryptorand"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/common/retry"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/grpc/prpc"
)

type cmdRun struct {
	subcommands.CommandRunBase

	context.Context

	exAuthPath string
	exAuth     *dm.Execution_Auth

	questDescPath string
	questDesc     *dm.Quest_Desc

	dmHost string

	client dm.DepsClient

	cmd *subcommands.Command
}

func (r *cmdRun) registerOptions() {
	r.Flags.StringVar(&r.exAuthPath, "execution-auth-path", "",
		"The path to a JSONPB encoded dm.Execution.Auth message for this client to"+
			" act as a DM client")
	r.Flags.StringVar(&r.questDescPath, "quest-desc-path", "",
		"The path to a JSONPB encoded dm.Quest.Data.Desc message of how this client"+
			" was invoked")
	r.Flags.StringVar(&r.dmHost, "dm-host", "", "The hostname of the DM service")
}

func loadJSONPB(c context.Context, flavor, path string, msg proto.Message) {
	if path == "" {
		logging.Errorf(c, "processing %q: required parameter", flavor)
		os.Exit(-1)
	}

	fil, err := os.Open(path)
	if err != nil {
		logging.Errorf(c, "processing %q: %s", flavor, err)
		os.Exit(-1)
	}
	defer fil.Close()
	if err := jsonpb.Unmarshal(fil, msg); err != nil {
		panic(err)
	}
}

func (r *cmdRun) start(a subcommands.Application, cr subcommands.CommandRun, c *subcommands.Command) {
	r.cmd = c

	r.Context = cli.GetContext(a, cr)

	r.exAuth = &dm.Execution_Auth{}
	loadJSONPB(r, "execution-auth-path", r.exAuthPath, r.exAuth)

	r.questDesc = &dm.Quest_Desc{}
	loadJSONPB(r, "quest-desc-path", r.questDescPath, r.questDesc)

	// initialize rpc client
	r.client = dm.NewDepsPRPCClient(&prpc.Client{
		Host: r.dmHost,
		Options: &prpc.Options{
			Retry: retry.Default,
		},
	})

	key := make([]byte, 32)
	_, err := cryptorand.Read(r, key)
	if err != nil {
		panic(err)
	}

	req := &dm.ActivateExecutionReq{
		Auth:           r.exAuth,
		ExecutionToken: key,
	}
	err = retry.Retry(r, retry.Default, func() error {
		_, err := r.client.ActivateExecution(r, req)
		return err
	}, retry.LogCallback(r, "ActivateExecution"))
	if err != nil {
		panic(err)
	}

	// activated with key!
	r.exAuth.Token = key
}

func (r *cmdRun) depOn(jobProperties ...interface{}) []string {
	req := &dm.EnsureGraphDataReq{
		ForExecution: r.exAuth,

		Quest:        make([]*dm.Quest_Desc, len(jobProperties)),
		QuestAttempt: make([]*dm.AttemptList_Nums, len(jobProperties)),

		Include: &dm.EnsureGraphDataReq_Include{
			Attempt: &dm.EnsureGraphDataReq_Include_Options{Result: true},
		},
	}

	for i, jProps := range jobProperties {
		data, err := json.Marshal(jProps)
		if err != nil {
			panic(err)
		}
		req.Quest[i] = proto.Clone(r.questDesc).(*dm.Quest_Desc)
		req.Quest[i].Parameters = string(data)
		req.QuestAttempt[i] = &dm.AttemptList_Nums{Nums: []uint32{1}}
	}

	rsp := (*dm.EnsureGraphDataRsp)(nil)
	err := retry.Retry(r, retry.Default, func() (err error) {
		rsp, err = r.client.EnsureGraphData(r, req)
		return
	}, retry.LogCallback(r, "EnsureGraphData"))
	if err != nil {
		panic(err)
	}
	if rsp.ShouldHalt {
		logging.Infof(r, "got ShouldHalt, quitting")
		os.Exit(0)
	}

	ret := make([]string, len(jobProperties))
	for i, qid := range rsp.QuestIds {
		ret[i] = rsp.Result.Quests[qid.Id].Attempts[1].Data.GetFinished().Data.Object
	}

	return ret
}

func (r *cmdRun) finish(data interface{}) {
	encData, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	err = retry.Retry(r, retry.Default, func() error {
		_, err := r.client.FinishAttempt(r, &dm.FinishAttemptReq{
			Auth: r.exAuth, Data: dm.NewJsonResult(string(encData)),
		})
		return err
	}, retry.LogCallback(r, "FinishAttempt"))
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

// argErr prints an err and usage to stderr and returns an exit code.
func (r *cmdRun) argErr(format string, a ...interface{}) int {
	if format != "" {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
	fmt.Fprintln(os.Stderr, r.cmd.ShortDesc)
	fmt.Fprintln(os.Stderr, r.cmd.UsageLine)
	fmt.Fprintln(os.Stderr, "\nFlags:")
	r.Flags.PrintDefaults()
	return -1
}

var application = &cli.Application{
	Name:  "jobsim_client",
	Title: "Executable Job simulator client",
	Context: func(ctx context.Context) context.Context {
		return gologger.StdConfig.Use(ctx)
	},
	Commands: []*subcommands.Command{
		cmdEditDistance,
		subcommands.CmdHelp,
	},
}

func main() {
	os.Exit(subcommands.Run(application, os.Args[1:]))
}
