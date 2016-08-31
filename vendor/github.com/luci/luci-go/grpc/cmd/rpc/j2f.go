// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/maruel/subcommands"

	"github.com/luci/luci-go/client/flagpb"
)

var cmdJ2F = &subcommands.Command{
	UsageLine: `j2f [flags]`,
	ShortDesc: "converts a message from JSON format to flagpb format.",
	LongDesc: `Converts a message from JSON format to flagpb format.

Example:

	$ echo '{"name": "Lucy"}' | rpc fmt j2f
	-name Lucy

See also f2j subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &j2fRun{}
		c.registerBaseFlags()
		return c
	},
}

type j2fRun struct {
	cmdRun
}

func (r *j2fRun) Run(a subcommands.Application, args []string) int {
	if r.cmd == nil {
		r.cmd = cmdJ2F
	}

	if len(args) != 0 {
		return r.argErr("")
	}

	return r.done(jsonToFlags())
}

// jsonToFlags reads JSON from stdin, parses it to a message and
// prints the message in flagpb format.
func jsonToFlags() error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(os.Stdin); err != nil {
		return err
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		return err
	}

	flags, err := flagpb.MarshalUntyped(msg)
	if err != nil {
		return err
	}
	for i := range flags {
		flags[i] = escapeFlag(flags[i])
	}
	fmt.Println(strings.Join(flags, " "))
	return nil
}

// Escape flags

var (
	quotable               = ` "`
	toEscapeExceptQuotable = "\t\n\r'`"
	toEscape               = toEscapeExceptQuotable + quotable
	escapeReplacer         *strings.Replacer
	escapeReplacerInit     sync.Once
)

func initEscapeReplacer() {
	replacerArgs := make([]string, 0, len(toEscape)*2)
	for _, r := range toEscape {
		replacerArgs = append(replacerArgs, string(r), `\`+string(r))
	}
	escapeReplacer = strings.NewReplacer(replacerArgs...)
}

func escapeFlag(s string) string {
	if strings.ContainsAny(s, toEscapeExceptQuotable) {
		escapeReplacerInit.Do(initEscapeReplacer)
		return escapeReplacer.Replace(s)
	}
	if strings.ContainsAny(s, quotable) {
		return "'" + s + "'"
	}
	return s
}
