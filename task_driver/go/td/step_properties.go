package td

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
