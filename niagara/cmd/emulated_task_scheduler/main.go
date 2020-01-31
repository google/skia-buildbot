package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"cloud.google.com/go/firestore"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/niagara"
)

const (
	recoverTime = 10 * time.Second

	maxFirestoreWriteAttempts = 5
	maxFirestoreOperationTime = 2 * time.Minute
)

func main() {
	flag.Parse()
	ifirestore.EnsureNotEmulator()
	fmt.Println("hello task scheduler")
	ctx := context.Background()

	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := ifirestore.NewClient(ctx, "skia-firestore", "niagara", "testing", nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	sklog.Infof("Firestore good %v\n", fsClient)

	ts := taskScheduler{
		client: fsClient,
	}

	sklog.Fatalf("Task scheduler stopped %s", ts.cycle(ctx))
}

func (t *taskScheduler) cycle(ctx context.Context) error {
	snap := t.client.Collection("machines").Where("state", "==", niagara.Ready).Snapshots(ctx)
	for {
		if err := ctx.Err(); err != nil {
			sklog.Debugf("Stopping due to context error: %s", err)
			snap.Stop()
			return skerr.Wrap(err)
		}
		qs, err := snap.Next()
		if err != nil {
			sklog.Errorf("reading query snapshot: %s", err)
			snap.Stop()
			// sleep and rebuild the snapshot query. Once a SnapshotQueryIterator returns
			// an error, it seems to always return that error.
			rt := recoverTime + time.Duration(float32(recoverTime)*rand.Float32())
			time.Sleep(rt)
			sklog.Infof("Trying to recreate query snapshot after having slept %v", t)
			snap = t.client.Collection("machines").Where("state", "==", niagara.Ready).Snapshots(ctx)
			continue
		}

		for _, dc := range qs.Changes {
			t.processReadyMachine(ctx, dc)
		}
	}
}

func (t *taskScheduler) processReadyMachine(ctx context.Context, dc firestore.DocumentChange) {
	if dc.Kind == firestore.DocumentRemoved {
		// We don't care about deleted events
		return
	}
	id := dc.Doc.Ref.ID
	sklog.Infof("Saw %s ready for task", id)

	task := niagara.FirestoreTaskEntry{
		MachineAssigned: id,
		Command:         "docker run alpine /bin/sleep 5",
		Status:          niagara.New,
		Created:         time.Now(),
	}
	doc := t.client.Collection("tasks").NewDoc()
	_, err := t.client.Create(ctx, doc, task, maxFirestoreWriteAttempts, maxFirestoreOperationTime)
	if err != nil {
		sklog.Warningf("error while creating task: %s", err)
	}
	sklog.Infof("Task made for %s", id)

	// maybe do this on a background thread to avoid the 1 write per second limit on a
	// single document
	_, err = t.client.Update(ctx, dc.Doc.Ref, maxFirestoreWriteAttempts, maxFirestoreOperationTime,
		[]firestore.Update{{Path: "state", Value: niagara.Assigned}})
	if err != nil {
		sklog.Warningf("Could not update machine %s with assigned", id)
	}
	sklog.Infof("machine set to assigned")
}

type taskScheduler struct {
	client *ifirestore.Client
}
