// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package workflow

import (
	_ "embed"
	"fmt"

	"github.com/urfave/cli/v2"
)

//go:embed docs/index.md
var workflowIndex string

//go:embed docs/task_failure_analysis.md
var workflowTaskFailureAnalysis string

//go:embed docs/task_drilldown.md
var workflowTaskDrilldown string

type workflow struct {
	name        string
	description string
	content     string
	aliases     []string
}

var workflows = []workflow{
	{
		name:        "index",
		description: "Print top-level information about all workflows.",
		content:     workflowIndex,
		aliases:     []string{""},
	},
	{
		name:        "task_failure_analysis",
		description: "Analyze task failures to find culprit commits",
		content:     workflowTaskFailureAnalysis,
	},
	{
		name:        "task_drilldown",
		description: "Deeply investigate a failing task to determine its root cause.",
		content:     workflowTaskDrilldown,
	},
}

func Command() *cli.Command {
	cmd := &cli.Command{
		Name:  "workflow",
		Usage: "Print instructions for high-level workflows.",
	}
	for _, wf := range workflows {
		cmd.Subcommands = append(cmd.Subcommands, makeWorkflowCommand(wf))
	}
	return cmd
}

func makeWorkflowCommand(wf workflow) *cli.Command {
	return &cli.Command{
		Name:    wf.name,
		Usage:   wf.description,
		Aliases: wf.aliases,
		Action: func(c *cli.Context) error {
			fmt.Println(wf.content)
			return nil
		},
	}
}
