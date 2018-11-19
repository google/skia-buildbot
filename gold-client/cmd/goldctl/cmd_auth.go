package main

import (
	"github.com/spf13/cobra"
)

// TODO(stephana): Implement the auth command that's currently stubbed out.

// authEnv provides the environment for the auth command.
type authEnv struct{}

// getAuthCmd returns the definition of the auth command.
func getAuthCmd() *cobra.Command {
	env := &authEnv{}
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate against GCP",
		Long: `
Authenticate against GCP - TODO: How to specify the service account file ? `,
		Run: env.runAuthCmd,
	}

	return authCmd
}

// runAuthCommand
func (a *authEnv) runAuthCmd(cmd *cobra.Command, args []string) { notImplemented(cmd) }
