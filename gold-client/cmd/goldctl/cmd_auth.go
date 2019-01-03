package main

import (
	gstorage "cloud.google.com/go/storage"
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/gold-client/go/goldclient"
)

// authEnv provides the environment for the auth command.
type authEnv struct {
	flagServiceAccount string
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
		Run: env.runAuthCmd,
	}

	// add the --service-account flag and make it required
	cmd.Flags().StringVarP(&env.flagServiceAccount, "service-account", "", "", "Service account file to be used to authenticate against GCP and Gold")
	_ = cmd.MarkFlagRequired("service-account")

	// add the --work-dir flag and make it required
	cmd.Flags().StringVarP(&env.flagWorkDir, "workdir", "", "", "Temporary work directory")
	_ = cmd.MarkFlagRequired("work-dir")

	return cmd
}

// runAuthCommand
func (a *authEnv) runAuthCmd(cmd *cobra.Command, args []string) {
	config := &goldclient.GoldClientConfig{
		WorkDir: a.flagWorkDir,
	}

	// Create a cloud based Gold client and authenticate.
	goldClient, err := goldclient.NewCloudClient(config, nil)
	ifErrLogExit(cmd, err)

	tokenSrc, err := auth.NewJWTServiceAccountTokenSource("#bogus", a.flagServiceAccount, gstorage.ScopeFullControl)
	ifErrLogExit(cmd, err)

	err = goldClient.ServiceAccount(tokenSrc)
	ifErrLogExit(cmd, err)

}
