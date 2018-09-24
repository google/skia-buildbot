package test_automation

import (
	"context"
	"net/http"
	"strings"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/exec"
)

const (
	MAX_STEP_NAME_CHARS = 100

	STEP_RESULT_SUCCESS = "SUCCESS"
	STEP_RESULT_FAILED  = "FAILED"
)

// Step represents a single action to take.
type Step struct {
	Id    string       `json:"id"`
	Fn    func() error `json:"-"`
	Cwd   string       `json:"cwd"`
	Env   []string     `json:"env,omitempty"`
	Infra bool         `json:"infra"`
	Name  string       `json:"name"`
}

// StepResult contains the results of a Step.
type StepResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Context is used to build and run Steps.
type Context struct {
	cwd    string
	env    []string
	infra  bool
	name   string
	parent *Context
	root   *rootContext
}

// rootContext contains resources needed by all Contexts.
type rootContext struct {
	ctx           context.Context
	httpClient    *http.Client
	stepCollector *stepCollector
}

// ContextProps describes properties used to create a Context.
type ContextProps struct {
	Cwd   string
	Env   []string
	Infra *bool
	Name  string
}

// Create a sub-context of this Context. Any properties which are not provided
// are inherited from this Context.
func (c *Context) SubContext(child *ContextProps) *Context {
	rv := &Context{
		cwd:   c.cwd,
		env:   c.env,
		name:  c.name + "_child",
		infra: c.infra,
		root:  c.root,
	}
	if child.Cwd != "" {
		rv.cwd = child.Cwd
	}
	if len(child.Env) != 0 {
		// TODO(borenet): Should we merge environments?
		rv.env = child.Env
	}
	if child.Name != "" {
		rv.name = child.Name
	}
	if child.Infra != nil {
		rv.infra = *child.Infra
	}
	return rv
}

// Create a sub-context for which all steps are infra steps.
func (c *Context) Infra() *Context {
	isInfra := true
	return c.SubContext(&ContextProps{
		Infra: &isInfra,
	})
}

// Create a sub-context in which all steps run in the given working directory.
func (c *Context) Cwd(cwd string) *Context {
	return c.SubContext(&ContextProps{
		Cwd: cwd,
	})
}

// Create a sub-context in which all steps have the given environment.
func (c *Context) Env(env []string) *Context {
	return c.SubContext(&ContextProps{
		Env: env,
	})
}

// Run the given function using this Context. Useful for managing scopes.
func (c *Context) Do(fn func(*Context) error) error {
	return fn(c)
}

// Run the given Step in this Context.
func (c *Context) RunStep(s *Step) error {
	// Prepare the step, using defaults from the Context.
	if s.Cwd == "" {
		s.Cwd = c.cwd
	}
	if len(s.Env) == 0 {
		// TODO(borenet): Should we merge environments?
		s.Env = c.env
	}
	// Don't inherit c.infra.

	// Run the step.
	s.Id = uuid.New() // TODO(borenet): Come up with a more systematic ID.
	c.root.stepCollector.Start(s)
	err := s.Fn()
	res := &StepResult{}
	if err != nil {
		res.Error = err.Error()
		res.Result = STEP_RESULT_FAILED
	} else {
		res.Result = STEP_RESULT_SUCCESS
	}
	c.root.stepCollector.Finish(res)
	return err
}

// Run the given function as a step in this Context.
func (c *Context) Run(name string, fn func() error) error {
	return c.RunStep(&Step{
		Fn:    fn,
		Infra: c.infra,
		Name:  name,
	})
}

// Run the given command as a step in this Context. Returns its output along
// with any error.
func (c *Context) Exec(args ...string) (string, error) {
	var output string
	err := c.RunStep(&Step{
		Fn: func() error {
			o, err := exec.RunCwd(c.Ctx(), c.cwd, args...)
			output = o
			return err
		},
		Infra: c.infra,
		Name:  strings.Join(args, " "),
	})
	return output, err
}

// Return a context.Context associated with this Context.
func (c *Context) Ctx() context.Context {
	return c.root.ctx
}

// Return an http.Client associated with this Context.
func (c *Context) HttpClient() *http.Client {
	return c.root.httpClient
}
