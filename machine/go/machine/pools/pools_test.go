// Pools handles processing the pool configuration and applying it to
// Dimensions.
package pools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/pools/poolstest"
	"go.skia.org/infra/machine/go/machineserver/config"
)

func setupForTest(t *testing.T) (*Pools, machine.Description) {
	ctx := context.Background()
	p, err := New(poolstest.PoolConfigForTesting)
	require.NoError(t, err)
	d := machine.NewDescription(ctx)
	return p, d
}

func TestHasValidPool_OnlyOneValidPool_ReturnsTrue(t *testing.T) {
	p, d := setupForTest(t)
	d.Dimensions[machine.DimPool] = []string{machine.PoolSkia}
	require.True(t, p.HasValidPool(d))
}

func TestHasValidPool_NoPoolKey_ReturnsFalse(t *testing.T) {
	p, d := setupForTest(t)
	require.False(t, p.HasValidPool(d))
}

func TestHasValidPool_TwoOrMorePoolNames_ReturnsFalse(t *testing.T) {
	p, d := setupForTest(t)
	d.Dimensions[machine.DimPool] = []string{machine.PoolSkia, machine.PoolSkiaInternal}
	require.False(t, p.HasValidPool(d))
}

func TestSetSwarmingPool_NameStartsWithSkiaI_PoolSetToSkiaInternal(t *testing.T) {
	p, d := setupForTest(t)
	d.Dimensions["id"] = []string{"skia-i-rpi-001"}
	p.SetSwarmingPool(&d)
	require.Equal(t, machine.PoolSkiaInternal, d.Dimensions.GetDimensionValueOrEmptyString(machine.DimPool))
}

func TestSetSwarmingPool_AllOtherMachinesGoInTheSkiaPool(t *testing.T) {
	p, d := setupForTest(t)
	d.Dimensions["id"] = []string{"skia-rpi2-rack4-shelf1-002"}
	p.SetSwarmingPool(&d)
	require.Equal(t, machine.PoolSkia, d.Dimensions.GetDimensionValueOrEmptyString(machine.DimPool))
}

func TestSetSwarmingPool_EmpytPoolConfig_DimPoolSetToUnknownPool(t *testing.T) {
	p, err := New(config.InstanceConfig{})
	require.NoError(t, err)
	d := machine.NewDescription(context.Background())
	d.Dimensions["id"] = []string{"skia-rpi2-rack4-shelf1-002"}
	p.SetSwarmingPool(&d)
	require.Equal(t, UnknownPool, d.Dimensions.GetDimensionValueOrEmptyString(machine.DimPool))
}

func TestNew_InvalidPoolName_ReturnsError(t *testing.T) {
	_, err := New(config.InstanceConfig{
		Pools: []config.Pool{
			{
				Name:  "this ^ is & not * a valid # pool @ name",
				Regex: "^skia-i-",
			},
		},
	})
	require.Error(t, err)
}

func TestNew_InvalidRegex_ReturnsError(t *testing.T) {
	_, err := New(config.InstanceConfig{
		Pools: []config.Pool{
			{
				Name:  "Skia",
				Regex: "(",
			},
		},
	})
	require.Error(t, err)
}
