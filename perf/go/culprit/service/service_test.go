package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	anomalygroup_mocks "go.skia.org/infra/perf/go/anomalygroup/mocks"
	a_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	culprit_mocks "go.skia.org/infra/perf/go/culprit/mocks"
	notify_mocks "go.skia.org/infra/perf/go/culprit/notify/mocks"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	subscription_mocks "go.skia.org/infra/perf/go/subscription/mocks"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func setUp(_ *testing.T) (*culpritService, *anomalygroup_mocks.Store, *culprit_mocks.Store, *subscription_mocks.Store, *notify_mocks.CulpritNotifier) {
	anomalygroup := new(anomalygroup_mocks.Store)
	culpritStore := new(culprit_mocks.Store)
	notifier := new(notify_mocks.CulpritNotifier)
	subscriptionStore := new(subscription_mocks.Store)
	service := New(anomalygroup, culpritStore, subscriptionStore, notifier)
	return service, anomalygroup, culpritStore, subscriptionStore, notifier
}

func TestGetCulprit_ValidInput_ShouldInvokeStoreGet(t *testing.T) {
	c, _, culpritStore, _, _ := setUp(t)
	ctx := context.Background()
	req := &pb.GetCulpritRequest{
		CulpritIds: []string{"cid"},
	}
	culpritStore.On("Get", mock.Anything, []string{"cid"}).Return(nil, nil)

	_, err := c.GetCulprit(ctx, req)

	// assert that the expectations were met
	culpritStore.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestPersistCulprit_ValidInput_ShouldInvokeStoreUpsert(t *testing.T) {
	c, _, culpritStore, _, _ := setUp(t)
	ctx := context.Background()
	commits := []*pb.Commit{{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	},
		{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main1",
			Revision: "456",
		},
	}
	req := &pb.PersistCulpritRequest{
		Commits: commits, AnomalyGroupId: "111",
	}
	culpritStore.On("Upsert", mock.Anything, "111", commits).Return(nil, nil)

	_, err := c.PersistCulprit(ctx, req)

	// assert that the expectations were met
	culpritStore.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestNotifyUser_ValidInput_ShouldInvokeNotifier(t *testing.T) {
	c, anomalygroup, culpritStore, subscriptionStore, notifier := setUp(t)
	ctx := context.Background()
	cids := []string{"culprit_id1"}
	stored_culprits := []*pb.Culprit{{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
		AnomalyGroupIds: []string{"aid1"},
		IssueIds:        []string{"issue_id1"},
		Id:              "culprit_id1",
	}}
	culpritStore.On("Get", mock.Anything, cids).Return(stored_culprits, nil)
	subscriptionName := "s_name"
	subscriptionRevision := "s_version"
	anomalygroup.On("LoadById", mock.Anything, "aid1").Return(
		&a_pb.AnomalyGroup{SubsciptionName: subscriptionName, SubscriptionRevision: subscriptionRevision}, nil)
	subscription := &sub_pb.Subscription{BugComponent: "123"}
	subscriptionStore.On("GetSubscription", mock.Anything,
		subscriptionName, subscriptionRevision).Return(subscription, nil)
	issueId := "issue_id1"
	notifier.On("NotifyCulpritFound", mock.Anything,
		stored_culprits[0], subscription).Return(issueId, nil)
	culpritStore.On("AddIssueId", mock.Anything, stored_culprits[0].Id, issueId, "aid1").Return(nil)
	req := &pb.NotifyUserRequest{
		CulpritIds:     cids,
		AnomalyGroupId: "aid1",
	}
	resp, err := c.NotifyUser(ctx, req)

	assert.Nil(t, err)
	assert.Equal(t, resp, &pb.NotifyUserResponse{IssueIds: []string{"issue_id1"}})
}
