package main

import (
	"context"
	"encoding/json"
	"flag"
	"time"

	"cloud.google.com/go/firestore"
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
		name = flag.String("id", "gce-0001", "machine id")
		pool = flag.String("pool", "skia", "pool this machine is in")
	)
	flag.Parse()
	ifirestore.EnsureNotEmulator()
	sklog.Infof("hello emulated machine %s\n", *name)
	ctx := context.Background()

	// Auth note: the underlying ifirestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := ifirestore.NewClient(ctx, "skia-firestore", "niagara", "testing", nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	sklog.Infof("Firestore good %v\n", fsClient)

	psClient, err := pubsub.NewClient(ctx, "skia-public")
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
	}

	sklog.Fatalf("Error running emulated machine %v", m.emulateMachine(ctx))
}

type emulatedMachine struct {
	id       string
	fsClient *ifirestore.Client
	topic    *pubsub.Topic
}

func (m *emulatedMachine) emulateMachine(ctx context.Context) error {
	if err := m.sendEvent(ctx, m.getDescription(), map[string]string{machine.EventAttribute: string(machine.Booted)}); err != nil {
		return skerr.Wrapf(err, "sending booted message")
	}
	// TODO(kjlubick) start a health check loop
	// TODO(kjlubick) listen to sigint to send rebooting or something

	q := m.fsClient.Collection("tasks").Where("machine_assigned", "==", m.id).
		Where("status", "==", task.New).Limit(1)
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
				sklog.Debugf("unexpected deletion of task")
				continue
			}
			entry := fs_entries.Task{}
			if err := dc.Doc.DataTo(&entry); err != nil {
				sklog.Errorf("corrupt data in firestore, could not unmarshal task entry with id %s", id)
				continue
			}
			err := m.runTask(ctx, id, entry)
			if err != nil {
				sklog.Errorf("error while running task: %s", err)
				continue
			}
			// FIXME(kjlubick) this only runs one task and then quits, just for a demo.
			return nil
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
		sklog.Infof("published %s\n", id)
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
	}
}

func (m *emulatedMachine) runTask(ctx context.Context, id string, entry fs_entries.Task) error {
	s := m.getDescription()
	s.CurrentTask = id
	if err := m.sendEvent(ctx, s, map[string]string{machine.EventAttribute: string(machine.StartedTask)}); err != nil {
		return skerr.Wrap(err)
	}

	// TODO(kjlubick) actually execute the task
	sklog.Infof("Executing task %s (by sleeping for 5 seconds)", id)
	sklog.Info(entry.Command)
	time.Sleep(5 * time.Second)
	sklog.Info("task finished")

	s = m.getDescription()
	s.CurrentTask = id
	if err := m.sendEvent(ctx, s, map[string]string{
		machine.EventAttribute:      string(machine.FinishedTask),
		machine.TaskStatusAttribute: string(task.Success),
	}); err != nil {
		return skerr.Wrap(err)
	}
	// TODO(kjlubick) after task hook
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
