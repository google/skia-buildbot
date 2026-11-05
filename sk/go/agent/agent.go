// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package agent

import (
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/sk/go/agent/codereview"
	"go.skia.org/infra/sk/go/agent/tool"
	"go.skia.org/infra/sk/go/agent/workflow"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "agent",
		Usage: "Commands intended for use by AI agents",
		Subcommands: []*cli.Command{
			codereview.Command(),
			tool.Command(),
			workflow.Command(),
		},
	}
}
