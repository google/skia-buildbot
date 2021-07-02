package switchboard

import (
	"context"
	"fmt"
	"math/rand"
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

	podsCollectionName = "pods"

	appName = "machineserver"

	updateTimeout = 10 * time.Second

	updateRetries = 5

	reserveRetries = 20

	// We use a range of ports from 10000-20000 when pick ports on
	// skia-switchboard pods, which is a large range to avoid collissions.
	portRangeBegin = 10_000

	portRangeEnd = 20_000
)

type metricName string

const (
	switchboardReserveMeetingpoint          = "switchboard_reserve_meetingpoint"
	switchboardReserveMeetingpointErrors    = "switchboard_reserve_meetingpoint_errors"
	switchboardClearMeetingpoint            = "switchboard_clear_meetingpoint"
	switchboardGetMeetingpoint              = "switchboard_get_meetingpoint"
	switchboardGetMeetingpointErrors        = "switchboard_get_meetingpoint_errors"
	switchboardKeepAliveMeetingpoint        = "switchboard_keepalive_meetingpoint"
	switchboardKeepAliveMeetingpointErrors  = "switchboard_keepalive_meetingpoint_errors"
	switchboardListMeetingpoint             = "switchboard_list_meetingpoint"
	switchboardListMeetingPointErrors       = "switchboard_list_meetingpoint_errors"
	switchboardNumMeetingpointsForPod       = "switchboard_num_meetingpoints_for_pod"
	switchboardNumMeetingPointsForPodErrors = "switchboard_num_meetingpoints_for_pod_errors"
	switchboardAddPod                       = "switchboard_add_pod"
	switchboardRemovePod                    = "switchboard_remove_pod"
	switchboardIsValidPod                   = "switchboard_is_valid_pod"
	switchboardKeepAlivePod                 = "switchboard_keepalive_pod"
	switchboardKeepAlivePodErrors           = "switchboard_keepalive_pod_errors"
	switchboardListPod                      = "switchboard_list_pod"
	switchboardListPodErrors                = "switchboard_list_pod_errors"
)

var (
	allMetricNames = []metricName{
		switchboardReserveMeetingpoint,
		switchboardReserveMeetingpointErrors,
		switchboardClearMeetingpoint,
		switchboardGetMeetingpoint,
		switchboardGetMeetingpointErrors,
		switchboardKeepAliveMeetingpoint,
		switchboardKeepAliveMeetingpointErrors,
		switchboardListMeetingpoint,
		switchboardListMeetingPointErrors,
		switchboardNumMeetingpointsForPod,
		switchboardNumMeetingPointsForPodErrors,
		switchboardAddPod,
		switchboardRemovePod,
		switchboardIsValidPod,
		switchboardKeepAlivePod,
		switchboardKeepAlivePodErrors,
		switchboardListPod,
		switchboardListPodErrors,
	}
)

// For both Pods and MeetingPoints we will create parallel structs
// podDesciption and meetingPointDescription that are used to store their values
// in the datastore. This will make it easier to have them diverge in the future
// if that's needed.

// podDescription is the struct that gets stored in the pods Collection.
type podDescription struct {
	LastUpdated time.Time
}

func descriptionToPod(d podDescription, name string) Pod {
	return Pod{
		Name:        name,
		LastUpdated: d.LastUpdated,
	}
}

// meetingPointDescription is the struct that gets stored in the meetingpoints Collection.
type meetingPointDescription struct {
	MeetingPoint
}

// The unique id for each MeetingPoint in the datastore is the PodName and the
// Port.
func (m meetingPointDescription) id() string {
	return fmt.Sprintf("%s:%d", m.PodName, m.Port)
}

func descriptionToMeetingPoint(m meetingPointDescription) MeetingPoint {
	return m.MeetingPoint
}

func meetingPointToDescription(m MeetingPoint) meetingPointDescription {
	return meetingPointDescription{m}
}

// switchboardImpl implements Switchboard using Cloud Firestore as a backend.
type switchboardImpl struct {
	firestoreClient         *firestore.Client
	meetingPointsCollection *gcfirestore.CollectionRef
	podsCollection          *gcfirestore.CollectionRef
	counters                map[metricName]metrics2.Counter
}

// New returns a new instance of switchboardImpl.
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
func (s *switchboardImpl) ReserveMeetingPoint(ctx context.Context, machineID string, username string) (MeetingPoint, error) {
	s.counters[switchboardReserveMeetingpoint].Inc(1)
	ret := MeetingPoint{
		LastUpdated: now.Now(ctx),
		Username:    username,
		MachineID:   machineID,
	}

	// Loop until we find an open spot for a reservation.
	//
	// We use a range of 10,000 ports per pod, and go through this loop 20
	// times, so even if we attach 100 machines to a single pod (which is a
	// lot), we have a 100/10,000 = 1/100 chance of a collision per loop, and a
	// chance of never finding an open spot is:
	//     1.0 - (1/100)^20
	//   = 1.0 - (.01)^20
	//   = 1 - 1e-40
	//   = 99.99[35 more nines go here]% chance of success.
	for i := 0; i < reserveRetries; i++ {
		pods, err := s.ListPods(ctx)
		if err != nil {
			s.counters[switchboardReserveMeetingpointErrors].Inc(1)
			sklog.Errorf("Reserve failed to load the available pods: %s", err)
			continue
		}

		if len(pods) == 0 {
			sklog.Error("No pods available")
			s.counters[switchboardReserveMeetingpointErrors].Inc(1)
			continue
		}

		// Choose a pod at random.
		pod := pods[rand.Intn(len(pods))]
		// Choose a port at random.
		port := rand.Intn(portRangeEnd-portRangeBegin) + portRangeBegin
		ret.PodName = pod.Name
		ret.Port = port

		desc := meetingPointToDescription(ret)
		docRef := s.meetingPointsCollection.Doc(desc.id())

		_, err = s.firestoreClient.Create(ctx, docRef, &desc, updateRetries, updateTimeout)
		if err != nil {
			sklog.Errorf("Failed to create: %s", err)
			s.counters[switchboardReserveMeetingpointErrors].Inc(1)
			continue
		}
		return ret, nil
	}

	return ret, ErrNoPodsFound
}

// ClearMeetingPoint implements Switchboard
func (s *switchboardImpl) ClearMeetingPoint(ctx context.Context, meetingPoint MeetingPoint) error {
	s.counters[switchboardClearMeetingpoint].Inc(1)
	_, err := s.meetingPointsCollection.Doc(meetingPointToDescription(meetingPoint).id()).Delete(ctx)
	return err
}

// GetMeetingPoint implements Switchboard
func (s *switchboardImpl) GetMeetingPoint(ctx context.Context, machineID string) (MeetingPoint, error) {
	s.counters[switchboardGetMeetingpoint].Inc(1)
	iter := s.meetingPointsCollection.Where("MachineID", "==", machineID).OrderBy("LastUpdated", gcfirestore.Desc).Documents(ctx)

	var ret MeetingPoint
	snap, err := iter.Next()
	if err == iterator.Done {
		s.counters[switchboardGetMeetingpointErrors].Inc(1)
		return ret, ErrMachineNotFound
	}
	if err != nil {
		s.counters[switchboardGetMeetingpointErrors].Inc(1)
		return ret, skerr.Wrapf(err, "List failed to read description.")
	}
	var desc meetingPointDescription
	if err := snap.DataTo(&desc); err != nil {
		s.counters[switchboardGetMeetingpointErrors].Inc(1)
		sklog.Errorf("Failed to read data from snapshot: %s", err)
		return ret, skerr.Wrapf(err, "List failed to load description.")
	}
	ret = descriptionToMeetingPoint(desc)

	return ret, nil
}

// KeepAliveMeetingPoint implements Switchboard
func (s *switchboardImpl) KeepAliveMeetingPoint(ctx context.Context, meetingPoint MeetingPoint) error {
	s.counters[switchboardKeepAliveMeetingpoint].Inc(1)
	desc := meetingPointToDescription(meetingPoint)
	docRef := s.meetingPointsCollection.Doc(desc.id())
	return s.firestoreClient.RunTransaction(ctx, "meetingpoints", "keepalive", updateRetries, updateTimeout, func(ctx context.Context, tx *gcfirestore.Transaction) error {
		snap, err := tx.Get(docRef)
		if err != nil {
			s.counters[switchboardKeepAliveMeetingpointErrors].Inc(1)
			return skerr.Wrapf(err, "Failed querying firestore for %q", desc.id())
		}
		if err := snap.DataTo(&desc); err != nil {
			s.counters[switchboardKeepAliveMeetingpointErrors].Inc(1)
			return skerr.Wrapf(err, "Failed to deserialize firestore Get response for %q", desc.id())
		}
		desc.LastUpdated = now.Now(ctx)

		return tx.Set(docRef, &desc)
	})
}

// AddPod implements Switchboard
func (s *switchboardImpl) AddPod(ctx context.Context, podName string) error {
	s.counters[switchboardAddPod].Inc(1)
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
	s.counters[switchboardKeepAlivePod].Inc(1)
	docRef := s.podsCollection.Doc(podName)
	return s.firestoreClient.RunTransaction(ctx, "pods", "keepalive", updateRetries, updateTimeout, func(ctx context.Context, tx *gcfirestore.Transaction) error {
		var podDescription podDescription
		snap, err := tx.Get(docRef)
		if err != nil {
			s.counters[switchboardKeepAlivePodErrors].Inc(1)
			return skerr.Wrapf(err, "Failed querying firestore for %q", podName)
		}
		if err := snap.DataTo(&podDescription); err != nil {
			s.counters[switchboardKeepAlivePodErrors].Inc(1)
			return skerr.Wrapf(err, "Failed to deserialize firestore Get response for %q", podName)
		}
		podDescription.LastUpdated = now.Now(ctx)

		return tx.Set(docRef, &podDescription)
	})
}

// RemovePod implements Switchboard
func (s *switchboardImpl) RemovePod(ctx context.Context, podName string) error {
	s.counters[switchboardRemovePod].Inc(1)
	_, err := s.podsCollection.Doc(podName).Delete(ctx)
	return err
}

// IsValidPod implements Switchboard
func (s *switchboardImpl) IsValidPod(ctx context.Context, podName string) bool {
	s.counters[switchboardIsValidPod].Inc(1)
	_, err := s.podsCollection.Doc(podName).Get(ctx)
	return err == nil
}

// ListPods implements Switchboard
func (s *switchboardImpl) ListPods(ctx context.Context) ([]Pod, error) {
	s.counters[switchboardListPod].Inc(1)

	ret := []Pod{}
	iter := s.podsCollection.Documents(ctx)
	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.counters[switchboardListPodErrors].Inc(1)
			return nil, skerr.Wrapf(err, "List failed to read description.")
		}
		var podDescription podDescription
		if err := snap.DataTo(&podDescription); err != nil {
			s.counters[switchboardListPodErrors].Inc(1)
			sklog.Errorf("Failed to read data from snapshot: %s", err)
			continue
		}

		ret = append(ret, descriptionToPod(podDescription, snap.Ref.ID))
	}
	return ret, nil
}

// ListMeetingPoints implements Switchboard
func (s *switchboardImpl) ListMeetingPoints(ctx context.Context) ([]MeetingPoint, error) {
	s.counters[switchboardListMeetingpoint].Inc(1)

	ret := []MeetingPoint{}
	iter := s.meetingPointsCollection.Documents(ctx)
	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.counters[switchboardListMeetingPointErrors].Inc(1)
			return nil, skerr.Wrapf(err, "failed to iterate")
		}
		var meetingPointDescription meetingPointDescription
		if err := snap.DataTo(&meetingPointDescription); err != nil {
			s.counters[switchboardListMeetingPointErrors].Inc(1)
			sklog.Errorf("Failed to read data from snapshot: %s", err)
			continue
		}

		ret = append(ret, descriptionToMeetingPoint(meetingPointDescription))
	}
	return ret, nil

}

// NumMeetingPointsForPod implements Switchboard.
func (s *switchboardImpl) NumMeetingPointsForPod(ctx context.Context, podName string) (int, error) {
	s.counters[switchboardNumMeetingpointsForPod].Inc(1)

	count := 0
	iter := s.meetingPointsCollection.Where("PodName", "==", podName).Documents(ctx)
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.counters[switchboardNumMeetingPointsForPodErrors].Inc(1)
			return 0, skerr.Wrapf(err, "failed to iterate for podName: %q", podName)
		}
		count++
	}
	return count, nil
}

// Assert that switchboardImpl implements the Switchboard interface.
var _ Switchboard = (*switchboardImpl)(nil)
