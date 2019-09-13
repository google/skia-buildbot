// Package goldpushk contains the Goldpushk struct, which coordinates all the
// operations performed by goldpushk.
//
// Also included in this package is function BuildDeployableUnitSet(), which
// returns a set with all the services goldpushk is able to manage.
//
// Function BuildDeployableUnitSet is the source of truth of goldpushk, and
// should be updated to reflect any relevant changes in configuration.

package goldpushk

import (
	"fmt"

	"go.skia.org/infra/go/skerr"
)

// Goldpushk contains information about the deployment steps to be carried out.
type Goldpushk struct {
	deployableUnits         []DeployableUnit
	canariedDeployableUnits []DeployableUnit
	dryRun                  bool

	unitTest bool // Disables confirmation prompt from unit tests.
}

// New is the Goldpushk constructor.
func New(deployableUnits []DeployableUnit, canariedDeployableUnits []DeployableUnit, dryRun bool) *Goldpushk {
	return &Goldpushk{
		deployableUnits:         deployableUnits,
		canariedDeployableUnits: canariedDeployableUnits,
		dryRun:                  dryRun,
	}
}

// Run carries out the deployment steps.
func (g *Goldpushk) Run() error {
	if ok, err := g.printOutInputsAndAskConfirmation(); err != nil {
		return err
	} else if !ok {
		return nil
	}
	if err := g.regenerateConfigFiles(); err != nil {
		return err
	}
	if err := g.commitConfigFiles(); err != nil {
		return err
	}
	if err := g.pushCanaries(); err != nil {
		return err
	}
	if err := g.monitorCanaries(); err != nil {
		return err
	}
	if err := g.pushServices(); err != nil {
		return err
	}
	if err := g.monitorServices(); err != nil {
		return err
	}
	return nil
}

// printOutInputsAndAskConfirmation prints out a summary of the actions to be
// taken, then asks the user for confirmation.
func (g *Goldpushk) printOutInputsAndAskConfirmation() (bool, error) {
	// Skip if running from an unit test.
	if g.unitTest {
		return true, nil
	}

	// Print out a summary of the services to deploy.
	if len(g.canariedDeployableUnits) != 0 {
		fmt.Println("The following services will be canaried:")
		for _, d := range g.canariedDeployableUnits {
			fmt.Printf("  %s\n", d.CanonicalName())
		}
		fmt.Println()
	}
	fmt.Println("The following services will be deployed:")
	for _, d := range g.deployableUnits {
		fmt.Printf("  %s\n", d.CanonicalName())
	}

	// Ask for confirmation, ending execution by default.
	fmt.Printf("\nProceed? (y/N): ")
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return false, skerr.Wrapf(err, "unable to read from standard input")
	}
	if input != "y" {
		fmt.Println("Aborting.")
		return false, nil
	}

	return true, nil
}

func (g *Goldpushk) regenerateConfigFiles() error {
	// TODO(lovisolo)
	return skerr.Fmt("not implemented")
}

func (g *Goldpushk) commitConfigFiles() error {
	// TODO(lovisolo)
	return skerr.Fmt("not implemented")
}

func (g *Goldpushk) pushCanaries() error {
	// TODO(lovisolo)
	return skerr.Fmt("not implemented")
}

func (g *Goldpushk) monitorCanaries() error {
	// TODO(lovisolo)
	return skerr.Fmt("not implemented")
}

func (g *Goldpushk) pushServices() error {
	// TODO(lovisolo)
	return skerr.Fmt("not implemented")
}

func (g *Goldpushk) monitorServices() error {
	// TODO(lovisolo)
	return skerr.Fmt("not implemented")
}
