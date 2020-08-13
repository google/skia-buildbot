package properties

import (
	"errors"

	"go.skia.org/infra/go/util"
)

const (
	// Special ID of the root step.
	STEP_ID_ROOT = "root"
)

// RunProperties are properties for a single run of a Task Driver.
type RunProperties struct {
	Local          bool   `json:"local"`
	SwarmingBot    string `json:"swarmingBot,omitempty"`
	SwarmingServer string `json:"swarmingServer,omitempty"`
	SwarmingTask   string `json:"swarmingTask,omitempty"`
}

// Return an error if the RunProperties are not valid.
func (p *RunProperties) Validate() error {
	if p.Local {
		if p.SwarmingBot != "" {
			return errors.New("SwarmingBot must be empty for local runs!")
		}
		if p.SwarmingServer != "" {
			return errors.New("SwarmingServer must be empty for local runs!")
		}
		if p.SwarmingTask != "" {
			return errors.New("SwarmingTask must be empty for local runs!")
		}
	} else {
		if p.SwarmingBot == "" {
			return errors.New("SwarmingBot is required for non-local runs!")
		}
		if p.SwarmingServer == "" {
			return errors.New("SwarmingServer is required for non-local runs!")
		}
		if p.SwarmingTask == "" {
			return errors.New("SwarmingTask is required for non-local runs!")
		}
	}
	return nil
}

// Return a copy of the RunProperties.
func (p *RunProperties) Copy() *RunProperties {
	if p == nil {
		return nil
	}
	return &RunProperties{
		Local:          p.Local,
		SwarmingBot:    p.SwarmingBot,
		SwarmingServer: p.SwarmingServer,
		SwarmingTask:   p.SwarmingTask,
	}
}

// StepProperties are basic properties of a step.
type StepProperties struct {
	// ID of the step. This is set by the framework and should not be set
	// by callers.
	Id string `json:"id"`

	// Name of the step.
	Name string `json:"name"`

	// If true, this step is marked as infrastructure-specific.
	IsInfra bool `json:"isInfra"`

	// All subprocesses spawned for this step will inherit these environment
	// variables.
	Environ []string `json:"environment,omitempty"`

	// Parent step ID. This is set by the framework and should not be set
	// by callers.
	Parent string `json:"parent,omitempty"`
}

// Props sets the name of the step. It returns a StepProperties instance which
// can be further modified by the caller.
func Props(name string) *StepProperties {
	return &StepProperties{
		Name: name,
	}
}

// Infra marks the step as infrastructure-specific.
func (p *StepProperties) Infra() *StepProperties {
	p.IsInfra = true
	return p
}

// Env applies the given environment variables to all commands run within this
// step. Note that this does NOT apply the variables to the environment of this
// process, just of subprocesses spawned using the context.
func (p *StepProperties) Env(env []string) *StepProperties {
	p.Environ = env
	return p
}

// Copy returns a deep copy of the StepProperties.
func (p *StepProperties) Copy() *StepProperties {
	if p == nil {
		return nil
	}
	return &StepProperties{
		Id:      p.Id,
		Name:    p.Name,
		IsInfra: p.IsInfra,
		Environ: util.CopyStringSlice(p.Environ),
		Parent:  p.Parent,
	}
}

// Return an error if the StepProperties are not valid.
func (p *StepProperties) Validate() error {
	if p.Id == "" {
		return errors.New("Id is required.")
	} else if p.Id != STEP_ID_ROOT && p.Parent == "" {
		return errors.New("Non-root steps must have a parent.")
	}
	return nil
}
