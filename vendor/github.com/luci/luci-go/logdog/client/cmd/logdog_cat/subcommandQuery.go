// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"

	"github.com/luci/luci-go/common/clock/clockflag"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/client/coordinator"
	"github.com/maruel/subcommands"
)

const (
	// defaultQueryResults is the default number of query results to return.
	defaultQueryResults = 200
)

type queryCommandRun struct {
	subcommands.CommandRunBase

	path        string
	contentType string
	tags        streamproto.TagMap
	results     int
	before      clockflag.Time
	after       clockflag.Time
	purged      trinaryValue

	json bool
	out  string
}

func newQueryCommand() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "query",
		ShortDesc: "Query for log streams.",
		CommandRun: func() subcommands.CommandRun {
			cmd := &queryCommandRun{}

			fs := cmd.GetFlags()
			fs.StringVar(&cmd.path, "path", "", "Filter logs matching this path (may include globbing).")
			fs.StringVar(&cmd.contentType, "contentType", "", "Limit results to a content type.")
			fs.Var(&cmd.tags, "tag", "Filter logs containing this tag (key[=value]).")
			fs.Var(&cmd.purged, "purged", "Include purged streams in the result. This requires administrative privileges.")
			fs.Var(&cmd.before, "before", "Limit the query to streams registered at or before this RFC3339 time.")
			fs.Var(&cmd.after, "after", "Limit the query to streams registered at or after this RFC3339 time.")
			fs.IntVar(&cmd.results, "results", defaultQueryResults,
				"The maximum number of results to return. If 0, no limit will be applied.")
			fs.BoolVar(&cmd.json, "json", false, "Output JSON state instead of log stream names.")
			fs.StringVar(&cmd.out, "out", "-", "Path to query result output. Use '-' for STDOUT (default).")

			return cmd
		},
	}
}

func (cmd *queryCommandRun) Run(scApp subcommands.Application, args []string) int {
	a := scApp.(*application)

	// User-friendly: trim any leading or trailing slashes from the path.
	project, path, unified, err := a.splitPath(cmd.path)
	if err != nil {
		log.WithError(err).Errorf(a, "Invalid path specifier.")
		return 1
	}

	// Open our output file, if necessary.
	w := io.Writer(nil)
	switch cmd.out {
	case "-":
		w = os.Stdout
	default:
		f, err := os.OpenFile(cmd.out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0643)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"path":       cmd.out,
			}.Errorf(a, "Failed to open output file for writing.")
			return 1
		}
		defer f.Close()
		w = f
	}

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	o := queryOutput(nil)
	if cmd.json {
		o = &jsonQueryOutput{
			Writer: bw,
		}
	} else {
		o = &pathQueryOutput{
			Writer:  bw,
			unified: unified,
		}
	}

	qo := coordinator.QueryOptions{
		ContentType: cmd.contentType,
		State:       cmd.json,
		Before:      cmd.before.Time(),
		After:       cmd.after.Time(),
		Purged:      cmd.purged.Trinary(),
	}
	count := 0
	log.Debugf(a, "Issuing query...")

	ierr := error(nil)
	err = a.coord.Query(a, project, path, qo, func(s *coordinator.LogStream) bool {
		if err := o.emit(s); err != nil {
			ierr = err
			return false
		}

		count++
		return !(cmd.results > 0 && count >= cmd.results)
	})
	if err == nil {
		// Propagate internal error.
		err = ierr
	}
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"count":      count,
		}.Errorf(a, "Query failed.")
		return 1
	}
	log.Fields{
		"count": count,
	}.Infof(a, "Query sequence completed.")

	// (Terminate output stream)
	if err := o.end(); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(a, "Failed to end output stream.")
	}

	return 0
}

type queryOutput interface {
	emit(*coordinator.LogStream) error
	end() error
}

// pathQueryOutput outputs query results as a list of stream path names.
type pathQueryOutput struct {
	*bufio.Writer

	unified bool
}

func (p *pathQueryOutput) emit(s *coordinator.LogStream) error {
	path := string(s.Path)
	if p.unified {
		path = makeUnifiedPath(s.Project, s.Path)
	}

	if _, err := p.WriteString(path); err != nil {
		return err
	}
	if _, err := p.WriteRune('\n'); err != nil {
		return err
	}
	if err := p.Flush(); err != nil {
		return err
	}
	return nil
}

func (p *pathQueryOutput) end() error { return nil }

// We will emit a JSON list of results. To get streaming JSON, we will
// manually construct the outer list and then use the JOSN library to build
// each internal element.
type jsonQueryOutput struct {
	*bufio.Writer

	enc   *json.Encoder
	count int
}

func (p *jsonQueryOutput) emit(s *coordinator.LogStream) error {
	if err := p.ensureStart(); err != nil {
		return err
	}

	if p.count > 0 {
		// Emit comma from previous element.
		_, err := p.WriteRune(',')
		if err != nil {
			return err
		}
	}
	p.count++

	o := struct {
		Project    string                     `json:"project"`
		Path       string                     `json:"path"`
		Descriptor *logpb.LogStreamDescriptor `json:"descriptor,omitempty"`

		Created          clockflag.Time `json:"created,omitempty"`
		Updated          clockflag.Time `json:"updated,omitempty"`
		TerminalIndex    int64          `json:"terminalIndex"`
		ArchiveIndexURL  string         `json:"archiveIndexUrl,omitempty"`
		ArchiveStreamURL string         `json:"archiveStreamUrl,omitempty"`
		ArchiveDataURL   string         `json:"archiveDataUrl,omitempty"`
		Purged           bool           `json:"purged,omitempty"`
	}{
		Project: string(s.Project),
		Path:    string(s.Path),
	}
	if s.State != nil {
		o.Created = clockflag.Time(s.State.Created)
		o.Updated = clockflag.Time(s.State.Updated)
		o.TerminalIndex = int64(s.State.TerminalIndex)
		o.ArchiveIndexURL = s.State.ArchiveIndexURL
		o.ArchiveStreamURL = s.State.ArchiveStreamURL
		o.ArchiveDataURL = s.State.ArchiveDataURL
		o.Purged = s.State.Purged
		o.Descriptor = s.Desc
	}

	if p.enc == nil {
		p.enc = json.NewEncoder(p)
	}
	if err := p.enc.Encode(&o); err != nil {
		return err
	}

	return p.Flush()
}

func (p *jsonQueryOutput) ensureStart() error {
	if p.count > 0 {
		return nil
	}
	_, err := p.WriteString("[\n")
	return err
}

func (p *jsonQueryOutput) end() error {
	if err := p.ensureStart(); err != nil {
		return err
	}

	_, err := p.WriteRune(']')
	return err
}
