package goldpushk

import (
	"fmt"
	"os"
)

// Contains information about all the actions to be carried out.
type Goldpushk struct {
	deployments         []GoldServiceDeployment
	canariedDeployments []GoldServiceDeployment
	dryRun              bool

	unitTest bool // Disables confirmation prompt from unit tests.
}

// Goldpushk constructor.
func New(deployments []GoldServiceDeployment, canariedDeployments []GoldServiceDeployment, dryRun bool) *Goldpushk {
	return &Goldpushk{
		deployments:         deployments,
		canariedDeployments: canariedDeployments,
		dryRun:              dryRun,
	}
}

// Carries out all the service deployment steps.
func (g *Goldpushk) Run() error {
	g.PrintOutInputsAndAskConfirmation()
	g.RegenerateConfigFiles()
	g.CommitConfigFiles()
	g.PushCanaries()
	g.MonitorCanaries()
	g.PushServices()
	g.MonitorServices()

	return nil
}

// Prints out a summary of the actions to be taken, then asks the user for confirmation.
func (g *Goldpushk) PrintOutInputsAndAskConfirmation() {
	// Skip if running from an unit test.
	if g.unitTest {
		return
	}

	// Print out a summary of the services to deploy.
	if len(g.canariedDeployments) != 0 {
		fmt.Println("The following services will be canaried:")
		for _, d := range g.canariedDeployments {
			fmt.Printf("  %s\n", d.CanonicalName())
		}
		fmt.Println()
	}
		fmt.Println("The following services will be deployed:")
	for _, d := range g.deployments {
		fmt.Printf("  %s\n", d.CanonicalName())
	}

	// Ask for confirmation, ending execution by default.
	fmt.Printf("\nProceed? (y/N): ")
	var input string
	fmt.Scanln(&input)
	if input != "y" {
		fmt.Println("Aborting.")
		os.Exit(1)
	}
}

func (g *Goldpushk) RegenerateConfigFiles() {
	// TODO(lovisolo)
	fmt.Printf("Not implemented.\n")
	os.Exit(1)
}

func (g *Goldpushk) CommitConfigFiles() {
	// TODO(lovisolo)
	fmt.Printf("Not implemented.\n")
	os.Exit(1)
}

func (g *Goldpushk) PushCanaries() {
	// TODO(lovisolo)
	fmt.Printf("Not implemented.\n")
	os.Exit(1)
}

func (g *Goldpushk) MonitorCanaries() {
	// TODO(lovisolo)
	fmt.Printf("Not implemented.\n")
	os.Exit(1)
}

func (g *Goldpushk) PushServices() {
	// TODO(lovisolo)
	fmt.Printf("Not implemented.\n")
	os.Exit(1)
}

func (g *Goldpushk) MonitorServices() {
	// TODO(lovisolo)
	fmt.Printf("Not implemented.\n")
	os.Exit(1)
}
