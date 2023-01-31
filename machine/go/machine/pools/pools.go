// Pools handles processing the pool configuration and applying it to
// Dimensions.
package pools

import (
	"regexp"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
)

const (
	// UnknownPool is the pool name used if a machine doesn't match any
	// configured pools.
	UnknownPool = "PoolNotFound"
)

var (
	validPoolName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-._]*$`)
)

// Pool is the config for a single pool.
type Pool struct {
	// Name of the pool as it will appear in Dimensions at the machine.DimPool key.
	Name string

	// Regex is a regular expression that matches a machine id if that machine
	// is in this pool.
	Regex *regexp.Regexp
}

// Pools handles the Pool part of InstanceConfig and applies it to Dimensions.
type Pools struct {
	pools             []Pool
	allValidPoolNames []string
}

// New returns a new instance of Pools.
func New(cfg config.InstanceConfig) (*Pools, error) {
	var pools []Pool
	var poolNames []string
	for _, pool := range cfg.Pools {
		r, err := regexp.Compile(pool.Regex)
		if err != nil {
			return nil, skerr.Wrapf(err, "compiling regex for pool: %q", pool.Name)
		}
		if !validPoolName.MatchString(pool.Name) {
			return nil, skerr.Fmt("invalid pool name: %q", pool.Name)
		}
		pools = append(pools, Pool{
			Name:  pool.Name,
			Regex: r,
		})
		poolNames = append(poolNames, pool.Name)
	}

	return &Pools{
		pools:             pools,
		allValidPoolNames: poolNames,
	}, nil
}

// HasValidPool returns true if the pool dimension is valid.
//
// By design, a task can only ever be scheduled in one pool and it must be a
// valid pool.
func (p *Pools) HasValidPool(d machine.Description) bool {
	pool, ok := d.Dimensions[machine.DimPool]

	return ok && len(pool) == 1 && util.In(pool[0], p.allValidPoolNames)
}

// SetSwarmingPool based on the machine id.
//
// Pools are checked in the order they appear in the config file.
func (p *Pools) SetSwarmingPool(d *machine.Description) {
	machineName := d.Dimensions.GetDimensionValueOrEmptyString("id")
	for _, pool := range p.pools {
		if pool.Regex.MatchString(machineName) {
			d.Dimensions[machine.DimPool] = []string{pool.Name}
			return
		}
	}
	d.Dimensions[machine.DimPool] = []string{UnknownPool}
}
