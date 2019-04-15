package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/gold-client/go/goldclient"
)

const (
	// Define the flag names to be consistent.
	fstrServiceAccount = "service-account"
	fstrLUCI           = "luci"
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
		Run: env.runAuthCmd,
	}

	// add the service-account flag.
	cmd.Flags().StringVarP(&env.flagServiceAccount, fstrServiceAccount, "", "", "Service account file to be used to authenticate against GCP and Gold")

	// add the luci flag to use the LUCI_CONTEXT for authentication.
	cmd.Flags().BoolVarP(&env.flagUseLUCIContext, fstrLUCI, "", false, "Use the LUCI context to retrieve an oauth token.")

	// add the workdir flag and make it required
	cmd.Flags().StringVarP(&env.flagWorkDir, fstrWorkDir, "", "", "Work directory for intermediate results")
	Must(cmd.MarkFlagRequired(fstrWorkDir))

	return cmd
}

// runAuthCommand
func (a *authEnv) runAuthCmd(cmd *cobra.Command, args []string) {
	_, err := fileutil.EnsureDirExists(a.flagWorkDir)
	if err != nil {
		logErrfAndExit(cmd, "Could not make work dir: %s", err)
	}

	if a.flagUseLUCIContext {
		err = goldclient.InitLUCIAuth(a.flagWorkDir)
	} else if a.flagServiceAccount != "" {
		err = goldclient.InitServiceAccountAuth(a.flagServiceAccount, a.flagWorkDir)
	} else {
		fmt.Println("Falling back to gsutil implementation")
		fmt.Println("This should not be used in production.")
		err = goldclient.InitGSUtil(a.flagWorkDir)
	}
	ifErrLogExit(cmd, err)
}
