package main

// goldctl is a CLI for working with the Gold service.

import (
	"github.com/spf13/cobra"
)

type authEnv struct{}

func getAuthCmd() *cobra.Command {
	env := &authEnv{}
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate against GCP",
		Long: `
Authenticate against GCP - TODO: How to specify the service account file ? `,
		Run: env.runAuthCommand,
	}

	return authCmd
}

func (a *authEnv) runAuthCommand(cmd *cobra.Command, args []string) { notImplemented(cmd) }
