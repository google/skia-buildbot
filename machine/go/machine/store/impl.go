package store

import (
	"context"
	"math/rand"
	"time"

	gcfirestore "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	machinesCollectionName = "machines"

	appName = "machineserver"

	updateTimeout = 10 * time.Second

	updateRetries = 5
)

var (
	// The amount of time, in seconds, at most, before retrying the query in Watch.
	watchRecoverBackoff int64 = 6
)

// StoreImpl implements the Store interface.
type StoreImpl struct {
	firestoreClient    *firestore.Client
	machinesCollection *gcfirestore.CollectionRef

	updateCounter                               metrics2.Counter
	updateDataToErrorCounter                    metrics2.Counter
	watchReceiveSnapshotCounter                 metrics2.Counter
	watchDataToErrorCounter                     metrics2.Counter
	listCounter                                 metrics2.Counter
	deleteCounter                               metrics2.Counter
	listIterFailureCounter                      metrics2.Counter
	watchForDeletablePodsReceiveSnapshotCounter metrics2.Counter
	watchForDeletablePodsDataToErrorCounter     metrics2.Counter
	watchForPowerCycleReceiveSnapshotCounter    metrics2.Counter
	watchForPowerCycleDataToErrorCounter        metrics2.Counter
}

// storeDescription is how machine.Description is mapped into firestore.
//
// Some fields from machine.Description are mirrored to top level
// storeDescription fields so we can query on them.
type storeDescription struct {
	// OS is a mirror of MachineDescription.Dimensions["os"].
	OS []string

	// OS is a mirror of MachineDescription.Dimensions["device_type"].
	DeviceType []string

	// OS is a mirror of MachineDescription.Dimensions["quarantined"].
	Quarantined []string

	// Mode is a mirror of MachineDescription.Mode.
	Mode machine.Mode

	// LastUpdated is a mirror of MachineDescription.LastUpdated.
	LastUpdated time.Time

	// ScheduledForDeletion is a mirror of MachineDescription.ScheduledForDeletion.
	ScheduledForDeletion string

	// RunningSwarmingTask is a mirror of MachineDescription.RunningSwarmingTask.
	RunningSwarmingTask bool

	// PowerCycle is a mirror of MachineDescription.PowerCycle.
	PowerCycle bool

	// MachineDescription is the full machine.Description. The values that are
	// mirrored to fields of storeDescription are still fully stored here and
	// are considered the source of truth.
	MachineDescription fsMachineDescription
}

// fsMachineDescription models how machine.Description is stored in Firestore. This serves to
// decouple the schema stored in FS from the schema used elsewhere.
type fsMachineDescription struct {
	Mode machine.Mode

	// Annotation is used to record the most recent user change to Description.
	// This will be in addition to the normal auditlog of user actions:
	// https://pkg.go.dev/go.skia.org/infra/go/auditlog?tab=doc
	Annotation fsAnnotation

	// Note is a user authored message on the state of a machine.
	Note fsAnnotation

	Dimensions machine.SwarmingDimensions
	PodName    string

	// KubernetesImage is the kubernetes image name.
	KubernetesImage string

	// Version of test_machine_monitor being run.
	Version string

	// ScheduledForDeletion will be a non-empty string and equal to PodName if
	// the pod should be deleted.
	ScheduledForDeletion string

	// PowerCycle is true if the machine needs to be power-cycled.
	PowerCycle bool

	LastUpdated         time.Time
	Battery             int                // Charge as an integer percent, e.g. 50% = 50.
	Temperature         map[string]float64 // In Celsius.
	RunningSwarmingTask bool
	LaunchedSwarming    bool      // True if test_machine_monitor launched Swarming.
	RecoveryStart       time.Time // When did the machine start being in recovery mode.
	DeviceUptime        int32     // Seconds
}

// fsAnnotation models how machine.Annotation is stored in Firestore. This serves to
// decouple the schema stored in FS from the schema used elsewhere.
type fsAnnotation struct {
	Message   string
	User      string
	Timestamp time.Time
}

// New returns a new instance of StoreImpl that is backed by Firestore.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*StoreImpl, error) {
	ts, err := auth.NewDefaultTokenSource(local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create tokensource.")
	}

	firestoreClient, err := firestore.NewClient(ctx, instanceConfig.Store.Project, appName, instanceConfig.Store.Instance, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create firestore client for app: %q instance: %q", appName, instanceConfig.Store.Instance)
	}
	return &StoreImpl{
		firestoreClient:                             firestoreClient,
		machinesCollection:                          firestoreClient.Collection(machinesCollectionName),
		updateCounter:                               metrics2.GetCounter("machine_store_update"),
		updateDataToErrorCounter:                    metrics2.GetCounter("machine_store_update_datato_error"),
		watchReceiveSnapshotCounter:                 metrics2.GetCounter("machine_store_watch_receive_snapshot"),
		watchDataToErrorCounter:                     metrics2.GetCounter("machine_store_watch_datato_error"),
		listCounter:                                 metrics2.GetCounter("machine_store_list"),
		deleteCounter:                               metrics2.GetCounter("machine_store_delete"),
		listIterFailureCounter:                      metrics2.GetCounter("machine_store_list_iter_error"),
		watchForDeletablePodsReceiveSnapshotCounter: metrics2.GetCounter("machine_store_watch_for_deletable_pods_receive_snapshot"),
		watchForDeletablePodsDataToErrorCounter:     metrics2.GetCounter("machine_store_watch_for_deletable_pods_datato_error"),
		watchForPowerCycleReceiveSnapshotCounter:    metrics2.GetCounter("machine_store_watch_for_power_cycle_receive_snapshot"),
		watchForPowerCycleDataToErrorCounter:        metrics2.GetCounter("machine_store_watch_for_power_cycle_datato_error"),
	}, nil
}

// Update implements the Store interface.
func (st *StoreImpl) Update(ctx context.Context, machineID string, updateCallback UpdateCallback) error {
	st.updateCounter.Inc(1)
	docRef := st.machinesCollection.Doc(machineID)
	return st.firestoreClient.RunTransaction(ctx, "store", "update", updateRetries, updateTimeout, func(ctx context.Context, tx *gcfirestore.Transaction) error {
		var storeDescription storeDescription
		machineDescription := machine.NewDescription(ctx)
		machineDescription.Dimensions[machine.DimID] = []string{machineID}
		if snap, err := tx.Get(docRef); err == nil {
			if err := snap.DataTo(&storeDescription); err != nil {
				st.updateDataToErrorCounter.Inc(1)
				return skerr.Wrapf(err, "Failed to deserialize firestore Get response for %q", machineID)
			}
			machineDescription = convertFSDescription(storeDescription)
		} else if st, ok := status.FromError(err); ok && st.Code() != codes.NotFound {
			return skerr.Wrapf(err, "Failed querying firestore for %q", machineID)
		}

		updatedMachineDescription := updateCallback(machineDescription)
		updatedStoreDescription := convertDescription(updatedMachineDescription)

		return tx.Set(docRef, &updatedStoreDescription)
	})
}

// Watch implements the Store interface.
func (st *StoreImpl) Watch(ctx context.Context, machineID string) <-chan machine.Description {
	iter := st.machinesCollection.Doc(machineID).Snapshots(ctx)
	ch := make(chan machine.Description)
	go func() {
		for {
			snap, err := iter.Next()
			if err != nil {
				if ctx.Err() == context.Canceled {
					sklog.Warningf("Context canceled; closing channel: %s", err)
				} else if stErr, ok := status.FromError(err); ok && stErr.Code() == codes.Canceled {
					sklog.Warningf("Context canceled; closing channel: %s", err)
				} else {
					iter.Stop()
					time.Sleep(time.Second * time.Duration(rand.Int63n(watchRecoverBackoff)))
					iter = st.machinesCollection.Doc(machineID).Snapshots(ctx)
					sklog.Warningf("iter returned error; retrying query: %s", err)
					continue
				}
				iter.Stop()
				close(ch)
				return
			}
			if !snap.Exists() {
				continue
			}
			var storeDescription storeDescription
			if err := snap.DataTo(&storeDescription); err != nil {
				sklog.Errorf("Failed to read data from snapshot: %s", err)
				st.watchDataToErrorCounter.Inc(1)
				continue
			}
			machineDescription := convertFSDescription(storeDescription)
			st.watchReceiveSnapshotCounter.Inc(1)
			ch <- machineDescription
		}
	}()
	return ch
}

// WatchForDeletablePods implements the Store interface.
func (st *StoreImpl) WatchForDeletablePods(ctx context.Context) <-chan string {
	q := st.machinesCollection.Where("ScheduledForDeletion", ">", "").Where("RunningSwarmingTask", "==", false)
	ch := make(chan string)
	go func() {
		defer close(ch)
		for qsnap := range firestore.QuerySnapshotChannel(ctx, q) {
			for {
				snap, err := qsnap.Documents.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					sklog.Errorf("Failed to read document snapshot: %s", err)
					continue
				}
				var storeDescription storeDescription
				if err := snap.DataTo(&storeDescription); err != nil {
					sklog.Errorf("Failed to read data from snapshot: %s", err)
					st.watchForDeletablePodsDataToErrorCounter.Inc(1)
					continue
				}
				machineDescription := convertFSDescription(storeDescription)
				st.watchForDeletablePodsReceiveSnapshotCounter.Inc(1)
				ch <- machineDescription.PodName
			}
		}
	}()
	return ch
}

// WatchForPowerCycle implements the Store interface.
func (st *StoreImpl) WatchForPowerCycle(ctx context.Context) <-chan string {
	q := st.machinesCollection.Where("PowerCycle", "==", true).Where("RunningSwarmingTask", "==", false)
	ch := make(chan string)
	go func() {
		defer close(ch)
		for qsnap := range firestore.QuerySnapshotChannel(ctx, q) {
			for {
				snap, err := qsnap.Documents.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					sklog.Errorf("Failed to read document snapshot: %s", err)
					continue
				}
				var storeDescription storeDescription
				if err := snap.DataTo(&storeDescription); err != nil {
					sklog.Errorf("Failed to read data from snapshot: %s", err)
					st.watchForPowerCycleDataToErrorCounter.Inc(1)
					continue
				}
				machineDescription := convertFSDescription(storeDescription)
				st.watchForPowerCycleReceiveSnapshotCounter.Inc(1)
				machineID := machineDescription.Dimensions[machine.DimID][0]
				err = st.Update(ctx, machineID, func(previous machine.Description) machine.Description {
					ret := previous.Copy()
					ret.PowerCycle = false
					return ret
				})
				if err != nil {
					sklog.Errorf("Failed to update machine.Description PowerCycle: %s", err)
					// Just log the error, still powercycle the machine.
				}
				ch <- machineID
			}
		}
	}()
	return ch
}

// List implements the Store interface.
func (st *StoreImpl) List(ctx context.Context) ([]machine.Description, error) {
	st.listCounter.Inc(1)
	ret := []machine.Description{}
	iter := st.machinesCollection.Documents(ctx)
	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			st.listIterFailureCounter.Inc(1)
			return nil, skerr.Wrapf(err, "List failed to read description.")
		}

		var storeDescription storeDescription
		if err := snap.DataTo(&storeDescription); err != nil {
			st.listIterFailureCounter.Inc(1)
			sklog.Errorf("Failed to read data from snapshot: %s", err)
			continue
		}
		machineDescription := convertFSDescription(storeDescription)
		ret = append(ret, machineDescription)
	}
	return ret, nil
}

// Delete implements the Store interface.
func (st *StoreImpl) Delete(ctx context.Context, machineID string) error {
	st.deleteCounter.Inc(1)

	_, err := st.machinesCollection.Doc(machineID).Delete(ctx)
	return err
}

func convertDescription(m machine.Description) storeDescription {
	return storeDescription{
		OS:                   m.Dimensions[machine.DimOS],
		DeviceType:           m.Dimensions[machine.DimDeviceType],
		Quarantined:          m.Dimensions[machine.DimQuarantined],
		Mode:                 m.Mode,
		LastUpdated:          m.LastUpdated,
		ScheduledForDeletion: m.ScheduledForDeletion,
		RunningSwarmingTask:  m.RunningSwarmingTask,
		PowerCycle:           m.PowerCycle,
		MachineDescription: fsMachineDescription{
			Mode:                 m.Mode,
			Annotation:           convertAnnotation(m.Annotation),
			Note:                 convertAnnotation(m.Note),
			Dimensions:           m.Dimensions,
			PodName:              m.PodName,
			KubernetesImage:      m.KubernetesImage,
			Version:              m.Version,
			ScheduledForDeletion: m.ScheduledForDeletion,
			PowerCycle:           m.PowerCycle,
			LastUpdated:          m.LastUpdated,
			Battery:              m.Battery,
			Temperature:          m.Temperature,
			RunningSwarmingTask:  m.RunningSwarmingTask,
			LaunchedSwarming:     m.LaunchedSwarming,
			RecoveryStart:        m.RecoveryStart,
			DeviceUptime:         m.DeviceUptime,
		},
	}
}

func convertAnnotation(a machine.Annotation) fsAnnotation {
	return fsAnnotation{
		Message:   a.Message,
		User:      a.User,
		Timestamp: a.Timestamp,
	}
}

func convertFSAnnotation(a fsAnnotation) machine.Annotation {
	return machine.Annotation{
		Message:   a.Message,
		User:      a.User,
		Timestamp: a.Timestamp,
	}
}

// convertFSDescription converts the firestore version of the description to the common format.
func convertFSDescription(s storeDescription) machine.Description {
	m := s.MachineDescription
	return machine.Description{
		Mode:                 m.Mode,
		Annotation:           convertFSAnnotation(m.Annotation),
		Note:                 convertFSAnnotation(m.Note),
		Dimensions:           m.Dimensions,
		PodName:              m.PodName,
		KubernetesImage:      m.KubernetesImage,
		Version:              m.Version,
		ScheduledForDeletion: m.ScheduledForDeletion,
		PowerCycle:           m.PowerCycle,
		LastUpdated:          m.LastUpdated,
		Battery:              m.Battery,
		Temperature:          m.Temperature,
		RunningSwarmingTask:  m.RunningSwarmingTask,
		LaunchedSwarming:     m.LaunchedSwarming,
		RecoveryStart:        m.RecoveryStart,
		DeviceUptime:         m.DeviceUptime,
	}
}

// Affirm that StoreImpl implements the Store interface.
var _ Store = (*StoreImpl)(nil)
