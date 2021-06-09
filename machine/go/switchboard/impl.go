package switchboard

import (
	"context"
	"time"

	gcfirestore "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machineserver/config"
	"google.golang.org/api/iterator"
)

const (
	meetingPointsCollectionName = "meetingpoints"
	podsCollectionName          = "pods"

	appName = "machineserver"

	updateTimeout = 10 * time.Second

	updateRetries = 5
)

// podDescription is the struct that gets stored in the pods Collection.
type podDescription struct {
	LastUpdated time.Time
}

type metricName string

const (
	switchboard_reserve_meetingpoint   = "switchboard_reserve_meetingpoint"
	switchboard_clear_meetingpoint     = "switchboard_clear_meetingpoint"
	switchboard_get_meetingpoint       = "switchboard_get_meetingpoint"
	switchboard_keepalive_meetingpoint = "switchboard_keepalive_meetingpoint"
	switchboard_list_meetingpoint      = "switchboard_list_meetingpoint"
	switchboard_add_pod                = "switchboard_add_pod"
	switchboard_remove_pod             = "switchboard_remove_pod"
	switchboard_keepalive_pod          = "switchboard_keepalive_pod"
	switchboard_keepalive_pod_errors   = "switchboard_keepalive_pod_errors"
	switchboard_list_pod               = "switchboard_list_pod"
	switchboard_list_pod_errors        = "switchboard_list_pod_errors"
)

var (
	allMetricNames = []metricName{
		switchboard_reserve_meetingpoint,
		switchboard_clear_meetingpoint,
		switchboard_get_meetingpoint,
		switchboard_keepalive_meetingpoint,
		switchboard_list_meetingpoint,
		switchboard_add_pod,
		switchboard_remove_pod,
		switchboard_keepalive_pod,
		switchboard_keepalive_pod_errors,
		switchboard_list_pod,
		switchboard_list_pod_errors,
	}
)

// switchboardImpl implements Switchboard using Cloud Firestore as a backend.
type switchboardImpl struct {
	firestoreClient         *firestore.Client
	meetingPointsCollection *gcfirestore.CollectionRef
	podsCollection          *gcfirestore.CollectionRef

	counters map[metricName]metrics2.Counter
}

func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*switchboardImpl, error) {
	ts, err := auth.NewDefaultTokenSource(local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create tokensource.")
	}

	firestoreClient, err := firestore.NewClient(ctx, instanceConfig.Store.Project, appName, instanceConfig.Store.Instance, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create firestore client for app: %q instance: %q", appName, instanceConfig.Store.Instance)
	}

	counters := map[metricName]metrics2.Counter{}
	for _, metricName := range allMetricNames {
		counters[metricName] = metrics2.GetCounter(string(metricName))
	}

	return &switchboardImpl{
		firestoreClient:         firestoreClient,
		meetingPointsCollection: firestoreClient.Collection(meetingPointsCollectionName),
		podsCollection:          firestoreClient.Collection(podsCollectionName),
		counters:                counters,
	}, nil
}

// ReserveMeetingPoint implements Switchboard
func (s *switchboardImpl) ReserveMeetingPoint(ctx context.Context, machineID string, Username string) (MeetingPoint, error) {
	panic("unimplemented")
}

// ClearMeetingPoint implements Switchboard
func (s *switchboardImpl) ClearMeetingPoint(ctx context.Context, meeingPoint MeetingPoint) error {
	panic("unimplemented")
}

// GetMeetingPoint implements Switchboard
func (s *switchboardImpl) GetMeetingPoint(ctx context.Context, machineID string) (MeetingPoint, error) {
	panic("unimplemented")
}

// KeepAliveMeetingPoint implements Switchboard
func (s *switchboardImpl) KeepAliveMeetingPoint(ctx context.Context, meetingPoint MeetingPoint) error {
	panic("unimplemented")
}

// AddPod implements Switchboard
func (s *switchboardImpl) AddPod(ctx context.Context, podName string) error {
	s.counters[switchboard_add_pod].Inc(1)
	docRef := s.podsCollection.Doc(podName)
	return s.firestoreClient.RunTransaction(ctx, "pods", "add", updateRetries, updateTimeout, func(ctx context.Context, tx *gcfirestore.Transaction) error {
		podDescription := podDescription{
			LastUpdated: now.Now(ctx),
		}
		return tx.Create(docRef, &podDescription)
	})
}

// KeepAlivePod implements Switchboard
func (s *switchboardImpl) KeepAlivePod(ctx context.Context, podName string) error {
	s.counters[switchboard_keepalive_pod].Inc(1)
	docRef := s.podsCollection.Doc(podName)
	return s.firestoreClient.RunTransaction(ctx, "pods", "keepalive", updateRetries, updateTimeout, func(ctx context.Context, tx *gcfirestore.Transaction) error {
		var podDescription podDescription
		snap, err := tx.Get(docRef)
		if err != nil {
			s.counters[switchboard_keepalive_pod_errors].Inc(1)
			return skerr.Wrapf(err, "Failed querying firestore for %q", podName)
		}
		if err := snap.DataTo(&podDescription); err != nil {
			s.counters[switchboard_keepalive_pod_errors].Inc(1)
			return skerr.Wrapf(err, "Failed to deserialize firestore Get response for %q", podName)
		}
		podDescription.LastUpdated = now.Now(ctx)

		return tx.Set(docRef, &podDescription)
	})
}

// RemovePod implements Switchboard
func (s *switchboardImpl) RemovePod(ctx context.Context, podName string) error {
	s.counters[switchboard_remove_pod].Inc(1)
	_, err := s.podsCollection.Doc(podName).Delete(ctx)
	return err
}

// ListPods implements Switchboard
func (s *switchboardImpl) ListPods(ctx context.Context) ([]Pod, error) {
	s.counters[switchboard_list_pod].Inc(1)

	ret := []Pod{}
	iter := s.podsCollection.Documents(ctx)
	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.counters[switchboard_list_pod_errors].Inc(1)
			return nil, skerr.Wrapf(err, "List failed to read description.")
		}
		var podDescription podDescription
		if err := snap.DataTo(&podDescription); err != nil {
			s.counters[switchboard_list_pod_errors].Inc(1)
			sklog.Errorf("Failed to read data from snapshot: %s", err)
			continue
		}

		ret = append(ret, Pod{
			Name:        snap.Ref.ID,
			LastUpdated: podDescription.LastUpdated,
		})
	}
	return ret, nil

}

// ListMeetingPoints implements Switchboard
func (s *switchboardImpl) ListMeetingPoints(ctx context.Context) ([]MeetingPoint, error) {
	panic("unimplemented")
}

// Assert that switchboardImpl implementst the Switchboard interface.
var _ Switchboard = (*switchboardImpl)(nil)
