package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configApi "go.chromium.org/luci/common/api/luci_config/config/v1"
	luciconfig_mocks "go.skia.org/infra/go/luciconfig/mocks"
	alert_mocks "go.skia.org/infra/perf/go/alerts/mock"
	subscription_mocks "go.skia.org/infra/perf/go/subscription/mocks"
)

func setUp(ctx context.Context, t *testing.T) (*sheriffconfigService, *subscription_mocks.Store, *alert_mocks.Store, *luciconfig_mocks.ApiClient) {
	subscriptionStore := new(subscription_mocks.Store)
	alertStore := new(alert_mocks.Store)
	luciconfigApiClient := new(luciconfig_mocks.ApiClient)
	service, err := New(ctx, subscriptionStore, alertStore, luciconfigApiClient)
	require.NoError(t, err)
	return service, subscriptionStore, alertStore, luciconfigApiClient
}

func TestImportSheriffConfig_InvalidConfig(t *testing.T) {
	ctx := context.Background()

	service, _, _, apiClient := setUp(ctx, t)

	mockReturn := []*configApi.LuciConfigGetConfigMultiResponseMessageConfigEntry{
		{
			Content: "c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKfQ==",
		},
	}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0:")
}

func TestImportSheriffConfig_ValidConfig(t *testing.T) {
	ctx := context.Background()

	service, _, _, apiClient := setUp(ctx, t)

	// Content translates to:
	// subscriptions {
	// 	name: "a"
	// 	contact_email: "test@google.com"
	// 	bug_component: "A>B>C"
	// 	anomaly_configs {
	// 		rules: {
	// 			match: {main: "ChromiumPerf"}
	// 		}
	// 	}
	// }
	mockReturn := []*configApi.LuciConfigGetConfigMultiResponseMessageConfigEntry{
		{
			Content: "c3Vic2NyaXB0aW9ucyB7CgluYW1lOiAiYSIKCWNvbnRhY3RfZW1haWw6ICJ0ZXN0QGdvb2dsZS5jb20iCglidWdfY29tcG9uZW50OiAiQT5CPkMiCglhbm9tYWx5X2NvbmZpZ3MgewoJCXJ1bGVzOiB7CgkJCW1hdGNoOiB7bWFpbjogIkNocm9taXVtUGVyZiJ9CgkJfQoJfQp9",
		},
	}

	apiClient.On("GetProjectConfigs", "dummy.path").Return(mockReturn, nil)

	err := service.ImportSheriffConfig(ctx, "dummy.path")

	require.NoError(t, err)
}
