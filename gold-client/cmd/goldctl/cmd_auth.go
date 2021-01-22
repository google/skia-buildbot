package main

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/gold-client/go/auth"
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
`,
		Run: env.runAuthCmd,
	}

	// add the service-account flag.
	cmd.Flags().StringVar(&env.flagServiceAccount, fstrServiceAccount, "", "Service account file to be used to authenticate against GCP and Gold")

	// add the luci flag to use the LUCI_CONTEXT for authentication.
	cmd.Flags().BoolVar(&env.flagUseLUCIContext, fstrLUCI, false, "Use the LUCI context to retrieve an oauth token.")

	// add the workdir flag and make it required
	cmd.Flags().StringVar(&env.flagWorkDir, fstrWorkDir, "", "Work directory for intermediate results")
	must(cmd.MarkFlagRequired(fstrWorkDir))

	return cmd
}

func (a *authEnv) runAuthCmd(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()
	a.Auth(ctx)
}

// Auth executes the logic for the auth command. It
// sets up the work directory to support future calls (e.g. imgtest)
func (a *authEnv) Auth(ctx context.Context) {
	_, err := fileutil.EnsureDirExists(a.flagWorkDir)
	if err != nil {
		logErrfAndExit(ctx, "Could not make work dir: %s", err)
	}

	if a.flagUseLUCIContext {
		err = auth.InitLUCIAuth(a.flagWorkDir)
	} else if a.flagServiceAccount != "" {
		err = auth.InitServiceAccountAuth(a.flagServiceAccount, a.flagWorkDir)
	} else {
		logInfo(ctx, "Falling back to gsutil implementation\n")
		logInfo(ctx, "This should not be used in production.\n")
		err = auth.InitGSUtil(a.flagWorkDir)
	}
	ifErrLogExit(ctx, err)
	abs, err := filepath.Abs(a.flagWorkDir)
	ifErrLogExit(ctx, err)

	logInfof(ctx, "Authentication set up in directory %s\n", abs)

	// Open up the auth we configured and see if we can get an authenticated HTTPClient.
	// This helps catch auth errors early.
	authDir, err := auth.LoadAuthOpt(a.flagWorkDir)
	ifErrLogExit(ctx, err)

	err = authDir.Validate()
	ifErrLogExit(ctx, err)

	_, err = authDir.GetHTTPClient()
	ifErrLogExit(ctx, err)

	logInfo(ctx, "self test passed\n")
	exitProcess(ctx, 0)
}
