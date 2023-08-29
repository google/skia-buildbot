package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestSandwichCommand(t *testing.T) {
	got := SandwichCommand()
	require.NotNil(t, got)
	assert.Equal(t, "sandwich", got.Name)
}

func TestSandwichCommand_action_noArgs_fails(t *testing.T) {
	sCmd := sandwichCmd{
		dryRun: true,
	}

	ctx := context.Background()
	cliCtx := cli.NewContext(nil, nil, nil)
	cliCtx.Context = ctx
	err := sCmd.action(cliCtx)
	require.Error(t, err)
}

func TestSandwichCommand_action_executionID(t *testing.T) {
	sCmd := sandwichCmd{
		executionID: "1234",
		dryRun:      true,
	}

	ctx := context.Background()
	cliCtx := cli.NewContext(nil, nil, nil)
	cliCtx.Context = ctx
	err := sCmd.action(cliCtx)
	require.NoError(t, err)
}
