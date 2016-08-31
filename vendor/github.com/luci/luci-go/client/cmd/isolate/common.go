// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/client/authcli"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/client/isolate"
	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/logging/gologger"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

type commonFlags struct {
	subcommands.CommandRunBase
	defaultFlags common.Flags
}

func (c *commonFlags) Init() {
	c.defaultFlags.Init(&c.Flags)
}

func (c *commonFlags) Parse() error {
	return c.defaultFlags.Parse()
}

type commonServerFlags struct {
	commonFlags
	isolatedFlags isolatedclient.Flags
	authFlags     authcli.Flags

	parsedAuthOpts auth.Options
}

func (c *commonServerFlags) Init() {
	c.commonFlags.Init()
	c.isolatedFlags.Init(&c.Flags)
	c.authFlags.Register(&c.Flags, auth.Options{
		Method: auth.UserCredentialsMethod, // disable GCE service account for now
	})
}

func (c *commonServerFlags) Parse() error {
	var err error
	if err = c.commonFlags.Parse(); err != nil {
		return err
	}
	if err = c.isolatedFlags.Parse(); err != nil {
		return err
	}
	c.parsedAuthOpts, err = c.authFlags.Options()
	return err
}

func (c *commonServerFlags) createAuthClient() (*http.Client, error) {
	// OptionalLogin is used here instead of SilentLogin to make IP whitelisted
	// bots to work without OAuth for now.
	ctx := gologger.StdConfig.Use(context.Background())
	return auth.NewAuthenticator(ctx, auth.OptionalLogin, c.parsedAuthOpts).Client()
}

type isolateFlags struct {
	// TODO(tandrii): move ArchiveOptions from isolate pkg to here.
	isolate.ArchiveOptions
}

func (c *isolateFlags) Init(f *flag.FlagSet) {
	c.ArchiveOptions.Init()
	f.StringVar(&c.Isolate, "isolate", "", ".isolate file to load the dependency data from")
	f.StringVar(&c.Isolate, "i", "", "Alias for --isolate")
	f.StringVar(&c.Isolated, "isolated", "", ".isolated file to generate or read")
	f.StringVar(&c.Isolated, "s", "", "Alias for --isolated")
	f.Var(&c.Blacklist, "blacklist", "List of regexp to use as blacklist filter when uploading directories")
	f.Var(&c.ConfigVariables, "config-variable", "Config variables are used to determine which conditions should be matched when loading a .isolate file, default: [].")
	f.Var(&c.PathVariables, "path-variable", "Path variables are used to replace file paths when loading a .isolate file, default: {}")
	f.Var(&c.ExtraVariables, "extra-variable", "Extraneous variables are replaced on the command entry and on paths in the .isolate file but are not considered relative paths.")
}

// RequiredIsolateFlags specifies which flags are required on the command line
// being parsed.
type RequiredIsolateFlags uint

const (
	// RequireIsolateFile means the --isolate flag is required.
	RequireIsolateFile RequiredIsolateFlags = 1 << iota
	// RequireIsolatedFile means the --isolated flag is required.
	RequireIsolatedFile
)

func (c *isolateFlags) Parse(cwd string, flags RequiredIsolateFlags) error {
	if !filepath.IsAbs(cwd) {
		return errors.New("cwd must be absolute path")
	}
	for _, vars := range [](map[string]string){c.ConfigVariables, c.ExtraVariables, c.PathVariables} {
		for k := range vars {
			if !isolate.IsValidVariable(k) {
				return fmt.Errorf("invalid key %s", k)
			}
		}
	}

	if c.Isolate == "" {
		if flags&RequireIsolateFile != 0 {
			return errors.New("-isolate must be specified")
		}
	} else {
		if !filepath.IsAbs(c.Isolate) {
			c.Isolate = filepath.Clean(filepath.Join(cwd, c.Isolate))
		}
	}

	if c.Isolated == "" {
		if flags&RequireIsolatedFile != 0 {
			return errors.New("-isolated must be specified")
		}
	} else {
		if !filepath.IsAbs(c.Isolated) {
			c.Isolated = filepath.Clean(filepath.Join(cwd, c.Isolated))
		}
	}
	return nil
}
