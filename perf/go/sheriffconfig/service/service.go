package service

import (
	"context"

	"go.skia.org/infra/go/luciconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
	"go.skia.org/infra/perf/go/sheriffconfig/validate"
	"go.skia.org/infra/perf/go/subscription"
)

// Function to address validation requests.
// Simply return the validation error, or nil if there's none.
func ValidateContent(content string) error {
	config, err := validate.DeserializeProto(content)
	if err != nil {
		return skerr.Wrap(err)
	}

	err = validate.ValidateConfig(config)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

type sheriffconfigService struct {
	subscriptionStore   subscription.Store
	alertStore          alerts.Store
	luciconfigApiClient luciconfig.ApiClient
}

// Create new SheriffConfig service.
func New(ctx context.Context,
	subscriptionStore subscription.Store,
	alertStore alerts.Store,
	luciconfigApiClient luciconfig.ApiClient) (*sheriffconfigService, error) {

	if luciconfigApiClient == nil {
		var err error
		luciconfigApiClient, err = luciconfig.NewApiClient(ctx)
		if err != nil {
			return nil, skerr.Fmt("Failed to create new LUCI Config client: %s.", err)
		}
	}

	return &sheriffconfigService{
		subscriptionStore:   subscriptionStore,
		alertStore:          alertStore,
		luciconfigApiClient: luciconfigApiClient,
	}, nil
}

// Fetches specified path config from LUCI Config, transforms it and stores it in the CockroachDB
// in Subscription and Alert tables.
func (s *sheriffconfigService) ImportSheriffConfig(ctx context.Context, path string) error {

	configs, err := s.luciconfigApiClient.GetProjectConfigs(path)
	if err != nil {
		return skerr.Wrap(err)
	}

	if len(configs) == 0 {
		return skerr.Fmt("Couldn't find any configs under path: %s,", path)
	}

	var sheriffconfigs []*pb.SheriffConfig
	for _, config := range configs {
		sheriffconfig, err := validate.DeserializeProto(config.Content)
		if err != nil {
			return skerr.Wrap(err)
		}
		err = validate.ValidateConfig(sheriffconfig)
		if err != nil {
			return skerr.Wrap(err)
		}
		sheriffconfigs = append(sheriffconfigs, sheriffconfig)
	}

	//TODO(eduardoyap): Add logic here to persist validated sheriff configs to database.

	return nil
}
