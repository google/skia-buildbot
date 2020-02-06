package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/logging"
	"cloud.google.com/go/pubsub"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

func main() {
	var (
		name             = flag.String("id", "gce-0001", "machine id")
		pool             = flag.String("pool", "skia", "pool this machine is in")
		project          = flag.String("project", "skia-public", "project this pool belongs to")
		firestoreProject = flag.String("firestore_project", "skia-firestore", "project where Firestore data lives")
	)
	flag.Parse()
	ifirestore.EnsureNotEmulator()
	fmt.Printf("hello emulated machine %s\n", *name)

	// Auth note: the underlying cloud clients look at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source. We might, if we don't go with the
	// GOOGLE_APPLICATION_CREDENTIALS and use metadata server or whatever.

	// Initialize Cloud Logging.
	labels := map[string]string{
		"machineID": *name,
		"logSource": "machine_daemon",
	}
	ctx := context.Background()
	logger, err := sklog.NewCloudLogger(ctx, *project, "machines", nil, labels)
	if err != nil {
		fmt.Printf("could not create logger: %s\n", err)
		os.Exit(1)
	}
	sklog.SetLogger(logger)

	fsClient, err := ifirestore.NewClient(ctx, *firestoreProject, "niagara", "testing", nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	sklog.Infof("Firestore initialized")

	psClient, err := pubsub.NewClient(ctx, *project)
	if err != nil {
		sklog.Fatalf("Unable to configure Pubsub: %s", err)
	}
	topic, err := setupTopic(ctx, psClient, "niagara-machines-"+*pool)
	if err != nil {
		sklog.Fatalf("Unable to setup subscription: %s", err)
	}

	m := emulatedMachine{
		id:       *name,
		topic:    topic,
		fsClient: fsClient,
		logger:   logger.Logger(), // might be useful
	}

	sklog.Fatalf("Error running emulated machine %v", m.emulateMachine(ctx))
}

type emulatedMachine struct {
	id       string
	fsClient *ifirestore.Client
	topic    *pubsub.Topic
	logger   *logging.Logger
}

func (m *emulatedMachine) emulateMachine(ctx context.Context) error {
	if err := m.sendEvent(ctx, m.getDescription(), map[string]string{machine.EventAttribute: string(machine.Booted)}); err != nil {
		return skerr.Wrapf(err, "sending booted message")
	}
	// TODO(kjlubick) start a health check loop
	// TODO(kjlubick) listen to sigint to send rebooting or something

	q := m.fsClient.Collection(fs_entries.TasksCollection).Where(fs_entries.TaskMachineAssignedField, "==", m.id).
		Where(fs_entries.TaskStatusField, "==", task.New).Limit(1)
	snap := q.Snapshots(ctx)
	for {
		if err := ctx.Err(); err != nil {
			sklog.Debugf("Stopping due to context error: %s", err)
			snap.Stop()
			return skerr.Wrap(err)
		}
		qs, err := snap.Next()
		if err != nil {
			return skerr.Wrap(err)
		}
		// In an ideal world, there will only be one task in a snapshot
		for _, dc := range qs.Changes {
			id := dc.Doc.Ref.ID
			if dc.Kind == firestore.DocumentRemoved {
				// This happens any time a task that was assigned to this machine is marked
				// as finished and no longer is in the query snapshot.
				continue
			}
			entry := fs_entries.Task{}
			if err := dc.Doc.DataTo(&entry); err != nil {
				sklog.Errorf("corrupt data in firestore, could not unmarshal task entry with id %s", id)
				continue
			}
			if entry.Command != "" {
				err := m.runTask(ctx, id, entry)
				if err != nil {
					sklog.Errorf("error while running task: %s", err)
					continue
				}

				s := m.getDescription()
				s.Uptime = 15 * time.Hour // FIXME(kjlubick) this will trigger reboot task.
				if err := m.sendEvent(ctx, s, map[string]string{
					machine.EventAttribute: string(machine.Idle),
				}); err != nil {
					return skerr.Wrap(err)
				}
			} else {
				sklog.Infof("performing maintenance task %s [%s] (id %s)", entry.MaintenanceTask, entry.Config, id)
				// TODO(kjlubick) actually do maintenance, for now we just quit to simulate
				//   a reboot.
				s := m.getDescription()
				s.Uptime = 15 * time.Hour
				if err := m.sendEvent(ctx, s, map[string]string{
					machine.EventAttribute:       string(machine.StartedTask),
					machine.CurrentTaskAttribute: id,
				}); err != nil {
					return skerr.Wrap(err)
				}
				if err := m.sendEvent(ctx, s, map[string]string{
					machine.EventAttribute:       string(machine.FinishedTask),
					machine.CurrentTaskAttribute: id,
					machine.TaskStatusAttribute:  string(task.Success),
				}); err != nil {
					return skerr.Wrap(err)
				}
				if err := m.sendEvent(ctx, s, map[string]string{
					machine.EventAttribute: string(machine.Rebooting),
				}); err != nil {
					return skerr.Wrap(err)
				}
				return skerr.Fmt("rebooting")
			}
		}
	}
}

func (m *emulatedMachine) sendEvent(ctx context.Context, s machine.Description, attr map[string]string) error {
	b, err := json.Marshal(s)
	if err != nil {
		return skerr.Wrap(err)
	}
	pr := m.topic.Publish(ctx, &pubsub.Message{
		Data:       b,
		Attributes: attr,
	})
	if id, err := pr.Get(ctx); err != nil {
		return skerr.Wrapf(err, "publishing a message")
	} else {
		sklog.Infof("published pubsub message %s\n", id)
	}
	return nil
}

func (m *emulatedMachine) getDescription() machine.Description {
	return machine.Description{
		ID: m.id,
		Dimensions: map[string][]string{
			"id":  {m.id},
			"os":  {"Linux", "Debian", "Debian10", "Debian10.3"},
			"gpu": {"8086", "8086:0f31", "8086:0f31-13.0.6"},
		},
		// TODO(kjlubick) uptime
	}
}

func (m *emulatedMachine) runTask(ctx context.Context, id string, entry fs_entries.Task) error {
	s := m.getDescription()
	if err := m.sendEvent(ctx, s, map[string]string{
		machine.EventAttribute:       string(machine.StartedTask),
		machine.CurrentTaskAttribute: id,
	}); err != nil {
		return skerr.Wrap(err)
	}

	// TODO(kjlubick) actually execute the task
	sklog.Infof("Executing task %s (by sleeping for 5 seconds)", id)
	sklog.Info(entry.Command)
	time.Sleep(5 * time.Second)
	sklog.Info("task finished")

	s = m.getDescription()
	if err := m.sendEvent(ctx, s, map[string]string{
		machine.EventAttribute:       string(machine.FinishedTask),
		machine.CurrentTaskAttribute: id,
		machine.TaskStatusAttribute:  string(task.Success),
	}); err != nil {
		return skerr.Wrap(err)
	}
	// TODO(kjlubick) after task hook
	// TODO(kjlubick) Maybe we should wait until Firestore confirms we have finished the task
	//   Going on to the next one.
	return nil
}

func setupTopic(ctx context.Context, psClient *pubsub.Client, topicName string) (*pubsub.Topic, error) {
	// Create the topic if it doesn't exist yet.
	topic := psClient.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking whether topic %s exists", topicName)
	} else if !exists {
		return nil, skerr.Fmt("PubSub topic %s doesn't yet exist (server needs to make it)", topicName)
	}
	return topic, nil
}
