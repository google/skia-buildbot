package pubsub

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
)

// AutoUpdateMap is a wrapper around repograph.Map which provides a convenience
// method for auto-updating the Graphs in the Map.
type AutoUpdateMap struct {
	btConf  *bt_gitstore.BTConfig
	Map     repograph.Map
	repoIDs map[string]int64
}

// NewBTGitStoreMap is a wrapper around bt_gitstore.NewBTGitStoreMap which
// provides a convenience method for auto-updating the Graphs in the Map.
func NewAutoUpdateMap(ctx context.Context, repoUrls []string, btConf *bt_gitstore.BTConfig) (*AutoUpdateMap, error) {
	rv := &AutoUpdateMap{
		btConf:  btConf,
		Map:     make(map[string]*repograph.Graph, len(repoUrls)),
		repoIDs: make(map[string]int64, len(repoUrls)),
	}
	for _, repoUrl := range repoUrls {
		gs, err := bt_gitstore.New(ctx, btConf, repoUrl)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create GitStore for %s", repoUrl)
		}
		graph, err := gitstore.GetRepoGraph(ctx, gs)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create Graph from GitStore for %s", repoUrl)
		}
		rv.Map[repoUrl] = graph
		rv.repoIDs[repoUrl] = gs.RepoID
	}
	return rv, nil
}

// Start initializes auto-updating of the AutoUpdateMap.
func (m *AutoUpdateMap) Start(ctx context.Context, subscriberID string, ts oauth2.TokenSource, fallbackInterval time.Duration, callback AutoUpdateMapCallback) error {
	for repoUrl, graph := range m.Map {
		// https://golang.org/doc/faq#closures_and_goroutines
		repoUrl := repoUrl
		err := AutoUpdateGraph(ctx, m.btConf.ProjectID, m.btConf.InstanceID, m.btConf.TableID, subscriberID, m.repoIDs[repoUrl], graph, ts, fallbackInterval, func(ctx context.Context, g *repograph.Graph, ack, nack func()) error {
			return callback(ctx, repoUrl, g, ack, nack)
		})
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// AutoUpdateCallback is a callback function used in AutoUpdateGraph which is
// called after the Graph is updated but before the changes are committed. If
// the callback returns an error, the changes are not committed. In addition to
// the Graph itself, the callback accepts two functions as parameters, ack and
// nack, which in turn call the Ack() or Nack() functions on the pubsub
// message(s) which triggered the update. This allows the caller to control when
// messages are redelivered.
type AutoUpdateCallback func(context.Context, *repograph.Graph, func(), func()) error

// AutoUpdateMapCallback like AutoUpdateCallback, except that it's handed to
// NewAutoUpdateMap.Start() and also includes the repo URL.
type AutoUpdateMapCallback func(context.Context, string, *repograph.Graph, func(), func()) error

// AutoUpdateGraph updates the passed-in Graph whenever a pubsub message is
// received for the given repo and at the given fallback interval. It calls the
// given callback function after the Graph updates but before the changes are
// committed.
func AutoUpdateGraph(ctx context.Context, btProject, btInstance, btTable, subscriberID string, repoID int64, graph *repograph.Graph, ts oauth2.TokenSource, fallbackInterval time.Duration, callback AutoUpdateCallback) error {
	var ticker *time.Ticker
	var tickCh <-chan time.Time
	if fallbackInterval > 0 {
		ticker = time.NewTicker(fallbackInterval)
		go func() {
			<-ctx.Done()
			ticker.Stop()
		}()
		tickCh = ticker.C
	}
	_, err := autoUpdateGraph(ctx, btProject, btInstance, btTable, subscriberID, repoID, graph, ts, tickCh, callback)
	if err != nil && ticker != nil {
		ticker.Stop()
	}
	return err
}

// updateRequest is a struct used for handling a request to update the Graph.
type updateRequest struct {
	ID   string
	done chan struct{}
	msg  *pubsub.Message
}

// sendAndWait sends the given Message on the given channel, then waits for it
// to be handled. The Message may be nil, ie. in the case of periodic updates
// not associated with a pubsub message.
func sendAndWait(ch chan<- *updateRequest, msg *pubsub.Message) {
	// The passed-in channel may be closed, eg. if the parent context is
	// canceled. Don't panic in this case, just drop the update.
	defer func() {
		if r := recover(); r != nil {
			sklog.Errorf("Recovered panic: %s", r)
		}
	}()
	// Create the update request.
	req := &updateRequest{
		done: make(chan struct{}),
	}
	if msg != nil {
		req.ID = msg.ID
		req.msg = msg
	}
	// Send the update request.
	ch <- req
	// Wait for the update to finish.
	<-req.done
}

// ackFn returns a function which Acks all of the messages in the batch.
func ackFn(batch map[string]*updateRequest) func() {
	return func() {
		for _, req := range batch {
			if req.msg != nil {
				req.msg.Ack()
			}
		}
	}
}

// nackFn returns a function which Nacks all of the messages in the batch.
func nackFn(batch map[string]*updateRequest) func() {
	return func() {
		for _, req := range batch {
			if req.msg != nil {
				req.msg.Nack()
			}
		}
	}
}

// autoUpdateGraph is a helper function used by AutoUpdateGraph, for testing.
// In addition to any error, returns a func which waits for all spawned
// goroutines to exit.
func autoUpdateGraph(ctx context.Context, btProject, btInstance, btTable, subscriberID string, repoID int64, graph *repograph.Graph, ts oauth2.TokenSource, tickCh <-chan time.Time, callback AutoUpdateCallback) (func(), error) {
	var wg sync.WaitGroup

	// This channel queues updates to the Graph whenever a pubsub message
	// is received, and whenever we hit the fallback interval.
	ch := make(chan *updateRequest)

	// Create the PubSub subscription.
	err := NewSubscriber(ctx, btProject, btInstance, btTable, subscriberID, repoID, ts, func(msg *pubsub.Message, branches map[string]string) {
		sendAndWait(ch, msg)
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create auto-updating repograph.Graph")
	}

	// Close the channel when the context is canceled.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		close(ch)
	}()

	// Spin up a goroutine to update the Graph at the specified fallback
	// interval.
	if tickCh != nil {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tickCh:
					sendAndWait(ch, nil)
				}
			}
		}()
	}

	// Spin up a goroutine to update the graph.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Take all of the available updateRequests from the channel,
		// and add them to a map, keyed by ID. This helps to deduplicate
		// requests.
		batch := map[string]*updateRequest{}
		for req := range ch {
			batch[req.ID] = req
			// If there are no more requests available, update the
			// Graph with the passed-in callback.
			if len(ch) == 0 {
				err := graph.UpdateWithCallback(ctx, func(g *repograph.Graph) error {
					err := callback(ctx, g, ackFn(batch), nackFn(batch))
					for _, req := range batch {
						req.done <- struct{}{}
					}
					return err
				})
				if err != nil {
					sklog.Errorf("Failed to update repo: %s", err)
				}
				batch = map[string]*updateRequest{}
			}
		}
	}()
	return wg.Wait, nil
}
