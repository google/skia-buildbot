// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"log"
	"os"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/auth"
	"github.com/maruel/subcommands"
)

// version must be updated whenever functional change (behavior, arguments,
// supported commands) is done.
const version = "0.2"

var opts = auth.Options{}

var application = &subcommands.DefaultApplication{
	Name:  "isolated",
	Title: "isolateserver.py but faster",
	// Keep in alphabetical order of their name.
	Commands: []*subcommands.Command{
		cmdArchive,
		cmdDownload,
		subcommands.CmdHelp,
		authcli.SubcommandInfo(opts, "info"),
		authcli.SubcommandLogin(opts, "login"),
		authcli.SubcommandLogout(opts, "logout"),
		common.CmdVersion(version),
	},
}

func main() {
	log.SetFlags(log.Lmicroseconds)
	os.Exit(subcommands.Run(application, nil))
}
