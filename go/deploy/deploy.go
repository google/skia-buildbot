package deploy

import (
	"flag"
	"fmt"

	"github.com/google/uuid"
	"go.skia.org/infra/go/util"
)

const (
	PROD        Deployment = "production" // This is not "prod" because BigTable has a minimum instance name length.
	INTERNAL    Deployment = "internal"
	STAGING     Deployment = "staging"
	DEVELOPMENT Deployment = "development"
)

var (
	VALID_DEPLOYMENTS = []string{
		string(PROD),
		string(INTERNAL),
		string(STAGING),
		string(DEVELOPMENT),
	}
)

// Deployment indicates whether a service is running in production, a staging
// environment, or an internal environment.
type Deployment string

// Valid returns true if the Deployment is valid.
func (d Deployment) Valid() bool {
	return util.In(string(d), VALID_DEPLOYMENTS)
}

// Validate returns an error if the Deployment is not valid.
func (d Deployment) Validate() error {
	if !d.Valid() {
		return fmt.Errorf("Invalid deployment %q; must be one of: %v", d, VALID_DEPLOYMENTS)
	}
	return nil
}

// See documentation for flag.Value interface.
func (d *Deployment) String() string {
	// According to the docs, the flag package may call String() with a
	// zero-valued receiver.
	if d == nil {
		return ""
	}
	return string(*d)
}

// See documentation for flag.Value interface.
func (d *Deployment) Set(v string) error {
	if err := Deployment(v).Validate(); err != nil {
		return err
	}
	*d = Deployment(v)
	return nil
}

// Flag returns a flag which parses a Deployment from a command line flag.
func Flag(name string) *Deployment {
	var d Deployment
	flag.Var(&d, name, fmt.Sprintf("Deployment environment. One of: %v", VALID_DEPLOYMENTS))
	return &d
}

// Testing returns a relatively-unique Deployment intended for testing. It is
// not Valid().
func Testing() Deployment {
	return Deployment(fmt.Sprintf("testing-%s", uuid.New()))
}
