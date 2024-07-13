package service

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/luciconfig"
	luciconfig_mocks "go.skia.org/infra/go/luciconfig/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/alerts"
	alert_mocks "go.skia.org/infra/perf/go/alerts/mock"
	subscription_mocks "go.skia.org/infra/perf/go/subscription/mocks"
	subscription_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

func setUp(ctx context.Context, t *testing.T) (*sheriffconfigService, *subscription_mocks.Store, *alert_mocks.Store, *luciconfig_mocks.ApiClient) {
	subscriptionStore := new(subscription_mocks.Store)
	alertStore := new(alert_mocks.Store)
	luciconfigApiClient := new(luciconfig_mocks.ApiClient)
	service, err := New(ctx, subscriptionStore, alertStore, luciconfigApiClient)
	require.NoError(t, err)

	return service, subscriptionStore, alertStore, luciconfigApiClient
}

func TestImportSheriffConfig_EmptyConfig(t *testing.T) {
	ctx := context.Background()

	service, _, _, apiClient := setUp(ctx, t)

	mockReturn := []*luciconfig.ProjectConfig{}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Couldn't find any configs under path: dummy.path")
}

func TestImportSheriffConfig_InvalidConfig(t *testing.T) {
	ctx := context.Background()

	service, _, _, apiClient := setUp(ctx, t)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content: "c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKfQ==",
		},
	}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0:")
}

func TestImportSheriffConfig_OneSubOneAlert(t *testing.T) {
	ctx := context.Background()

	service, subscriptionStore, alertStore, apiClient := setUp(ctx, t)

	// Encoded content translates to:
	//	subscriptions {
	//		name: "a"
	//		contact_email: "test@google.com"
	//		bug_component: "A>B>C"
	//		anomaly_configs {
	//			rules: {
	//				match: {master: "ChromiumPerf"}
	//			}
	//		}
	//	}
	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  "c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKCWNvbnRhY3RfZW1haWw6ICJ0ZXN0QGdvb2dsZS5jb20iCglidWdfY29tcG9uZW50OiAiQT5CPkMiCglhbm9tYWx5X2NvbmZpZ3MgewoJCXJ1bGVzOiB7CgkJCW1hdGNoOiB7bWFzdGVyOiAiQ2hyb21pdW1QZXJmIn0KCQl9Cgl9Cn0=",
			Revision: "abcd",
		},
	}

	expectedSubscriptions := []*subscription_pb.Subscription{
		{
			Name:         "a",
			ContactEmail: "test@google.com",
			BugComponent: "A>B>C",
			BugPriority:  2,
			BugSeverity:  2,
			Revision:     "abcd",
		},
	}

	expectedAlerts := []*alerts.SaveRequest{
		{
			Cfg: &alerts.Alert{
				IDAsString:           "-1",
				DisplayName:          "master=ChromiumPerf",
				Query:                "master=ChromiumPerf",
				Alert:                "test@google.com",
				Algo:                 "stepfit",
				StateAsString:        "ACTIVE",
				Owner:                "test@google.com",
				MinimumNum:           1,
				Radius:               1,
				DirectionAsString:    "BOTH",
				Action:               "noaction",
				SubscriptionName:     "a",
				SubscriptionRevision: "abcd",
			},
			SubKey: &alerts.SubKey{
				SubName:     "a",
				SubRevision: "abcd",
			},
		},
	}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)
	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}

func TestImportSheriffConfig_MultipleAlerts(t *testing.T) {
	ctx := context.Background()

	service, subscriptionStore, alertStore, apiClient := setUp(ctx, t)

	// Encoded content translates to:
	//	subscriptions {
	//		name: "b"
	//		contact_email: "test@google.com"
	//		bug_component: "A>B>C"
	//		bug_priority: P3
	//		anomaly_configs {
	//			step: COHEN_STEP
	//			radius: 2
	//			threshold: 3.0
	//			action: BISECT
	//			rules {
	//				match: [
	//					{master: "ChromiumPerf", benchmark: "blink_perf.webcodecs"},
	//					{master: "ChromiumPerf", test: "browser_accessibility_events_sum"}
	//				]
	//				exclude: [
	//					{bot: "lacros-eve-perf"},
	//					{bot: "~android-*"}
	//				]
	//			}
	//		}
	//	}
	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  "c3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJhIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGJ1Z19wcmlvcml0eTogUDMgYW5vbWFseV9jb25maWdzIHsgc3RlcDogQ09IRU5fU1RFUCByYWRpdXM6IDIgdGhyZXNob2xkOiAzLjAgYWN0aW9uOiBCSVNFQ1QgcnVsZXMgeyBtYXRjaDogWyB7bWFzdGVyOiAiQ2hyb21pdW1QZXJmIiwgYmVuY2htYXJrOiAiYmxpbmtfcGVyZi53ZWJjb2RlY3MifSwge21hc3RlcjogIkNocm9taXVtUGVyZiIsIHRlc3Q6ICJicm93c2VyX2FjY2Vzc2liaWxpdHlfZXZlbnRzX3N1bSJ9IF0gZXhjbHVkZTogWyB7Ym90OiAibGFjcm9zLWV2ZS1wZXJmIn0sIHtib3Q6ICJ+YW5kcm9pZC0qIn0gXSB9IH0gfQ==",
			Revision: "abcd",
		},
	}

	expectedSubscriptions := []*subscription_pb.Subscription{
		{
			Name:         "a",
			ContactEmail: "test@google.com",
			BugComponent: "A>B>C",
			BugPriority:  3,
			BugSeverity:  2,
			Revision:     "abcd",
		},
	}

	expectedAlerts := []*alerts.SaveRequest{
		{
			Cfg: &alerts.Alert{
				IDAsString:           "-1",
				DisplayName:          "master=ChromiumPerf&benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*",
				Query:                "master=ChromiumPerf&benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*",
				Alert:                "test@google.com",
				Algo:                 "stepfit",
				Step:                 "cohen",
				StateAsString:        "ACTIVE",
				Owner:                "test@google.com",
				MinimumNum:           1,
				Interesting:          3,
				Radius:               2,
				DirectionAsString:    "BOTH",
				Action:               "bisect",
				SubscriptionName:     "a",
				SubscriptionRevision: "abcd",
			},
			SubKey: &alerts.SubKey{
				SubName:     "a",
				SubRevision: "abcd",
			},
		},
		{
			Cfg: &alerts.Alert{
				IDAsString:           "-1",
				DisplayName:          "master=ChromiumPerf&test=browser_accessibility_events_sum&bot=!lacros-eve-perf&bot=!~android-*",
				Query:                "master=ChromiumPerf&test=browser_accessibility_events_sum&bot=!lacros-eve-perf&bot=!~android-*",
				Alert:                "test@google.com",
				Algo:                 "stepfit",
				Step:                 "cohen",
				StateAsString:        "ACTIVE",
				Owner:                "test@google.com",
				MinimumNum:           1,
				Interesting:          3,
				Radius:               2,
				DirectionAsString:    "BOTH",
				Action:               "bisect",
				SubscriptionName:     "a",
				SubscriptionRevision: "abcd",
			},
			SubKey: &alerts.SubKey{
				SubName:     "a",
				SubRevision: "abcd",
			},
		},
	}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)
	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}

func TestImportSheriffConfig_MultipleSubs(t *testing.T) {
	ctx := context.Background()

	service, subscriptionStore, alertStore, apiClient := setUp(ctx, t)

	// Encoded content translates to:
	//	subscriptions {
	//		name: "a"
	//		contact_email: "test@google.com"
	//		bug_component: "A>B>C"
	//		anomaly_configs {
	//			rules: {
	//				match: {main: "ChromiumPerf"}
	//			}
	//		}
	//	}
	//	subscriptions {
	//		name: "b"
	//		contact_email: "test@google.com"
	//		bug_component: "A>B>C"
	//		bug_priority: P3
	//		anomaly_configs {
	//			step: COHEN_STEP
	//			radius: 2
	//			threshold: 3.0
	//			action: BISECT
	//			rules {
	//				match: [
	//					{main: "ChromiumPerf", benchmark: "blink_perf.webcodecs"},
	//					{main: "ChromiumPerf", test: "browser_accessibility_events_sum"}
	//				]
	//				exclude: [
	//					{bot: "lacros-eve-perf"},
	//					{bot: "~android-*"}
	//				]
	//			}
	//		}
	//	}

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  "c3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJhIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGFub21hbHlfY29uZmlncyB7IHJ1bGVzOiB7IG1hdGNoOiB7bWFzdGVyOiAiQ2hyb21pdW1QZXJmIn0gfSB9IH0gc3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJiIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGJ1Z19wcmlvcml0eTogUDMgYW5vbWFseV9jb25maWdzIHsgc3RlcDogQ09IRU5fU1RFUCByYWRpdXM6IDIgdGhyZXNob2xkOiAzLjAgYWN0aW9uOiBCSVNFQ1QgcnVsZXMgeyBtYXRjaDogWyB7bWFzdGVyOiAiQ2hyb21pdW1QZXJmIiwgYmVuY2htYXJrOiAiYmxpbmtfcGVyZi53ZWJjb2RlY3MifSwge21hc3RlcjogIkNocm9taXVtUGVyZiIsIHRlc3Q6ICJicm93c2VyX2FjY2Vzc2liaWxpdHlfZXZlbnRzX3N1bSJ9IF0gZXhjbHVkZTogWyB7Ym90OiAibGFjcm9zLWV2ZS1wZXJmIn0sIHtib3Q6ICJ+YW5kcm9pZC0qIn0gXSB9IH0gfQ==",
			Revision: "abcd",
		},
	}

	expectedSubscriptions := []*subscription_pb.Subscription{
		{
			Name:         "a",
			ContactEmail: "test@google.com",
			BugComponent: "A>B>C",
			BugPriority:  2,
			BugSeverity:  2,
			Revision:     "abcd",
		},
		{
			Name:         "b",
			ContactEmail: "test@google.com",
			BugComponent: "A>B>C",
			BugPriority:  3,
			BugSeverity:  2,
			Revision:     "abcd",
		},
	}

	expectedAlerts := []*alerts.SaveRequest{
		{
			Cfg: &alerts.Alert{
				IDAsString:           "-1",
				DisplayName:          "master=ChromiumPerf",
				Query:                "master=ChromiumPerf",
				Alert:                "test@google.com",
				Algo:                 "stepfit",
				StateAsString:        "ACTIVE",
				Owner:                "test@google.com",
				MinimumNum:           1,
				Radius:               1,
				DirectionAsString:    "BOTH",
				Action:               "noaction",
				SubscriptionName:     "a",
				SubscriptionRevision: "abcd",
			},
			SubKey: &alerts.SubKey{
				SubName:     "a",
				SubRevision: "abcd",
			},
		},
		{
			Cfg: &alerts.Alert{
				IDAsString:           "-1",
				DisplayName:          "master=ChromiumPerf&benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*",
				Query:                "master=ChromiumPerf&benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*",
				Alert:                "test@google.com",
				Algo:                 "stepfit",
				Step:                 "cohen",
				StateAsString:        "ACTIVE",
				Owner:                "test@google.com",
				MinimumNum:           1,
				Interesting:          3,
				Radius:               2,
				DirectionAsString:    "BOTH",
				Action:               "bisect",
				SubscriptionName:     "b",
				SubscriptionRevision: "abcd",
			},
			SubKey: &alerts.SubKey{
				SubName:     "b",
				SubRevision: "abcd",
			},
		},
		{
			Cfg: &alerts.Alert{
				IDAsString:           "-1",
				DisplayName:          "master=ChromiumPerf&test=browser_accessibility_events_sum&bot=!lacros-eve-perf&bot=!~android-*",
				Query:                "master=ChromiumPerf&test=browser_accessibility_events_sum&bot=!lacros-eve-perf&bot=!~android-*",
				Alert:                "test@google.com",
				Algo:                 "stepfit",
				Step:                 "cohen",
				StateAsString:        "ACTIVE",
				Owner:                "test@google.com",
				MinimumNum:           1,
				Interesting:          3,
				Radius:               2,
				DirectionAsString:    "BOTH",
				Action:               "bisect",
				SubscriptionName:     "b",
				SubscriptionRevision: "abcd",
			},
			SubKey: &alerts.SubKey{
				SubName:     "b",
				SubRevision: "abcd",
			},
		},
	}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)
	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}
