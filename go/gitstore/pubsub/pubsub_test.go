package pubsub

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	btProject    = "fake-test-project"
	btInstance   = "fake-test-instance"
	btAppProfile = "testing"
	repoID       = 9999
	subID        = "test-subscriber"
)

func TestPubSub(t *testing.T) {
	unittest.RequiresPubSubEmulator(t)

	// This is just a thin wrapper around Cloud PubSub, so all we really
	// need to test is that we can create a publisher and subscriber with
	// the same BigTable information and verify that we call the callback
	// at least once per published message.
	ctx := context.Background()
	btTable := uuid.New().String()
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  btProject,
		InstanceID: btInstance,
		TableID:    btTable,
		AppProfile: btAppProfile,
	}
	p, err := NewPublisher(ctx, btConf, repoID, nil)
	assert.NoError(t, err)
	ch := make(chan map[string]string)
	err = NewSubscriber(ctx, btConf, subID, repoID, nil, func(msg *pubsub.Message, branches map[string]string) {
		ch <- branches
		msg.Ack()
	})
	assert.NoError(t, err)

	// These are the messages we'll send.
	msgs := []map[string]string{
		{"a": "a1"},
		{"a": "a2"},
		{"b": "a1"},
		{"b": "b1"},
		{
			"a": "a4",
			"b": "b3",
		},
	}

	// Send the messages.
	for _, msg := range msgs {
		p.Publish(ctx, msg)
	}

	// Collect the results. Stop when we've got them all, or when the
	// timeout is reached.
	results := map[string]bool{}
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case msg := <-ch:
			json := testutils.MarshalIndentJSON(t, msg)
			results[json] = true
			if len(results) >= len(msgs) {
				break loop
			}
		case <-timeout:
			assert.FailNow(t, "Failed to receive pubsub messages within allotted time.")
		}
	}

	// Ensure that we actually saw each one of the messages.
	for _, msg := range msgs {
		json := testutils.MarshalIndentJSON(t, msg)
		assert.True(t, results[json])
	}
}

func TestUpdateUsingPubSub(t *testing.T) {
	unittest.RequiresPubSubEmulator(t)

	ctx := cipd_git.UseGitFinder(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create the git repo and graph.
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	gd := git.GitDir(gb.Dir())
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	graph, err := repograph.NewLocalGraph(ctx, gb.RepoUrl(), tmp)
	assert.NoError(t, err)

	// Create the pubsub publisher and start auto-updating the Graph.
	btTable := uuid.New().String()
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  btProject,
		InstanceID: btInstance,
		TableID:    btTable,
		AppProfile: btAppProfile,
	}
	p, err := NewPublisher(ctx, btConf, repoID, nil)
	assert.NoError(t, err)
	ch := make(chan []*git.Branch)
	tickCh := make(chan time.Time)
	wait, err := updateUsingPubSubHelper(ctx, btConf, subID, repoID, graph, nil, tickCh, func(ctx context.Context, g *repograph.Graph, ack, nack func()) error {
		gotBranches := g.BranchHeads()
		ch <- gotBranches
		ack()
		return nil
	})
	assert.NoError(t, err)
	defer func() {
		cancel()
		close(ch)
		wait()
	}()

	// Helper functions to add commits, send pubsub messages, and wait for
	// the Graph to auto-update, asserting that the branch heads are
	// correct.
	t0 := time.Unix(1566572650, 0) // Arbitrary; makes commit hashes deterministic.
	tc := time.Duration(0)
	commit := func() string {
		// Add a commit.
		hash := gb.CommitGenAt(ctx, "fake", t0.Add(tc*time.Second))
		tc++
		return hash
	}
	test := func() {
		// Get the expected branches, send a pubsub message.
		expectBranches, err := gd.Branches(ctx)
		assert.NoError(t, err)
		branchMap := make(map[string]string, len(expectBranches))
		for _, b := range expectBranches {
			branchMap[b.Name] = b.Head
		}
		p.Publish(ctx, branchMap)
		// Wait for the Graph to auto-update.
		gotBranches := <-ch
		assertdeep.Equal(t, expectBranches, gotBranches)
	}
	commitAndTest := func() {
		commit()
		test()
	}
	tickAndTest := func() {
		// Get the expected branches, send a tick.
		expectBranches, err := gd.Branches(ctx)
		assert.NoError(t, err)
		tickCh <- time.Now()
		// Wait for the Graph to auto-update.
		gotBranches := <-ch
		assertdeep.Equal(t, expectBranches, gotBranches)
	}

	// Tests.
	tickAndTest()
	commitAndTest()
	commit()
	gb.CreateBranchTrackBranch(ctx, "branch2", git.MainBranch)
	test()
	commit()
	commit()
	test()
	tickAndTest()
	commit()
	tickAndTest()
}
