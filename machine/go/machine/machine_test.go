package machine_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/machinetest"
)

func TestCopy(t *testing.T) {
	in := machinetest.FullyFilledInDescription
	out := in.Copy()
	require.Equal(t, in, out)
	assertdeep.Copy(t, in, out)

	// Confirm that the two Dimensions are separate.
	in.Dimensions["baz"] = []string{"quux"}
	in.Dimensions["alpha"][0] = "zeta"
	require.NotEqual(t, in, out)
}

func TestAsMetricsTags_EmptyDimensions_ReturnsEmptyTags(t *testing.T) {
	emptyTags := map[string]string{
		machine.DimID:         "",
		machine.DimOS:         "",
		machine.DimDeviceType: "",
	}
	assert.Equal(t, emptyTags, machine.SwarmingDimensions{}.AsMetricsTags())
}

func TestAsMetricsTags_NilSlices_ReturnsEmptyTags(t *testing.T) {
	emptyTags := map[string]string{
		machine.DimID:         "",
		machine.DimOS:         "",
		machine.DimDeviceType: "",
	}
	assert.Equal(t, emptyTags, machine.SwarmingDimensions{
		machine.DimID:         nil,
		machine.DimOS:         nil,
		machine.DimDeviceType: nil,
	}.AsMetricsTags())
}

func TestAsMetricsTags_ZeroLengthSlices_ReturnsEmptyTags(t *testing.T) {
	emptyTags := map[string]string{
		machine.DimID:         "",
		machine.DimOS:         "",
		machine.DimDeviceType: "",
	}
	assert.Equal(t, emptyTags, machine.SwarmingDimensions{
		machine.DimID:         {},
		machine.DimOS:         {},
		machine.DimDeviceType: {},
	}.AsMetricsTags())
}

func TestAsMetricsTags_MultipleValues_ReturnsTagsWithMostSpecificValues(t *testing.T) {
	expected := map[string]string{
		machine.DimID:         "",
		machine.DimOS:         "iOS-13.6",
		machine.DimDeviceType: "",
	}
	assert.Equal(t, expected, machine.SwarmingDimensions{"os": []string{"iOS", "iOS-13.6"}}.AsMetricsTags())
}

func TestNewEvent(t *testing.T) {
	assert.Equal(t, machine.EventTypeRawState, machine.NewEvent().EventType)
}

func TestNewDescription(t *testing.T) {
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)
	ctx := now.TimeTravelingContext(serverTime)
	actual := machine.NewDescription(ctx)
	expected := machine.Description{
		AttachedDevice: machine.AttachedDeviceNone,
		Dimensions:     machine.SwarmingDimensions{},
		LastUpdated:    serverTime,
	}
	assert.Equal(t, expected, actual)
}

func descForCombination(maintenanceMode string, isQuarantined bool, recovering string) machine.Description {
	return machine.Description{
		MaintenanceMode: maintenanceMode,
		IsQuarantined:   isQuarantined,
		Recovering:      recovering,
		Dimensions:      machine.SwarmingDimensions{},
	}
}

func TestSetSwarmingQuarantinedMessage_NoQuarantined_MessageIsNotSet(t *testing.T) {
	d := descForCombination("", false, "")
	quarantined := machine.SetSwarmingQuarantinedMessage(&d)
	_, ok := d.Dimensions[machine.DimQuarantined]
	require.False(t, ok)
	require.False(t, quarantined)
}

func TestSetSwarmingQuarantinedMessage_MaintenanceMode_MessageIsSet(t *testing.T) {
	d := descForCombination("barney@example.com", false, "")
	quarantined := machine.SetSwarmingQuarantinedMessage(&d)
	require.Equal(t, "Maintenance: barney@example.com", d.Dimensions[machine.DimQuarantined][0])
	require.True(t, quarantined)
}

func TestSetSwarmingQuarantinedMessage_MaintenanceModeAndQuarantined_MessageIsSet(t *testing.T) {
	d := descForCombination("barney@example.com", true, "")
	quarantined := machine.SetSwarmingQuarantinedMessage(&d)
	require.Equal(t, "Maintenance: barney@example.com, Forced Quarantine", d.Dimensions[machine.DimQuarantined][0])
	require.True(t, quarantined)
}

func TestSetSwarmingQuarantinedMessage_MaintenanceModeAndQuarantinedAndRecovering_MessageIsSet(t *testing.T) {
	d := descForCombination("barney@example.com", true, "Low power.")
	quarantined := machine.SetSwarmingQuarantinedMessage(&d)
	require.Equal(t, "Maintenance: barney@example.com, Forced Quarantine, Recovering: Low power.", d.Dimensions[machine.DimQuarantined][0])
	require.True(t, quarantined)
}

func TestDescription_IsRecovering_ReturnsTrueIfHasRecoveryMessage(t *testing.T) {
	require.True(t, machine.Description{Recovering: "any non-empty string"}.IsRecovering())
}

func TestDescription_IsRecovering_ReturnsFalseIfRecoveryMessageIsEmpty(t *testing.T) {
	require.False(t, machine.Description{Recovering: ""}.IsRecovering())
}

func TestDescription_InMaintenanceMode_ReturnsTrueIfHasMaintenanceModeMessage(t *testing.T) {
	require.True(t, machine.Description{MaintenanceMode: "any non-empty string"}.InMaintenanceMode())
}

func TestDescription_InMaintenanceMode_ReturnsFalseIfMaintenanceModeMessageIsEmpty(t *testing.T) {
	require.False(t, machine.Description{}.InMaintenanceMode())
}

func TestSetSwarmingPool_NameStartsWithSkiaI_PoolSetToSkiaInternal(t *testing.T) {
	d := machine.NewDescription(context.Background())
	d.Dimensions["id"] = []string{"skia-i-rpi-001"}
	machine.SetSwarmingPool(&d)
	require.Equal(t, machine.PoolSkiaInternal, d.Dimensions.GetDimensionValueOrEmptyString(machine.DimPool))
}

func TestSetSwarmingPool_AllOtherMachinesGoInTheSkiaPool(t *testing.T) {
	d := machine.NewDescription(context.Background())
	d.Dimensions["id"] = []string{"skia-rpi2-rack4-shelf1-002"}
	machine.SetSwarmingPool(&d)
	require.Equal(t, machine.PoolSkia, d.Dimensions.GetDimensionValueOrEmptyString(machine.DimPool))
}

func TestHasValidPool_OnlyOneValidPool_ReturnsTrue(t *testing.T) {
	ctx := context.Background()
	d := machine.NewDescription(ctx)
	d.Dimensions[machine.DimPool] = []string{machine.PoolSkia}
	require.True(t, d.HasValidPool())
}

func TestHasValidPool_NoPoolKey_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	d := machine.NewDescription(ctx)
	require.False(t, d.HasValidPool())
}

func TestHasValidPool_TwoOrMorePoolNames_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	d := machine.NewDescription(ctx)
	d.Dimensions[machine.DimPool] = []string{machine.PoolSkia, machine.PoolSkiaInternal}
	require.False(t, d.HasValidPool())
}
