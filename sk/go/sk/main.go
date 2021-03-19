package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"go.skia.org/infra/sk/go/try"
)

func main() {
	rootCmd := &cobra.Command{
		Use:  "sk",
		Long: `sk provides developer workflow tools for Skia.`,
	}
	rootCmd.AddCommand(try.Command())
	ctx := context.Background()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
