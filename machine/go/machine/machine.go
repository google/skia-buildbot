package machine

import (
	"context"
	"strings"
	"time"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
)

// SwarmingDimensions is for de/serializing swarming dimensions:
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+doc/master/appengine/swarming/doc/Magic-Values.md#bot-dimensions
type SwarmingDimensions map[string][]string

// Copy returns a deep copy of the dimensions map.
func (s SwarmingDimensions) Copy() SwarmingDimensions {
	n := make(SwarmingDimensions, len(s))
	for k, v := range s {
		copyValues := make([]string, len(v))
		copy(copyValues, v)
		n[k] = copyValues
	}
	return n
}

// GetDimensionValueOrEmptyString returns the last string in the value slice at
// the given key, or the empty string if the key doesn't exist.
func (s SwarmingDimensions) GetDimensionValueOrEmptyString(key string) string {
	if values, ok := s[key]; ok && len(values) > 0 {
		return values[len(values)-1]
	}
	return ""
}

// AsMetricsTags returns a map that is suitable to pass as tags for a metric.
//
// If there are multiple values for a key only the most specific value us used.
func (s SwarmingDimensions) AsMetricsTags() map[string]string {
	// Note that all metrics with the same name must have the exact same set of
	// tag keys so we can't just stuff all the Dimensions into the tags.
	return map[string]string{
		DimID:         s.GetDimensionValueOrEmptyString(DimID),
		DimOS:         s.GetDimensionValueOrEmptyString(DimOS),
		DimDeviceType: s.GetDimensionValueOrEmptyString(DimDeviceType),
	}
}

// Well-known swarming dimensions:
const (
	DimID                     = "id"
	DimOS                     = "os"
	DimQuarantined            = "quarantined"
	DimDeviceType             = "device_type"
	DimAndroidDevices         = "android_devices"
	DimChromeOSChannel        = "chromeos_channel"
	DimChromeOSMilestone      = "chromeos_milestone"
	DimChromeOSReleaseVersion = "release_version"
	DimCores                  = "cores"
	DimCPU                    = "cpu"
	DimGPU                    = "gpu"
	DimTaskType               = "task_type"
	DimPool                   = "pool"

	BadBatteryLevel = -99
)

// TaskRequestor is the system that is allowed to schedule jobs on a machine.
// This is the value type for the Dimension keyed by DimTaskType.
type TaskRequestor string

const (
	// Swarming is allowed to schedule tasks.
	Swarming TaskRequestor = "swarming"

	// SkTask means Skia Task Scheduler is allowed to schedule tasks.
	SkTask TaskRequestor = "sktask"
)

var AllTaskRequestorStates = []TaskRequestor{Swarming, SkTask}

// Pool names.
const (
	PoolSkia         = "Skia"
	PoolSkiaInternal = "SkiaInternal"
)

// AllValidPools contains all valid pool names.
var AllValidPools = []string{PoolSkia, PoolSkiaInternal}

// PowerCycleState is the state of powercycling for a single machine.
type PowerCycleState string

const (
	// NotAvailable means powercycling is not available for this machine. This
	// is the default.
	NotAvailable PowerCycleState = "not_available"

	// Available means powercycling is available for this machine.
	Available PowerCycleState = "available"

	// InError means that powercycle should be available, but an error has
	// occurred on powercycle_server, likely it failed to connect to the power
	// cycle device, aka the POE switch, or the PD.
	InError PowerCycleState = "in_error"
)

var AllPowerCycleStates = []PowerCycleState{NotAvailable, Available, InError}

// AttachedDevice is what kind of mobile device, if any, we expect to find attached to the test machine.
type AttachedDevice string

const (
	// AttachedDeviceNone means no device is attached. Used for all non-mobile
	// test machines, like Windows boxes, as well as Raspberry Pis that have no
	// device attached yet.
	AttachedDeviceNone AttachedDevice = "nodevice"

	// AttachedDeviceAdb means an Android device, or anything else that
	// understands adb.
	AttachedDeviceAdb AttachedDevice = "adb"

	// AttachedDeviceIOS means an iOS device, or anything else that we talk to
	// using idevice* commands.
	AttachedDeviceIOS AttachedDevice = "ios"

	// AttachedDeviceSSH means a ChromeOS device, or any other device we
	// interact with via SSH.
	AttachedDeviceSSH AttachedDevice = "ssh"
)

var AllAttachedDevices = []AttachedDevice{AttachedDeviceNone, AttachedDeviceAdb, AttachedDeviceIOS, AttachedDeviceSSH}

// Annotation represents a timestamped message.
type Annotation struct {
	Message   string
	User      string
	Timestamp time.Time
}

// Description is the current state of a single machine.
type Description struct {

	// MaintenanceMode is non-empy if someone manually puts the machine into
	// this mode. The value will be the user's email address and the date of the
	// change.
	MaintenanceMode string `sql:"maintenance_mode STRING NOT NULL DEFAULT ''"`

	// IsQuarantined is true if the machine has failed too many tasks and should
	// stop running tasks pending user intervention. Recipes/Task Drivers can
	// write a $HOME/${SWARMING_BOT_ID}.quarantined file to move a machine into
	// quarantined mode.
	IsQuarantined bool `sql:"is_quarantined BOOL NOT NULL DEFAULT FALSE"`

	// Recovering is a non-empty string if test_machine_monitor detects the
	// device is too hot, or low on charge. The value is a description of what
	// is recovering.
	Recovering string `sql:"recovering STRING NOT NULL DEFAULT ''"`

	// AttachedDevice defines the kind of device attached to this test machine,
	// if any.
	AttachedDevice AttachedDevice `sql:"attached_device STRING NOT NULL DEFAULT 'nodevice'"`

	// Annotation is used to record the most recent non-user change to Description.
	// For example, if the device battery is too low, this will be set automatically.
	// This will be in addition to the normal auditlog of user actions:
	// https://pkg.go.dev/go.skia.org/infra/go/auditlog?tab=doc
	Annotation Annotation `sql:"annotation JSONB NOT NULL"`

	// Note is a user authored message on the state of a machine.
	Note Annotation `sql:"note JSONB NOT NULL"`

	// Version of test_machine_monitor being run.
	Version string `sql:"version STRING NOT NULL DEFAULT ''"`

	// PowerCycle is true if the machine needs to be power-cycled.
	PowerCycle bool `sql:"powercycle BOOL NOT NULL DEFAULT FALSE"`

	// PowerCycleState is the state of power cycling availability for this
	// machine.
	PowerCycleState PowerCycleState `sql:"powercycle_state STRING NOT NULL DEFAULT 'not_available'"`

	LastUpdated         time.Time          `sql:"last_updated TIMESTAMPTZ NOT NULL"`
	Battery             int                `sql:"battery INT NOT NULL DEFAULT 0"` // Charge as an integer percent, e.g. 50% = 50.
	Temperature         map[string]float64 `sql:"temperatures JSONB NOT NULL"`    // In Celsius.
	RunningSwarmingTask bool               `sql:"running_swarmingTask BOOL NOT NULL DEFAULT FALSE"`
	LaunchedSwarming    bool               `sql:"launched_swarming BOOL NOT NULL DEFAULT FALSE"` // True if test_machine_monitor launched Swarming.
	RecoveryStart       time.Time          `sql:"recovery_start TIMESTAMPTZ NOT NULL"`           // When did the machine start being in recovery mode.
	// DeviceUptime is how long the attached device has been up. It is measured in seconds.
	DeviceUptime int32 `sql:"device_uptime INT4 DEFAULT 0"`

	// SSHUserIP, for example, "root@skia-sparky360-03" indicates we should connect to the
	// given ChromeOS device at that username and ip/hostname.
	SSHUserIP string `sql:"ssh_user_ip STRING NOT NULL DEFAULT ''"`

	// SuppliedDimensions are dimensions that we, the humans, supply because they are difficult
	// for the automated system to gather. These are used only for ChromeOS devices, which don't
	// readily report their CPU and GPU.
	SuppliedDimensions SwarmingDimensions `sql:"supplied_dimensions JSONB NOT NULL"`

	// Dimensions describe what hardware/software this machine has and informs what tasks
	// it can run.
	Dimensions SwarmingDimensions `sql:"dimensions JSONB NOT NULL"`

	// TaskRequest, if present, will be the trigger that launches a task.
	//
	// To kill a running TaskRequest, just modify the Description to delete the TaskRequest, i.e. TaskRequest = null.
	TaskRequest *types.TaskRequest `sql:"task_request JSONB" json:",omitempty"`

	// TaskStarted records when a task was started. This value is set on
	// machineserver during Update.
	TaskStarted time.Time `sql:"task_started TIMESTAMPTZ NOT NULL"`

	// Create a computed column with the machine id to use as the primary key.
	machineIDComputed struct{} `sql:"machine_id STRING PRIMARY KEY AS (dimensions->'id'->>0) STORED"`

	// Create generalized inverted index (GIN) for Dimensions.
	dimensionsIndex struct{} `sql:"INVERTED INDEX dimensions_gin (dimensions)"`

	// Create an index for the powercycle column.
	powerCycleIndex struct{} `sql:"INDEX by_powercycle (powercycle)"`
}

// IsRecovering returns true if the machine is recoving, i.e. has a non-empty Recovering message.
func (d Description) IsRecovering() bool {
	return d.Recovering != ""
}

// InMaintenanceMode returns true if the machine is in maintenance mode, i.e. has a non-empty MaintenanceMode message.
func (d Description) InMaintenanceMode() bool {
	return d.MaintenanceMode != ""
}

// HasValidPool returns true if the pool dimension is valid.
func (d Description) HasValidPool() bool {
	pool, ok := d.Dimensions[DimPool]

	return ok && len(pool) == 1 && util.In(pool[0], AllValidPools)
}

// DestFromDescription returns a slice of interface containing pointers to every public member
// of Description. This is useful in code that stores the Description in an SQL database.
//
// Make sure this always stays in the same order as the fields appear in the struct.
func DestFromDescription(d *Description) []interface{} {
	return []interface{}{
		&d.MaintenanceMode,
		&d.IsQuarantined,
		&d.Recovering,
		&d.AttachedDevice,
		&d.Annotation,
		&d.Note,
		&d.Version,
		&d.PowerCycle,
		&d.PowerCycleState,
		&d.LastUpdated,
		&d.Battery,
		&d.Temperature,
		&d.RunningSwarmingTask,
		&d.LaunchedSwarming,
		&d.RecoveryStart,
		&d.DeviceUptime,
		&d.SSHUserIP,
		&d.SuppliedDimensions,
		&d.Dimensions,
		&d.TaskRequest,
		&d.TaskStarted,
	}
}

// SetSwarmingQuarantinedMessage sets the Swarming Dimensions to reflect the
// full quarantined state. Returns true if the machine is quarantined.
func SetSwarmingQuarantinedMessage(d *Description) bool {
	parts := []string{}
	if d.InMaintenanceMode() {
		parts = append(parts, "Maintenance: "+d.MaintenanceMode)
	}
	if d.IsQuarantined {
		parts = append(parts, "Forced Quarantine")
	}
	if d.IsRecovering() {
		parts = append(parts, "Recovering: "+d.Recovering)
	}
	msg := strings.Join(parts, ", ")

	delete(d.Dimensions, DimQuarantined)
	if msg != "" {
		d.Dimensions[DimQuarantined] = []string{msg}
		return true
	}
	return false
}

// SetSwarmingPool based on the machine id.
func SetSwarmingPool(d *Description) {
	machineName := d.Dimensions.GetDimensionValueOrEmptyString("id")
	if strings.HasPrefix(machineName, "skia-i-") {
		d.Dimensions[DimPool] = []string{PoolSkiaInternal}
	} else {
		d.Dimensions[DimPool] = []string{PoolSkia}
	}
}

// NewDescription returns a new Description instance. It describes an available machine with no
// known dimensions.
func NewDescription(ctx context.Context) Description {
	return Description{
		AttachedDevice: AttachedDeviceNone,
		Dimensions:     SwarmingDimensions{},
		LastUpdated:    now.Now(ctx),
	}
}

// Copy returns a deep copy of Description.
func (d Description) Copy() Description {
	ret := d
	ret.Dimensions = d.Dimensions.Copy()
	ret.SuppliedDimensions = d.SuppliedDimensions.Copy()
	ret.Temperature = map[string]float64{}
	for k, v := range d.Temperature {
		ret.Temperature[k] = v
	}
	if d.TaskRequest != nil {
		tr := *d.TaskRequest
		ret.TaskRequest = &tr
	}
	return ret
}

// EventType is the type of update we got from the machine.
type EventType string

const (
	// EventTypeRawState means the raw state from test_machine_monitor has been
	// updated.
	EventTypeRawState EventType = "raw_state"
)

// Android contains the raw results from interrogating an Android device.
type Android struct {
	GetProp               string `json:"getprop"`
	DumpsysBattery        string `json:"dumpsys_battery"`
	DumpsysThermalService string `json:"dumpsys_thermal_service"`
	// A positive Uptime indicates there is an Android device attached to the host.
	Uptime time.Duration `json:"uptime"`
}

// IsPopulated returns whether the Android subevent record has been filled out, indicating that an
// Android device is attached.
func (a *Android) IsPopulated() bool {
	return a.Uptime > 0
}

// Host is information about the host machine.
type Host struct {
	// Name is the machine id, from SWARMING_BOT_ID environment variable or hostname().
	Name string `json:"name"`

	// Version of test_machine_monitor being run.
	Version string `json:"version"`

	// StartTime is when the test_machine_monitor started running.
	StartTime time.Time `json:"start_time"`
}

// ChromeOS encapsulates the information reported by a ChromeOS machine.
type ChromeOS struct {
	Channel        string `json:"channel"`
	Milestone      string `json:"milestone"`
	ReleaseVersion string `json:"release_version"`
	// A positive Uptime indicates there is an ChromeOS device attached to the host.
	Uptime time.Duration `json:"uptime"`
}

// IsPopulated returns whether the ChromeOS subevent record has been filled out, implying that the
// machine from which the event originated drives tests on a ChromeOS device.
func (c *ChromeOS) IsPopulated() bool {
	return c.Uptime > 0
}

type IOS struct {
	OSVersion  string `json:"version"`     // e.g. "13.3.1". "" if it couldn't be detected.
	DeviceType string `json:"device_type"` // e.g. "iPhone10,1"
	Battery    int    `json:"battery"`     // as integer percent, or BadBatteryLevel
}

// IsPopulated returns whether the IOS subevent record has been filled out, implying an attached iOS
// device.
func (i *IOS) IsPopulated() bool {
	return i.DeviceType != ""
}

// Standalone represents the Swarming-style dimensions of a test machine that runs tests on itself,
// not on some attached device.
//
// We may merge this into Host later, once we get consistent about dimensions referring to either
// the host or the attached device. Right now, they're a mix. Having Standalone makes it more
// obvious, in the processor flow control, which dimension values take precedence.
type Standalone struct {
	// Number of CPU cores:
	Cores int `json:"cores"`

	// Model of CPU, e.g. "arm64-64-Apple_M1" or "x86-64", in various precisions, e.g. "x86",
	// "x86-64", "x86-64-i7-9750H":
	CPUs []string `json:"cpus"`

	// Model of GPU, e.g. "1002:6821-4.0.20-3.2.8" or "8086:591e", in various precisions:
	GPUs []string `json:"gpus"`

	// OS version in various previsions, e.g. ["Mac-10", "Mac-10.15", "Mac-10.15.7"]:
	OSVersions []string `json:"os_versions"`
}

// IsPopulated returns whether the Standalone subevent record is filled out, which is the case iff a
// host is explicitly marked as having no device in the machineserver UI. IsPopulated does not
// return true when a device merely falls off a host accidentally.
func (s *Standalone) IsPopulated() bool {
	return s.Cores > 0
}

// Event is the information a machine should send via Source when its local state has changed.
type Event struct {
	EventType           EventType  `json:"type"`
	Android             Android    `json:"android"`
	ChromeOS            ChromeOS   `json:"chromeos"`
	IOS                 IOS        `json:"ios"`
	Standalone          Standalone `json:"standalone"`
	Host                Host       `json:"host"`
	RunningSwarmingTask bool       `json:"running_swarming_task"`

	// ForcedQuarantine is true if a TaskDriver or Recipe wrote a file $HOME/${MACHINE_ID}.force_quarantine.
	ForcedQuarantine bool `json:"forced_quarantine"`

	// LaunchedSwarming is true if test_machine_monitor launched Swarming.
	LaunchedSwarming bool `json:"launched_swarming"`
}

// NewEvent returns a new Event instance.
func NewEvent() Event {
	return Event{
		EventType: EventTypeRawState,
	}
}
