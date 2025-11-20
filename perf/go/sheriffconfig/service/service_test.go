package service

import (
	"context"
	"encoding/base64"
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
	"go.skia.org/infra/perf/go/sql/sqltest"
	subscription_mocks "go.skia.org/infra/perf/go/subscription/mocks"
	subscription_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

func setUp(ctx context.Context, t *testing.T) (*sheriffconfigService, *subscription_mocks.Store, *alert_mocks.Store, *luciconfig_mocks.ApiClient) {
	db := sqltest.NewSpannerDBForTests(t, "substore")

	subscriptionStore := new(subscription_mocks.Store)
	alertStore := new(alert_mocks.Store)
	luciconfigApiClient := new(luciconfig_mocks.ApiClient)
	service, err := New(ctx, db, subscriptionStore, alertStore, luciconfigApiClient, "chrome-internal")
	require.NoError(t, err)

	return service, subscriptionStore, alertStore, luciconfigApiClient
}

// Test with GetProjectConfigs returning no configs. It should fail.
func TestImportSheriffConfig_EmptyConfig(t *testing.T) {
	ctx := context.Background()

	service, _, _, apiClient := setUp(ctx, t)

	mockReturn := []*luciconfig.ProjectConfig{}

	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Couldn't find any configs under path: dummy.path")
}

// Test attempts to import an invalid configuration. Check it fails with appropriate error.
func TestImportSheriffConfig_InvalidConfig(t *testing.T) {
	ctx := context.Background()

	service, _, _, apiClient := setUp(ctx, t)

	config, err := base64.StdEncoding.DecodeString("c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKfQ==")
	require.NoError(t, err)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content: string(config),
		},
	}

	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)

	err = service.ImportSheriffConfig(ctx, "dummy.path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0:")
}

// Test creating one subscription with one alert.
func TestImportSheriffConfig_OneSubOneAlert(t *testing.T) {
	ctx := context.Background()

	service, subscriptionStore, alertStore, apiClient := setUp(ctx, t)

	// Encoded content translates to:
	// subscriptions {
	// 		name: "a"
	// 		contact_email: "test@google.com"
	// 		bug_component: "A>B>C"
	// 		anomaly_configs {
	// 			rules: {
	// 				match: "master=ChromiumPerf"
	// 			}
	// 		}
	// 		instance: "chrome-internal"
	// }
	config, err := base64.StdEncoding.DecodeString("c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKCWNvbnRhY3RfZW1haWw6ICJ0ZXN0QGdvb2dsZS5jb20iCglidWdfY29tcG9uZW50OiAiQT5CPkMiCglhbm9tYWx5X2NvbmZpZ3MgewoJCXJ1bGVzOiB7CgkJCW1hdGNoOiAibWFzdGVyPUNocm9taXVtUGVyZiIKCQl9Cgl9CglpbnN0YW5jZTogImNocm9tZS1pbnRlcm5hbCIKfQ==")
	require.NoError(t, err)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  string(config),
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
				Radius:               4,
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
	subscriptionStore.On("GetSubscription", testutils.AnyContext, "a", "abcd").Return(nil, nil)
	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)
	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err = service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}

// Test creating one subscription with 2 alerts.
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
	//					"master=ChromiumPerf&benchmark=blink_perf.webcodecs",
	//					"master=ChromiumPerf&test=browser_accessibility_events_sum"
	//				]
	//				exclude: [
	// 					"bot=lacros-eve-perf",
	//					"bot=~android-*"
	//				]
	//			}
	//		}
	//		instance: "chrome-internal"
	//	}
	config, err := base64.StdEncoding.DecodeString("c3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJhIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGJ1Z19wcmlvcml0eTogUDMgYW5vbWFseV9jb25maWdzIHsgc3RlcDogQ09IRU5fU1RFUCByYWRpdXM6IDIgdGhyZXNob2xkOiAzLjAgYWN0aW9uOiBCSVNFQ1QgcnVsZXMgeyBtYXRjaDogWyAibWFzdGVyPUNocm9taXVtUGVyZiZiZW5jaG1hcms9YmxpbmtfcGVyZi53ZWJjb2RlY3MiLCAibWFzdGVyPUNocm9taXVtUGVyZiZ0ZXN0PWJyb3dzZXJfYWNjZXNzaWJpbGl0eV9ldmVudHNfc3VtIiBdIGV4Y2x1ZGU6IFsgImJvdD1sYWNyb3MtZXZlLXBlcmYiLCAiYm90PX5hbmRyb2lkLSoiIF0gfSB9IGluc3RhbmNlOiAiY2hyb21lLWludGVybmFsIiB9")
	require.NoError(t, err)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  string(config),
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
				DisplayName:          "benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf",
				Query:                "benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf",
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
				DisplayName:          "bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf&test=browser_accessibility_events_sum",
				Query:                "bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf&test=browser_accessibility_events_sum",
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

	subscriptionStore.On("GetSubscription", testutils.AnyContext, "a", "abcd").Return(nil, nil)
	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)
	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err = service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}

// Test creating two subscriptions and 3 alerts.
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
	//				match: "master=ChromiumPerf"
	//			}
	//		}
	//		instance: "chrome-internal"
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
	//					"master=ChromiumPerf&benchmark=blink_perf.webcodecs",
	//					"master=ChromiumPerf&test=browser_accessibility_events_sum"
	//				]
	//				exclude: [
	// 					"bot=lacros-eve-perf",
	//					"bot=~android-*"
	//				]
	//			}
	//		}
	//		instance: "chrome-internal"
	//	}
	config, err := base64.StdEncoding.DecodeString("c3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJhIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGFub21hbHlfY29uZmlncyB7IHJ1bGVzOiB7IG1hdGNoOiAibWFzdGVyPUNocm9taXVtUGVyZiIgfSB9IGluc3RhbmNlOiAiY2hyb21lLWludGVybmFsIiB9IHN1YnNjcmlwdGlvbnMgeyBuYW1lOiAiYiIgY29udGFjdF9lbWFpbDogInRlc3RAZ29vZ2xlLmNvbSIgYnVnX2NvbXBvbmVudDogIkE+Qj5DIiBidWdfcHJpb3JpdHk6IFAzIGFub21hbHlfY29uZmlncyB7IHN0ZXA6IENPSEVOX1NURVAgcmFkaXVzOiAyIHRocmVzaG9sZDogMy4wIGFjdGlvbjogQklTRUNUIHJ1bGVzIHsgbWF0Y2g6IFsgIm1hc3Rlcj1DaHJvbWl1bVBlcmYmYmVuY2htYXJrPWJsaW5rX3BlcmYud2ViY29kZWNzIiwgIm1hc3Rlcj1DaHJvbWl1bVBlcmYmdGVzdD1icm93c2VyX2FjY2Vzc2liaWxpdHlfZXZlbnRzX3N1bSIgXSBleGNsdWRlOiBbICJib3Q9bGFjcm9zLWV2ZS1wZXJmIiwgImJvdD1+YW5kcm9pZC0qIiBdIH0gfSBpbnN0YW5jZTogImNocm9tZS1pbnRlcm5hbCIgfQ==")
	require.NoError(t, err)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  string(config),
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
				Radius:               4,
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
				DisplayName:          "benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf",
				Query:                "benchmark=blink_perf.webcodecs&bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf",
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
				DisplayName:          "bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf&test=browser_accessibility_events_sum",
				Query:                "bot=!lacros-eve-perf&bot=!~android-*&master=ChromiumPerf&test=browser_accessibility_events_sum",
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

	subscriptionStore.On("GetSubscription", testutils.AnyContext, "a", "abcd").Return(nil, nil)
	subscriptionStore.On("GetSubscription", testutils.AnyContext, "b", "abcd").Return(nil, nil)
	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)
	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err = service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}

// Input is two subscriptions ("a" and "b"). Subscription "b" already exists in the DB, so we check we only
// insert subscriptions and alerts associated with "a".
func TestImportSheriffConfig_MultipleSubsOneExists(t *testing.T) {
	ctx := context.Background()

	service, subscriptionStore, alertStore, apiClient := setUp(ctx, t)

	// Encoded content translates to:
	//	subscriptions {
	//		name: "a"
	//		contact_email: "test@google.com"
	//		bug_component: "A>B>C"
	//		anomaly_configs {
	//			rules: {
	//				match: "master=ChromiumPerf"
	//			}
	//		}
	//		instance: "chrome-internal"
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
	//					"master=ChromiumPerf&benchmark=blink_perf.webcodecs",
	//					"master=ChromiumPerf&test=browser_accessibility_events_sum"
	//				]
	//				exclude: [
	// 					"bot=lacros-eve-perf",
	//					"bot=~android-*"
	//				]
	//			}
	//		}
	//		instance: "chrome-internal"
	//	}
	config, err := base64.StdEncoding.DecodeString("c3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJhIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGFub21hbHlfY29uZmlncyB7IHJ1bGVzOiB7IG1hdGNoOiAibWFzdGVyPUNocm9taXVtUGVyZiIgfSB9IGluc3RhbmNlOiAiY2hyb21lLWludGVybmFsIiB9IHN1YnNjcmlwdGlvbnMgeyBuYW1lOiAiYiIgY29udGFjdF9lbWFpbDogInRlc3RAZ29vZ2xlLmNvbSIgYnVnX2NvbXBvbmVudDogIkE+Qj5DIiBidWdfcHJpb3JpdHk6IFAzIGFub21hbHlfY29uZmlncyB7IHN0ZXA6IENPSEVOX1NURVAgcmFkaXVzOiAyIHRocmVzaG9sZDogMy4wIGFjdGlvbjogQklTRUNUIHJ1bGVzIHsgbWF0Y2g6IFsgIm1hc3Rlcj1DaHJvbWl1bVBlcmYmYmVuY2htYXJrPWJsaW5rX3BlcmYud2ViY29kZWNzIiwgIm1hc3Rlcj1DaHJvbWl1bVBlcmYmdGVzdD1icm93c2VyX2FjY2Vzc2liaWxpdHlfZXZlbnRzX3N1bSIgXSBleGNsdWRlOiBbICJib3Q9bGFjcm9zLWV2ZS1wZXJmIiwgImJvdD1+YW5kcm9pZC0qIiBdIH0gfSBpbnN0YW5jZTogImNocm9tZS1pbnRlcm5hbCIgfQ==")
	require.NoError(t, err)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  string(config),
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
				Radius:               4,
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

	subscriptionStore.On("GetSubscription", testutils.AnyContext, "a", "abcd").Return(nil, nil)
	subscriptionStore.On("GetSubscription", testutils.AnyContext, "b", "abcd").Return(&subscription_pb.Subscription{Name: "b"}, nil)
	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)

	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err = service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}

// Input is two subscriptions ("a" and "b"), one with instance "v8" and one with "chrome-internal".
// Only subscription "a" should be imported.
func TestImportSheriffConfig_MultipleSubsMultipleInstances(t *testing.T) {
	ctx := context.Background()

	service, subscriptionStore, alertStore, apiClient := setUp(ctx, t)

	// Encoded content translates to:
	//	subscriptions {
	//		name: "a"
	//		contact_email: "test@google.com"
	//		bug_component: "A>B>C"
	//		anomaly_configs {
	//			rules: {
	//				match: "master=ChromiumPerf"
	//			}
	//		}
	//		instance: "chrome-internal"
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
	//					"master=ChromiumPerf&benchmark=blink_perf.webcodecs",
	//					"master=ChromiumPerf&test=browser_accessibility_events_sum"
	//				]
	//				exclude: [
	// 					"bot=lacros-eve-perf",
	//					"bot=~android-*"
	//				]
	//			}
	//		}
	//		instance: "v8"
	//	}
	config, err := base64.StdEncoding.DecodeString("c3Vic2NyaXB0aW9ucyB7IG5hbWU6ICJhIiBjb250YWN0X2VtYWlsOiAidGVzdEBnb29nbGUuY29tIiBidWdfY29tcG9uZW50OiAiQT5CPkMiIGFub21hbHlfY29uZmlncyB7IHJ1bGVzOiB7IG1hdGNoOiAibWFzdGVyPUNocm9taXVtUGVyZiIgfSB9IGluc3RhbmNlOiAiY2hyb21lLWludGVybmFsIiB9IHN1YnNjcmlwdGlvbnMgeyBuYW1lOiAiYiIgY29udGFjdF9lbWFpbDogInRlc3RAZ29vZ2xlLmNvbSIgYnVnX2NvbXBvbmVudDogIkE+Qj5DIiBidWdfcHJpb3JpdHk6IFAzIGFub21hbHlfY29uZmlncyB7IHN0ZXA6IENPSEVOX1NURVAgcmFkaXVzOiAyIHRocmVzaG9sZDogMy4wIGFjdGlvbjogQklTRUNUIHJ1bGVzIHsgbWF0Y2g6IFsgIm1hc3Rlcj1DaHJvbWl1bVBlcmYmYmVuY2htYXJrPWJsaW5rX3BlcmYud2ViY29kZWNzIiwgIm1hc3Rlcj1DaHJvbWl1bVBlcmYmdGVzdD1icm93c2VyX2FjY2Vzc2liaWxpdHlfZXZlbnRzX3N1bSIgXSBleGNsdWRlOiBbICJib3Q9bGFjcm9zLWV2ZS1wZXJmIiwgImJvdD1+YW5kcm9pZC0qIiBdIH0gfSBpbnN0YW5jZTogInY4IiB9")
	require.NoError(t, err)

	mockReturn := []*luciconfig.ProjectConfig{
		{
			Content:  string(config),
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
				Radius:               4,
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

	subscriptionStore.On("GetSubscription", testutils.AnyContext, "a", "abcd").Return(nil, nil)
	subscriptionStore.On("GetSubscription", testutils.AnyContext, "b", "abcd").Return(nil, nil)
	apiClient.On("GetProjectConfigs", testutils.AnyContext, "dummy.path").Return(mockReturn, nil)

	subscriptionStore.On("InsertSubscriptions", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedSubscriptions := args.Get(1).([]*subscription_pb.Subscription)
		if diff := cmp.Diff(expectedSubscriptions, passedSubscriptions, protocmp.Transform()); diff != "" {
			t.Errorf("Subscription protos are no equal:\n%s", diff)
		}
	}).Return(nil)

	alertStore.On("ReplaceAll", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		passedAlerts := args.Get(1).([]*alerts.SaveRequest)
		if diff := cmp.Diff(expectedAlerts, passedAlerts); diff != "" {
			t.Errorf("Alert objects are no equal:\n%s", diff)
		}
	}).Return(nil)

	err = service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}
