// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/kr/pretty"
	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/maruel/subcommands"
)

var cmdRequestShow = &subcommands.Command{
	UsageLine: "request-show <task_id>",
	ShortDesc: "returns properties of a request",
	LongDesc:  "Returns the properties, what, when, by who, about a request on the Swarming server.",
	CommandRun: func() subcommands.CommandRun {
		r := &requestShowRun{}
		r.Init()
		return r
	},
}

type requestShowRun struct {
	commonFlags

	// Used to authenticate requests to server.
	authFlags      authcli.Flags
	parsedAuthOpts auth.Options
}

func (c *requestShowRun) Init() {
	c.commonFlags.Init()
	c.authFlags.Register(&c.Flags, auth.Options{
		Method: auth.UserCredentialsMethod, // disable GCE service account for now
	})
}

func (c *requestShowRun) createAuthClient() (*http.Client, error) {
	ctx := gologger.StdConfig.Use(context.Background())
	return auth.NewAuthenticator(ctx, auth.OptionalLogin, c.parsedAuthOpts).Client()
}

func (c *requestShowRun) Parse(a subcommands.Application, args []string) error {
	if err := c.commonFlags.Parse(); err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New("must only provide a task id")
	}
	return nil
}

func (c *requestShowRun) main(a subcommands.Application, taskid string) error {
	client, err := c.createAuthClient()
	if err != nil {
		return err
	}

	s, err := swarming.New(client)
	if err != nil {
		return err
	}
	s.BasePath = c.commonFlags.serverURL + "/api/swarming/v1/"

	call := s.Task.Request(taskid)
	result, err := call.Do()

	pretty.Println(result)

	return err
}

func (c *requestShowRun) Run(a subcommands.Application, args []string) int {
	if err := c.Parse(a, args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	cl, err := c.defaultFlags.StartTracing()
	if err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	defer cl.Close()
	if err := c.main(a, args[0]); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
