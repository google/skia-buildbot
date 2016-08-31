// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Command authutil can be used to interact with OAuth2 token cache on disk.
package main

import (
	"os"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/logging/gologger"
)

func main() {
	application := &cli.Application{
		Name:  "authutil",
		Title: "LUCI Authentication Utility",
		Context: func(ctx context.Context) context.Context {
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			authcli.SubcommandInfoWithParams(authcli.CommandParams{
				Name:       "info",
				ScopesFlag: true,
			}),
			authcli.SubcommandLoginWithParams(authcli.CommandParams{
				Name:       "login",
				ScopesFlag: true,
			}),
			authcli.SubcommandLogoutWithParams(authcli.CommandParams{
				Name:       "logout",
				ScopesFlag: true,
			}),
			authcli.SubcommandTokenWithParams(authcli.CommandParams{
				Name:       "token",
				ScopesFlag: true,
			}),
		},
	}
	os.Exit(subcommands.Run(application, nil))
}
