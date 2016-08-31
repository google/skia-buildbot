// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package authcli implements authentication related CLI subcommands.
//
// It can be used from CLI tools that want customize authentication
// configuration from the command line.
//
// It use luci-go/common/cli.GetContext() to grab a context for logging, so
// callers should prefer using cli.Application for hosting subcommands and
// making the context:
//
//
//	import (
//	  "github.com/luci/luci-go/client/authcli"
//	  "github.com/luci/luci-go/common/cli"
//	)
//
//	var application = &cli.Application{
//		Name:  "app_name",
//
//		Context: func(ctx context.Context) context.Context {
//			... configure logging, etc. ...
//			return ctx
//		},
//
//		Commands: []*subcommands.Command{
//			authcli.SubcommandInfo(auth.Options{}, "auth-info"),
//			authcli.SubcommandLogin(auth.Options{}, "auth-login"),
//			authcli.SubcommandLogout(auth.Options{}, "auth-logout"),
//			...
//		},
//	}
//
//	func main() {
//		os.Exit(subcommands.Run(application, nil))
//	}
package authcli

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/cli"
)

// CommandParams specifies various parameters for a subcommand.
type CommandParams struct {
	Name string // name of the subcommand.

	AuthOptions auth.Options // default auth options.

	// ScopesFlag specifies if -scope flag must be registered.
	// AuthOptions.Scopes is used as a default value.
	// If it is empty, defaults to "https://www.googleapis.com/auth/userinfo.email".
	ScopesFlag bool
}

// Flags defines command line flags related to authentication.
type Flags struct {
	defaults           auth.Options
	serviceAccountJSON string
	scopes             string
	registerScopesFlag bool
}

// Register adds auth related flags to a FlagSet.
func (fl *Flags) Register(f *flag.FlagSet, defaults auth.Options) {
	fl.defaults = defaults
	f.StringVar(&fl.serviceAccountJSON, "service-account-json", "", "Path to JSON file with service account credentials to use.")
	if fl.registerScopesFlag {
		defaultScopes := strings.Join(defaults.Scopes, " ")
		if defaultScopes == "" {
			defaultScopes = auth.OAuthScopeEmail
		}
		f.StringVar(&fl.scopes, "scopes", defaultScopes, "space-separated OAuth 2.0 scopes")
	}
}

// Options return instance of auth.Options struct with values set accordingly to
// parsed command line flags.
func (fl *Flags) Options() (auth.Options, error) {
	opts := fl.defaults
	if fl.serviceAccountJSON != "" {
		opts.Method = auth.ServiceAccountMethod
		opts.ServiceAccountJSONPath = fl.serviceAccountJSON
	}

	if fl.registerScopesFlag {
		opts.Scopes = strings.Split(fl.scopes, " ")
		sort.Strings(opts.Scopes)
	}
	return opts, nil
}

type commandRunBase struct {
	subcommands.CommandRunBase
	flags  Flags
	params *CommandParams
}

func (c *commandRunBase) registerBaseFlags() {
	c.flags.registerScopesFlag = c.params.ScopesFlag
	c.flags.Register(&c.Flags, c.params.AuthOptions)
}

// SubcommandLogin returns subcommands.Command that can be used to perform
// interactive login.
func SubcommandLogin(opts auth.Options, name string) *subcommands.Command {
	return SubcommandLoginWithParams(CommandParams{Name: name, AuthOptions: opts})
}

// SubcommandLoginWithParams returns subcommands.Command that can be used to perform
// interactive login.
func SubcommandLoginWithParams(params CommandParams) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: params.Name,
		ShortDesc: "performs interactive login flow",
		LongDesc:  "Performs interactive login flow and caches obtained credentials",
		CommandRun: func() subcommands.CommandRun {
			c := &loginRun{}
			c.params = &params
			c.registerBaseFlags()
			return c
		},
	}
}

type loginRun struct {
	commandRunBase
}

func (c *loginRun) Run(a subcommands.Application, _ []string) int {
	opts, err := c.flags.Options()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := cli.GetContext(a, c)
	client, err := auth.NewAuthenticator(ctx, auth.InteractiveLogin, opts).Client()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %s\n", err.Error())
		return 2
	}
	if canReportIdentity(opts.Scopes) {
		err = reportIdentity(ctx, client)
		if err != nil {
			return 3
		}
	} else {
		fmt.Println("Success.")
	}
	return 0
}

// SubcommandLogout returns subcommands.Command that can be used to purge cached
// credentials.
func SubcommandLogout(opts auth.Options, name string) *subcommands.Command {
	return SubcommandLogoutWithParams(CommandParams{Name: name, AuthOptions: opts})
}

// SubcommandLogoutWithParams returns subcommands.Command that can be used to purge cached
// credentials.
func SubcommandLogoutWithParams(params CommandParams) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: params.Name,
		ShortDesc: "removes cached credentials",
		LongDesc:  "Removes cached credentials from the disk",
		CommandRun: func() subcommands.CommandRun {
			c := &logoutRun{}
			c.params = &params
			c.registerBaseFlags()
			return c
		},
	}
}

type logoutRun struct {
	commandRunBase
}

func (c *logoutRun) Run(a subcommands.Application, args []string) int {
	opts, err := c.flags.Options()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := cli.GetContext(a, c)
	err = auth.NewAuthenticator(ctx, auth.SilentLogin, opts).PurgeCredentialsCache()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

// SubcommandInfo returns subcommand.Command that can be used to print current
// cached credentials.
func SubcommandInfo(opts auth.Options, name string) *subcommands.Command {
	return SubcommandInfoWithParams(CommandParams{Name: name, AuthOptions: opts})
}

// SubcommandInfoWithParams returns subcommand.Command that can be used to print current
// cached credentials.
func SubcommandInfoWithParams(params CommandParams) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: params.Name,
		ShortDesc: "prints an email address associated with currently cached token",
		LongDesc:  "Prints an email address associated with currently cached token",
		CommandRun: func() subcommands.CommandRun {
			c := &infoRun{}
			c.params = &params
			c.registerBaseFlags()
			return c
		},
	}
}

type infoRun struct {
	commandRunBase
}

func (c *infoRun) Run(a subcommands.Application, args []string) int {
	opts, err := c.flags.Options()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := cli.GetContext(a, c)
	client, err := auth.NewAuthenticator(ctx, auth.SilentLogin, opts).Client()
	if err == auth.ErrLoginRequired {
		fmt.Fprintln(os.Stderr, "Not logged in")
		return 2
	} else if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	if canReportIdentity(opts.Scopes) {
		err = reportIdentity(ctx, client)
		if err != nil {
			return 4
		}
	} else {
		fmt.Printf(
			"Refresh token exists, but it doesn't have %q scope, so can't report "+
				"who it belongs to.\n", auth.OAuthScopeEmail)
	}
	return 0
}

// SubcommandToken returns subcommand.Command that can be used to print current
// access token.
func SubcommandToken(opts auth.Options, name string) *subcommands.Command {
	return SubcommandTokenWithParams(CommandParams{Name: name, AuthOptions: opts})
}

// SubcommandTokenWithParams returns subcommand.Command that can be used to print current
// access token.
func SubcommandTokenWithParams(params CommandParams) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: params.Name,
		ShortDesc: "prints an access token",
		LongDesc:  "Generates an access token if requested and prints it.",
		CommandRun: func() subcommands.CommandRun {
			c := &tokenRun{}
			c.params = &params
			c.registerBaseFlags()
			c.Flags.DurationVar(
				&c.lifetime, "lifetime", time.Minute,
				"Minimum token lifetime. If existing token expired and refresh token or service account is not present, returns nothing.",
			)
			c.Flags.StringVar(
				&c.jsonOutput, "json-output", "",
				"Destination file to print token and expiration time in JSON. \"-\" for standard output.")
			return c
		},
	}
}

type tokenRun struct {
	commandRunBase
	lifetime   time.Duration
	jsonOutput string
}

// Process exit codes of SubcommandToken subcommand.
const (
	TokenExitCodeValidToken = iota
	TokenExitCodeNoValidToken
	TokenExitCodeInvalidInput
	TokenExitCodeInternalError
)

func (c *tokenRun) Run(a subcommands.Application, args []string) int {
	opts, err := c.flags.Options()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return TokenExitCodeInvalidInput
	}
	if c.lifetime > 45*time.Minute {
		fmt.Fprintln(os.Stderr, "lifetime cannot exceed 45m")
		return TokenExitCodeInvalidInput
	}

	ctx := cli.GetContext(a, c)
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, opts)
	token, err := authenticator.GetAccessToken(c.lifetime)
	if err != nil {
		if err == auth.ErrLoginRequired {
			fmt.Fprintln(os.Stderr, "Not logged in. Run 'authutil login'.")
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return TokenExitCodeNoValidToken
	}
	if token.AccessToken == "" {
		return TokenExitCodeNoValidToken
	}

	if c.jsonOutput == "" {
		fmt.Println(token.AccessToken)
	} else {
		out := os.Stdout
		if c.jsonOutput != "-" {
			out, err = os.Create(c.jsonOutput)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return TokenExitCodeInvalidInput
			}
			defer out.Close()
		}
		data := struct {
			Token  string `json:"token"`
			Expiry int64  `json:"expiry"`
		}{token.AccessToken, token.Expiry.Unix()}
		if err = json.NewEncoder(out).Encode(data); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return TokenExitCodeInternalError
		}
	}
	return TokenExitCodeValidToken
}

// canReportIdentity returns true if reportIdentity can be used with given list
// of scopes.
//
// reportIdentity works only if userinfo.email scope was used (which is also
// the default if no scopes are provided).
func canReportIdentity(scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		if scope == auth.OAuthScopeEmail {
			return true
		}
	}
	return false
}

// reportIdentity prints identity associated with credentials that the client
// puts into each request (if any).
func reportIdentity(ctx context.Context, c *http.Client) error {
	service := auth.NewGroupsService("", c)
	ident, err := service.FetchCallerIdentity(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch current identity: %s\n", err)
		return err
	}
	fmt.Printf("Logged in as %s\n", ident)
	return nil
}
