package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	anomalygroup_mocks "go.skia.org/infra/perf/go/anomalygroup/mocks"
	a_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	culprit_mocks "go.skia.org/infra/perf/go/culprit/mocks"
	notify_mocks "go.skia.org/infra/perf/go/culprit/notify/mocks"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	subscription_mocks "go.skia.org/infra/perf/go/subscription/mocks"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func setUp(_ *testing.T, testConfig *config.InstanceConfig) (*culpritService, *anomalygroup_mocks.Store, *culprit_mocks.Store, *subscription_mocks.Store, *notify_mocks.CulpritNotifier) {
	anomalygroupStore := new(anomalygroup_mocks.Store)
	culpritStore := new(culprit_mocks.Store)
	subscriptionStore := new(subscription_mocks.Store)
	notifier := new(notify_mocks.CulpritNotifier)
	service := New(anomalygroupStore, culpritStore, subscriptionStore, notifier, testConfig)
	return service, anomalygroupStore, culpritStore, subscriptionStore, notifier
}

func TestGetCulprit_ValidInput_ShouldInvokeStoreGet(t *testing.T) {
	c, _, culpritStore, _, _ := setUp(t, nil)
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
	c, anomalygroupStore, culpritStore, _, _ := setUp(t, nil)
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
	mockCulpritIds := []string{"cid1", "cid2"}
	culpritStore.On("Upsert", mock.Anything, "111", commits).Return(mockCulpritIds, nil)
	anomalygroupStore.On("AddCulpritIDs", mock.Anything, "111", mockCulpritIds).Return(nil, nil)

	_, err := c.PersistCulprit(ctx, req)

	// assert that the expectations were met
	culpritStore.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestNotifyUserOfCulprit_ValidInput_ShouldInvokeNotifier(t *testing.T) {
	c, anomalygroup, culpritStore, subscriptionStore, notifier := setUp(t, nil)
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
	req := &pb.NotifyUserOfCulpritRequest{
		CulpritIds:     cids,
		AnomalyGroupId: "aid1",
	}
	resp, err := c.NotifyUserOfCulprit(ctx, req)

	assert.Nil(t, err)
	assert.Equal(t, resp, &pb.NotifyUserOfCulpritResponse{IssueIds: []string{"issue_id1"}})
}

func TestNotifyUserOfAnomaly_ValidInput_ShouldInvokeNotifier(t *testing.T) {
	c, anomalygroup, _, subscriptionStore, notifier := setUp(t, nil)
	ctx := context.Background()

	subscriptionName := "s_name"
	subscriptionRevision := "s_version"
	group := &a_pb.AnomalyGroup{SubsciptionName: subscriptionName, SubscriptionRevision: subscriptionRevision}
	anomalygroup.On("LoadById", mock.Anything, "aid1").Return(group, nil)
	subscription := &sub_pb.Subscription{BugComponent: "123"}
	subscriptionStore.On("GetSubscription", mock.Anything,
		subscriptionName, subscriptionRevision).Return(subscription, nil)
	issueId := "issue_id1"
	anomalies := []*pb.Anomaly{
		{
			StartCommit: 123,
			EndCommit:   678,
			Paramset: map[string]string{
				"benchmark": "b",
			},
		},
	}
	notifier.On("NotifyAnomaliesFound", mock.Anything,
		group, subscription, anomalies).Return(issueId, nil)
	req := &pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: "aid1",
		Anomaly:        anomalies,
	}
	resp, err := c.NotifyUserOfAnomaly(ctx, req)

	assert.Nil(t, err)
	assert.Equal(t, resp, &pb.NotifyUserOfAnomalyResponse{IssueId: "issue_id1"})
}

func TestNotifyUserOfAnomaly_WithConfig_ValidInput_NotifyByConfig(t *testing.T) {
	cfg := &config.InstanceConfig{
		SheriffConfigsToNotify: []string{"s_name"},
	}
	c, anomalygroup, _, subscriptionStore, notifier := setUp(t, cfg)
	ctx := context.Background()

	subscriptionName := "s_name"
	subscriptionRevision := "s_version"
	group := &a_pb.AnomalyGroup{SubsciptionName: subscriptionName, SubscriptionRevision: subscriptionRevision}
	anomalygroup.On("LoadById", mock.Anything, "aid1").Return(group, nil)
	subscription := &sub_pb.Subscription{BugComponent: "123"}
	subscriptionStore.On("GetSubscription", mock.Anything,
		subscriptionName, subscriptionRevision).Return(subscription, nil)
	issueId := "issue_id1"
	anomalies := []*pb.Anomaly{
		{
			StartCommit: 123,
			EndCommit:   678,
			Paramset: map[string]string{
				"benchmark": "b",
			},
		},
	}
	notifier.On("NotifyAnomaliesFound", mock.Anything,
		group, subscription, anomalies).Return(issueId, nil)
	req := &pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: "aid1",
		Anomaly:        anomalies,
	}
	resp, err := c.NotifyUserOfAnomaly(ctx, req)

	assert.Nil(t, err)
	assert.Equal(t, resp, &pb.NotifyUserOfAnomalyResponse{IssueId: "issue_id1"})
}

func TestNotifyUserOfAnomaly_WithConfig_NoSub_NotNotify(t *testing.T) {
	cfg := &config.InstanceConfig{
		SheriffConfigsToNotify: []string{"s_name"},
	}
	c, anomalygroup, _, subscriptionStore, notifier := setUp(t, cfg)
	ctx := context.Background()

	subscriptionName := "s_name"
	subscriptionRevision := "s_version"
	group := &a_pb.AnomalyGroup{SubsciptionName: subscriptionName, SubscriptionRevision: subscriptionRevision}
	anomalygroup.On("LoadById", mock.Anything, "aid1").Return(group, nil)
	fakeSubscription := &sub_pb.Subscription{
		Name:         "Mocked Sub For Anomaly - Report",
		Revision:     "Mocked Revision - Report",
		BugLabels:    []string{"Mocked Sub Label"},
		Hotlists:     []string{"5141966"},
		BugComponent: "1325852",
		BugPriority:  2,
		BugSeverity:  3,
		BugCcEmails:  []string{""},
		ContactEmail: "",
	}
	subscriptionStore.On("GetSubscription", mock.Anything,
		subscriptionName, subscriptionRevision).Return(nil, nil)
	issueId := "issue_id1"
	anomalies := []*pb.Anomaly{
		{
			StartCommit: 123,
			EndCommit:   678,
			Paramset: map[string]string{
				"benchmark": "b",
			},
		},
	}
	notifier.On("NotifyAnomaliesFound", mock.Anything,
		group, fakeSubscription, anomalies).Return(issueId, nil)
	req := &pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: "aid1",
		Anomaly:        anomalies,
	}
	resp, err := c.NotifyUserOfAnomaly(ctx, req)

	assert.Nil(t, err)
	assert.Equal(t, resp, &pb.NotifyUserOfAnomalyResponse{IssueId: "issue_id1"})
}

func TestNotifyUserOfAnomaly_WithConfig_NotAllowedSub_NotNotify(t *testing.T) {
	cfg := &config.InstanceConfig{
		SheriffConfigsToNotify: []string{"s_name_x"},
	}
	c, anomalygroup, _, subscriptionStore, notifier := setUp(t, cfg)
	ctx := context.Background()

	subscriptionName := "s_name"
	subscriptionRevision := "s_version"
	group := &a_pb.AnomalyGroup{SubsciptionName: subscriptionName, SubscriptionRevision: subscriptionRevision}
	anomalygroup.On("LoadById", mock.Anything, "aid1").Return(group, nil)
	subscription := &sub_pb.Subscription{
		Name:         "Original Sub For Anomaly - Report", // Used
		Revision:     "Original Revision - Report",        //Used
		BugComponent: "123",                               //Not used
	}
	fakeSubscription := &sub_pb.Subscription{
		Name:         "Original Sub For Anomaly - Report",
		Revision:     "Original Revision - Report",
		BugLabels:    []string{"Mocked Sub Label - overwrite"},
		Hotlists:     []string{"5141966"},
		BugComponent: "1325852",
		BugCcEmails:  []string{""},
		ContactEmail: "",
	}
	subscriptionStore.On("GetSubscription", mock.Anything,
		subscriptionName, subscriptionRevision).Return(subscription, nil)
	issueId := "issue_id1"
	anomalies := []*pb.Anomaly{
		{
			StartCommit: 123,
			EndCommit:   678,
			Paramset: map[string]string{
				"benchmark": "b",
			},
		},
	}
	notifier.On("NotifyAnomaliesFound", mock.Anything,
		group, fakeSubscription, anomalies).Return(issueId, nil)
	req := &pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: "aid1",
		Anomaly:        anomalies,
	}
	resp, err := c.NotifyUserOfAnomaly(ctx, req)

	assert.Nil(t, err)
	assert.Equal(t, resp, &pb.NotifyUserOfAnomalyResponse{IssueId: "issue_id1"})
}
