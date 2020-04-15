package store

import (
	"context"
	"time"

	gcfirestore "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	machinesCollectionName = "machines"

	appName = "machineserver"

	updateTimeout = 10 * time.Second

	updateRetries = 5
)

// StoreImpl implements the Store interface.
type StoreImpl struct {
	firestoreClient    *firestore.Client
	machinesCollection *gcfirestore.CollectionRef
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

	// MachineDescription is the full machine.Description. The values that are
	// mirrored to fields of storeDescription are still fully stored here and
	// are considered the source of truth.
	MachineDescription machine.Description
}

// New returns a new instance of StoreImpl.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*StoreImpl, error) {
	ts, err := auth.NewDefaultTokenSource(local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, err
	}

	firestoreClient, err := firestore.NewClient(ctx, firestore.FIRESTORE_PROJECT, appName, instanceConfig.Store.Instance, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create firestore client for app: %q instance: %q", appName, instanceConfig.Store.Instance)
	}
	return &StoreImpl{
		firestoreClient:    firestoreClient,
		machinesCollection: firestoreClient.Collection(machinesCollectionName),
	}, nil
}

// Update implements the Store interface.
func (st *StoreImpl) Update(ctx context.Context, machineID string, txCallback TxCallback) error {
	docRef := st.machinesCollection.Doc(machineID)
	return st.firestoreClient.RunTransaction(ctx, "store", "update", updateRetries, updateTimeout, func(ctx context.Context, tx *gcfirestore.Transaction) error {
		var storeDescription storeDescription
		machineDescription := machine.NewDescription()
		if snap, err := tx.Get(docRef); err == nil {
			if err := snap.DataTo(&storeDescription); err != nil {
				return err
			}
			machineDescription = storeToMachineDescription(storeDescription)
		} else if st, ok := status.FromError(err); ok && st.Code() != codes.NotFound {
			return err
		}

		updatedMachineDescription := txCallback(machineDescription)
		updatedStoreDescription := machineDescriptionToStoreDescription(updatedMachineDescription)

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
				} else if st, ok := status.FromError(err); ok && st.Code() == codes.Canceled {
					sklog.Warningf("Context canceled; closing channel: %s", err)
				} else {
					sklog.Errorf("iter returned error; closing channel: %s", err)
				}
				iter.Stop()
				close(ch)
				return
			}
			var m machine.Description
			if err := snap.DataTo(&m); err != nil {
				sklog.Errorf("Failed to read data from snapshot: %s", err)
				continue
			}
			ch <- m
		}
	}()
	return ch
}

func machineDescriptionToStoreDescription(m machine.Description) storeDescription {
	return storeDescription{
		OS:                 m.Dimensions["os"],
		DeviceType:         m.Dimensions["device_type"],
		Quarantined:        m.Dimensions["quarantined"],
		Mode:               m.Mode,
		LastUpdated:        m.LastUpdated,
		MachineDescription: m,
	}
}

func storeToMachineDescription(s storeDescription) machine.Description {
	return s.MachineDescription
}

// Affirm that StoreImpl implements the Store interface.
var _ Store = (*StoreImpl)(nil)
