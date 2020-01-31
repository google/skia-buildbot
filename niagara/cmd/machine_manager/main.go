package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/niagara"
)

const (
	pubsubTopic = "niagara-machines-skia"

	maxFirestoreWriteAttempts = 5
	maxFirestoreOperationTime = 2 * time.Minute
)

func main() {
	flag.Parse()
	ifirestore.EnsureNotEmulator()
	fmt.Println("hello machine manager")
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
	sub, err := setupTopicSubscription(ctx, psClient, pubsubTopic, "machine-manager-01")
	if err != nil {
		sklog.Fatalf("Unable to setups subscription: %s", err)
	}

	sklog.Infof("PubSub topic/subscription set up %v\n", sub)
	m := Manager{
		client: fsClient,
	}

	sklog.Fatalf("error while receiving pubsub notifications: %s", sub.Receive(ctx, m.processMachineMessage))
}

type Manager struct {
	client *ifirestore.Client
}

func (m *Manager) processMachineMessage(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()

	var state niagara.MachineState
	if err := json.Unmarshal(msg.Data, &state); err != nil {
		sklog.Errorf("Invalid message data %s: %s", err, string(msg.Data))
		return
	}
	mID := msg.Attributes[niagara.MachineID]
	if mID == "" {
		sklog.Errorf("Invalid message - no id: %v", msg)
		return
	}
	event := niagara.MachineEvent(msg.Attributes[niagara.Event])

	sklog.Infof("Got %s from machine %s", event, mID)
	if event == niagara.MachineBooted {
		// TODO(kjlubick) Check that either we haven't seen this machine before, or the previous
		//  state was rebooting or sitting idle
		// TODO(kjlubick) check the health of this machine.
		if err := m.updateMachine(ctx, mID, niagara.Ready, niagara.MachineBooted, state); err != nil {
			sklog.Errorf("Could not update machine entry %s", err)
		}
	} else if event == niagara.MachineStartedTask {
		// TODO(kjlubick) Check that the machine had previously finished a task or booted.
		if err := m.updateMachine(ctx, mID, niagara.Busy, niagara.MachineStartedTask, state); err != nil {
			sklog.Errorf("Could not update machine entry %s", err)
		}
		now := time.Now()
		if err := m.updateTask(ctx, state.CurrentTask, niagara.Running, firestore.Update{Path: "started", Value: now}); err != nil {
			sklog.Errorf("Could not update task entry %s", err)
		}
	} else if event == niagara.MachineFinishedTask {
		// TODO(kjlubick) Somehow need to know if a task failed, had an infra failure, or something
		// else when horribly wrong (niagara failure).
		// TODO(kjlubick) Check that the machine was expected to be running a task
		// TODO(kjlubick) check the health of this machine.
		if err := m.updateMachine(ctx, mID, niagara.Quarantined, niagara.MachineFinishedTask, state); err != nil {
			sklog.Errorf("Could not update machine entry %s", err)
		}
		sklog.Infof("Quarantined machine %s", mID)
		now := time.Now()
		if err := m.updateTask(ctx, state.CurrentTask, niagara.Success, firestore.Update{Path: "completed", Value: now}); err != nil {
			sklog.Errorf("Could not update task entry %s", err)
		}
	}

}

func (m *Manager) updateMachine(ctx context.Context, machineID string, newStatus niagara.MachineStatus, currEvent niagara.MachineEvent, state niagara.MachineState) error {
	fme := niagara.FirestoreMachineEntry{
		State:       newStatus,
		LastEvent:   currEvent,
		Updated:     time.Now(),
		Dimensions:  state.Dimensions,
		CurrentTask: state.CurrentTask,
	}
	// store to ifirestore
	doc := m.client.Collection("machines").Doc(machineID)
	if _, err := m.client.Set(ctx, doc, &fme, maxFirestoreWriteAttempts, maxFirestoreOperationTime); err != nil {
		return skerr.Wrapf(err, "updating machine entry for %s", machineID)
	}
	return nil
}

func (m *Manager) updateTask(ctx context.Context, taskID string, status niagara.TaskStatus, ups ...firestore.Update) error {
	ups = append(ups, firestore.Update{Path: "status", Value: status}, firestore.Update{Path: "updated", Value: time.Now()})
	doc := m.client.Collection("tasks").Doc(taskID)
	if _, err := m.client.Update(ctx, doc, maxFirestoreWriteAttempts, maxFirestoreOperationTime, ups); err != nil {
		sklog.Warningf("Could not update task %s", taskID)
	}
	return nil
}

func setupTopicSubscription(ctx context.Context, psClient *pubsub.Client, topicName, subscriberName string) (*pubsub.Subscription, error) {
	// Create the topic if it doesn't exist yet.
	topic := psClient.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking whether topic %s exists", topicName)
	} else if !exists {
		if topic, err = psClient.CreateTopic(ctx, topicName); err != nil {
			return nil, skerr.Wrapf(err, "creating pubsub topic '%s'", topicName)
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	sub := psClient.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking existence of pubsub subscription '%s'", subName)
	} else if !exists {
		sub, err = psClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:             topic,
			AckDeadline:       10 * time.Second,
			RetentionDuration: time.Hour,
			ExpirationPolicy:  time.Duration(0),
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "creating pubsub subscription '%s'", subName)
		}
	}
	sub.ReceiveSettings.MaxOutstandingMessages = 100
	sub.ReceiveSettings.NumGoroutines = 10
	return sub, nil
}
