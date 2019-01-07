package periodic

import (
	"context"
	"fmt"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Firestore instance name.
	FIRESTORE_INSTANCE = "task-scheduler-periodic"

	// Collection name for periodic trigger entries.
	COLLECTION_PERIODIC_TRIGGERS = "periodic-triggers"

	// Sub-collection name for individual instances of periodic triggers.
	SUBCOLLECTION_ENTRIES = "entries"

	// PERIODIC_TRIGGER_ENTRY_NAME_FORMAT is the timestamp format used to
	// derive names for entries.
	// TODO(borenet): What if we want hourly (or other smaller-than-day)
	// periods in the future? Maybe the cron job should generate the name?
	PERIODIC_TRIGGER_ENTRY_NAME_FORMAT = "2006-01-02"

	// PERIODIC_TRIGGER_MEASUREMENT is the name of the liveness metric for
	// periodic triggers.
	PERIODIC_TRIGGER_MEASUREMENT = "periodic_trigger"

	// PERIODIC_TRIGGER_METRICS_UPDATE_MEASUREMENT is the name of the
	// liveness metric for the metrics self-update for the periodic
	// triggers.
	PERIODIC_TRIGGER_METRICS_UPDATE_MEASUREMENT = "periodic_trigger_metrics_update"
)

type periodicEntry struct {
	Timestamp time.Time `firestore:"timestamp"`
}

// Periodic is a struct used for synchronizing periodic triggers across multiple
// schedulers.
type Periodic struct {
	lv     map[string]metrics2.Liveness
	client *firestore.Client
	colls  map[string]*fs.CollectionRef
}

// New returns an instance of Periodic.
func New(ctx context.Context, project, instance string, ts oauth2.TokenSource) (*Periodic, error) {
	client, err := firestore.NewClient(ctx, project, firestore.APP_TASK_SCHEDULER, instance, ts)
	if err != nil {
		return nil, err
	}
	coll := client.Collection(COLLECTION_PERIODIC_TRIGGERS)
	colls := make(map[string]*fs.CollectionRef, len(specs.PERIODIC_TRIGGERS))
	lv := make(map[string]metrics2.Liveness, len(specs.PERIODIC_TRIGGERS))
	for _, t := range specs.PERIODIC_TRIGGERS {
		lv[t] = metrics2.NewLiveness(PERIODIC_TRIGGER_MEASUREMENT, map[string]string{
			"trigger": t,
		})
		colls[t] = coll.Doc(t).Collection(SUBCOLLECTION_ENTRIES)
	}
	rv := &Periodic{
		lv:     lv,
		client: client,
		colls:  colls,
	}
	if err := rv.update(ctx); err != nil {
		return nil, err
	}
	return rv, nil
}

func (p *Periodic) update(ctx context.Context) error {
	// Update our local metrics based on the most recent entries in Firestore.
	for _, t := range specs.PERIODIC_TRIGGERS {
		if err := firestore.IterDocs(p.colls[t].OrderBy("timestamp", fs.Desc).Limit(1), 3, time.Minute, func(d *fs.DocumentSnapshot) error {
			var e periodicEntry
			if err := d.DataTo(&e); err != nil {
				return err
			}
			p.lv[t].ManualReset(e.Timestamp)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for db.DBCloser interface.
func (p *Periodic) Close() error {
	// TODO(borenet): Should Close() also stop the metrics self-update loop?
	return p.client.Close()
}

// Start begins the periodic trigger metrics self-update loop.
func (p *Periodic) Start(ctx context.Context) {
	lv := metrics2.NewLiveness(PERIODIC_TRIGGER_METRICS_UPDATE_MEASUREMENT)
	go util.RepeatCtx(time.Minute, ctx, func() {
		if err := p.update(ctx); err != nil {
			sklog.Errorf("Failed to update periodic trigger metrics: %s", err)
		} else {
			lv.Reset()
		}
	})
}

// MaybeTrigger wraps the given function, only calling it if the given periodic
// trigger has not yet run. If the given function returns an error, then
// MaybeTrigger also returns an error and the function is allowed to run again
// on subsequent calls.
func (p *Periodic) MaybeTrigger(ctx context.Context, trigger string, fn func() error) (rvErr error) {
	if !util.In(trigger, specs.PERIODIC_TRIGGERS) {
		return fmt.Errorf("Unknown trigger name %q", trigger)
	}
	now := time.Now().UTC()
	name := now.Format(PERIODIC_TRIGGER_ENTRY_NAME_FORMAT)
	ref := p.colls[trigger].Doc(name)
	e := periodicEntry{
		Timestamp: now,
	}
	// Note that, if one caller creates the entry and is running the
	// passed-in function, and another caller attempts to create the entry,
	// it will fail to do so, assuming that the first caller has already
	// performed the work. If the first caller fails during the passed-in
	// function, the second caller will never be able to attempt a retry.
	if _, err := firestore.Create(ref, &e, 3, time.Minute); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return nil
		}
		return err
	}
	defer func() {
		if rvErr != nil {
			if _, err := firestore.Delete(ref, 3, time.Minute); err != nil {
				sklog.Errorf("Periodic trigger callback failed, and failed to delete trigger entry; periodic jobs will not run: %s", err)
			}
		}
	}()
	return fn()
}
