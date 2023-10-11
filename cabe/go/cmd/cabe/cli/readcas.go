package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

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
	commonCmd
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
		Usage:       "cas instance",
		Destination: &cmd.casInstance,
	}
	flags := cmd.flags()
	flags = append(flags, rootDigestFlag)
	flags = append(flags, casInstanceFlag)
	return &cli.Command{
		Name:        "readcas",
		Usage:       "readcas reads perf results json data from RBE-CAS, located using the provided root-digest.",
		Description: "cabe readcas -- --root-digest <root-digest> --cas-instance <cas-instance>",
		Flags:       flags,
		Action:      cmd.action,
		After:       cmd.cleanup,
	}
}

// action runs reading results data from RBE-CAS.
func (cmd *readCASCmd) action(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	if err := cmd.dialBackends(ctx); err != nil {
		return err
	}

	benchmarkResults, err := cmd.casResultReader(ctx, cmd.casInstance, cmd.rootDigest)

	if err != nil {
		sklog.Errorf("fetch benchmark json: %v", err)
		return err
	}

	for benchmarkName, res := range benchmarkResults {
		fmt.Println(benchmarkName, res)
	}

	return nil
}
