package main

import (
	"fmt"

	gstorage "cloud.google.com/go/storage"
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/gold-client/go/goldclient"
	"golang.org/x/oauth2"
)

const (
	// Define the flag names to be consistent.
	fstrServiceAccount = "service-account"
	fstrLUCI           = "luci"
	fstrWorkDir        = "workdir"
)

// authEnv provides the environment for the auth command.
type authEnv struct {
	flagServiceAccount string
	flagUseLUCIContext bool
	flagWorkDir        string
}

// getAuthCmd returns the definition of the auth command.
func getAuthCmd() *cobra.Command {
	env := &authEnv{}
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate against GCP and Gold instances",
		Long: `
Authenticate against GCP and the Gold instance.
Currently only service accounts are supported. `,
		PreRunE: env.validateFlags,
		Run:     env.runAuthCmd,
	}

	// add the service-account flag.
	cmd.Flags().StringVarP(&env.flagServiceAccount, fstrServiceAccount, "", "", "Service account file to be used to authenticate against GCP and Gold")

	// add the luci flag to use the LUCI_CONTEXT for authentication.
	cmd.Flags().BoolVarP(&env.flagUseLUCIContext, fstrLUCI, "", false, "Use the LUCI context to retrieve an oauth token.")

	// add the workdir flag and make it required
	cmd.Flags().StringVarP(&env.flagWorkDir, fstrWorkDir, "", "", "Temporary work directory")
	_ = cmd.MarkFlagRequired(fstrWorkDir)

	return cmd
}

// validateFlags validates across individual flags.
func (a *authEnv) validateFlags(cmd *cobra.Command, args []string) error {
	if a.flagServiceAccount == "" && !a.flagUseLUCIContext {
		return fmt.Errorf("ERROR: Either the %q or %q flag must be set to choose an auth token source.", fstrServiceAccount, fstrLUCI)
	}
	return nil
}

// runAuthCommand
func (a *authEnv) runAuthCmd(cmd *cobra.Command, args []string) {
	config := &goldclient.GoldClientConfig{
		WorkDir: a.flagWorkDir,
	}

	// Create a cloud based Gold client and authenticate.
	goldClient, err := goldclient.NewCloudClient(config, nil)
	ifErrLogExit(cmd, err)

	var tokenSrc oauth2.TokenSource
	if a.flagUseLUCIContext {
		tokenSrc, err = auth.NewLUCIContextTokenSource(gstorage.ScopeFullControl)
	} else {
		tokenSrc, err = auth.NewJWTServiceAccountTokenSource("#bogus", a.flagServiceAccount, gstorage.ScopeFullControl)
	}
	ifErrLogExit(cmd, err)

	err = goldClient.ServiceAccount(tokenSrc)
	ifErrLogExit(cmd, err)
}
