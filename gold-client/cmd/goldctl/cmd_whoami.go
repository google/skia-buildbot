package main

import (
	"fmt"

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

// runWhoamiCmd executes the whoami command.
func (w *whoamiEnv) runWhoamiCmd(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	auth, err := goldclient.LoadAuthOpt(w.workDir)
	ifErrLogExit(ctx, err)

	if auth == nil {
		logErrf(ctx, "Auth is empty - did you call goldctl auth first?")
		exitProcess(ctx, 1)
	}

	config := goldclient.GoldClientConfig{
		InstanceID: w.instanceID,
		WorkDir:    w.workDir,
	}

	// Overwrite any existing config in the work directory.
	goldClient, err := goldclient.NewCloudClient(auth, config)
	ifErrLogExit(ctx, err)

	url := goldclient.GetGoldInstanceURL(w.instanceID)
	logVerbose(ctx, fmt.Sprintf("Making request to %s/json/whoami\n", url))
	email, err := goldClient.Whoami()
	ifErrLogExit(ctx, err)
	logInfof(ctx, "%s/json/whoami returned \"%s\".\n", url, email)
}
