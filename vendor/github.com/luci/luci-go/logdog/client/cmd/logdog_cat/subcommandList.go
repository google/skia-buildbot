// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/client/coordinator"
	"github.com/luci/luci-go/logdog/common/types"
	"github.com/maruel/subcommands"
)

const (
	// defaultListResults is the default number of list results to return.
	defaultListResults = 200
)

type listCommandRun struct {
	subcommands.CommandRunBase

	o coordinator.ListOptions
}

func newListCommand() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "ls",
		ShortDesc: "List log stream hierarchy space.",
		CommandRun: func() subcommands.CommandRun {
			cmd := &listCommandRun{}

			fs := cmd.GetFlags()
			fs.BoolVar(&cmd.o.State, "l", false, "Perform long listing, showing stream details.")
			fs.BoolVar(&cmd.o.StreamsOnly, "streams", false, "Only list streams, not intermediate components.")
			fs.BoolVar(&cmd.o.Purged, "purged", false, "Include purged streams in listing (admin-only).")
			return cmd
		},
	}
}

func (cmd *listCommandRun) Run(scApp subcommands.Application, args []string) int {
	a := scApp.(*application)

	if len(args) == 0 {
		args = []string{""}
	}

	bio := bufio.NewWriter(os.Stdout)
	defer bio.Flush()

	for _, arg := range args {
		arg = strings.TrimSpace(arg)

		var (
			project  config.ProjectName
			pathBase string
			unified  bool
		)
		if len(arg) > 0 {
			// User-friendly: trim any leading or trailing slashes from the path.
			var err error
			project, pathBase, unified, err = a.splitPath(string(types.StreamPath(arg).Trim()))
			if err != nil {
				log.WithError(err).Errorf(a, "Invalid path specifier.")
				return 1
			}
		}

		err := a.coord.List(a, project, pathBase, cmd.o, func(lr *coordinator.ListResult) bool {
			p := lr.Name
			if cmd.o.State {
				// Long listing, show full path.
				fp := lr.FullPath()
				if unified {
					p = makeUnifiedPath(lr.Project, fp)
				} else {
					p = string(fp)
				}
			}

			if lr.State == nil {
				fmt.Fprintf(bio, "%s\n", p)
			} else {
				fmt.Fprintf(bio, "%s\t[%s]\n", p, cmd.attributes(lr))
			}
			return true
		})
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"pathBase":   pathBase,
				"project":    project,
			}.Errorf(a, "Failed to list path base.")
			return 1
		}
	}
	return 0
}

func (*listCommandRun) attributes(lr *coordinator.ListResult) string {
	var parts []string

	if s := lr.State; s != nil {
		if d := s.Desc; d != nil {
			p := []string{
				d.GetTimestamp().Time().String(),
				fmt.Sprintf("type:%s", d.StreamType.String()),
			}

			if t := d.GetTags(); len(t) > 0 {
				tstr := make([]string, 0, len(t))
				for k, v := range t {
					if v == "" {
						tstr = append(tstr, k)
					} else {
						tstr = append(tstr, fmt.Sprintf("%s=%s", k, v))
					}
				}
				sort.Strings(tstr)
				p = append(p, fmt.Sprintf("tags:{%s}", strings.Join(tstr, ", ")))
			}

			parts = append(parts, p...)
		}

		if st := s.State; st != nil {
			if st.TerminalIndex >= 0 {
				parts = append(parts, fmt.Sprintf("terminal:%d", st.TerminalIndex))
			} else {
				parts = append(parts, "streaming")
			}

			if st.Archived {
				parts = append(parts, "archived")
			}

			if st.Purged {
				parts = append(parts, "purged")
			}
		}
	}

	return strings.Join(parts, " ")
}
