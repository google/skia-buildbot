package main

import (
	"context"

	"github.com/spf13/cobra"
	"go.skia.org/infra/gold-client/go/goldclient"
)

// whoamiEnv provides the environment for the whoami command.
type whoamiEnv struct {
	workDir    string
	instanceID string
}

// getWhoamiCmd returns the definition of the whoami command.
func getWhoamiCmd() *cobra.Command {
	env := &whoamiEnv{}
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Make a request to Gold's /json/v1/whoami endpoint and print out its output.",
		Long: `
Will print out the email address of the user or service account used to authenticate the request.
For debugging purposes only.`,
		Run: env.runWhoamiCmd,
	}

	cmd.Flags().StringVar(&env.workDir, fstrWorkDir, "", "Work directory for intermediate results")
	cmd.Flags().StringVar(&env.instanceID, "instance", "", "ID of the Gold instance.")
	must(cmd.MarkFlagRequired(fstrWorkDir))
	must(cmd.MarkFlagRequired("instance"))

	return cmd
}

func (w *whoamiEnv) runWhoamiCmd(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	w.WhoAmI(ctx)
}

// WhoAmI loads the authentication data from disk and queries the server for who this data
// belongs to.
func (w *whoamiEnv) WhoAmI(ctx context.Context) {
	ctx = loadAuthenticatedClients(ctx, w.workDir)

	config := goldclient.GoldClientConfig{
		InstanceID: w.instanceID,
		WorkDir:    w.workDir,
	}

	// Overwrite any existing config in the work directory.
	goldClient, err := goldclient.NewCloudClient(config)
	ifErrLogExit(ctx, err)

	email, err := goldClient.Whoami(ctx)
	ifErrLogExit(ctx, err)
	logInfof(ctx, "Logged in as \"%s\".\n", email)
	exitProcess(ctx, 0)
}
