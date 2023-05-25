package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/go/sklog"
)

// flag names
const (
	rootDigestFlagName  = "root-digest"
	casInstanceFlagName = "cas-instance"
)

// readCASCmd holds the flag values and any internal state necessary for
// executing the `readCASCmd` subcommand.
type readCASCmd struct {
	rootDigest  string
	casInstance string
}

// ReadCASCommand returns a [*cli.Command] for reading results data from RBE-CAS.
func ReadCASCommand() *cli.Command {
	cmd := &readCASCmd{}
	rootDigestFlag := &cli.StringFlag{
		Name:        rootDigestFlagName,
		Value:       "",
		Usage:       "CAS digest for the root node",
		Destination: &cmd.rootDigest,
	}
	casInstanceFlag := &cli.StringFlag{
		Name:        casInstanceFlagName,
		Value:       "projects/chrome-swarming/instances/default_instance",
		Usage:       "RBE-CAS instance",
		Destination: &cmd.casInstance,
	}
	return &cli.Command{
		Name:        "readcas",
		Description: "readcas reads results data from RBE-CAS.",
		Usage:       "cabe readcas -- --root-digest <root-digest> --cas-instance <cas-instance>",
		Flags: []cli.Flag{
			rootDigestFlag,
			casInstanceFlag,
		},
		Action: cmd.action,
	}
}

// action runs reading results data from RBE-CAS.
func (cmd *readCASCmd) action(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	rbeClients, err := backends.DialRBECAS(ctx)
	if err != nil {
		sklog.Fatalf("dialing RBE-CAS backends: %v", err)
		return err
	}

	rbeClient := rbeClients[cmd.casInstance]
	benchmarkResults, err := backends.FetchBenchmarkJSON(ctx, rbeClient, cmd.rootDigest)
	if err != nil {
		sklog.Fatalf("fetch benchmark json: %v", err)
		return err
	}

	for benchmarkName, res := range benchmarkResults {
		fmt.Println(benchmarkName, res)
	}

	return nil
}
