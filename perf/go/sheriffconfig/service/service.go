package service

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/luciconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
	"go.skia.org/infra/perf/go/sheriffconfig/validate"
	"go.skia.org/infra/perf/go/subscription"
	subscription_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
)

// Custom default values for Alert and Subscription parameters.
const (
	defaultBugPriority = 2
	defaultBugSeverity = 2
	defaultRadius      = 1
	defaultStepUpOnly  = false
	defaultMinimumNum  = 1
	defaultSparse      = false
	defaultK           = 0
	defaultGroupBy     = ""
)

var directionMap = map[string]alerts.Direction{
	"BOTH": alerts.BOTH,
	"UP":   alerts.UP,
	"DOWN": alerts.DOWN,
}

var clusterAlgoMap = map[string]types.RegressionDetectionGrouping{
	"STEPFIT": types.StepFitGrouping,
	"KMEANS":  types.KMeansGrouping,
}

var stepAlgoMap = map[string]types.StepDetection{
	"ORIGINAL_STEP":  types.OriginalStep,
	"ABSOLUTE_STEP":  types.AbsoluteStep,
	"CONST_STEP":     types.Const,
	"PERCENT_STEP":   types.PercentStep,
	"COHEN_STEP":     types.CohenStep,
	"MANN_WHITNEY_U": types.MannWhitneyU,
}

var actionMap = map[string]types.AlertAction{
	"NOACTION": types.NoAction,
	"TRIAGE":   types.FileIssue,
	"BISECT":   types.Bisection,
}

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

	saveRequests := []*alerts.SaveRequest{}
	subscriptions := []*subscription_pb.Subscription{}
	for _, config := range configs {
		ss, srs, err := s.processConfig(config)
		if err != nil {
			return skerr.Wrap(err)
		}
		subscriptions = append(subscriptions, ss...)
		saveRequests = append(saveRequests, srs...)

	}
	if err := s.subscriptionStore.InsertSubscriptions(ctx, subscriptions); err != nil {
		return skerr.Wrap(err)
	}
	if err := s.alertStore.ReplaceAll(ctx, saveRequests); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// processConfig handles validation and transformation of a single config.
func (s *sheriffconfigService) processConfig(config *luciconfig.ProjectConfig) ([]*subscription_pb.Subscription, []*alerts.SaveRequest, error) {
	// Validate and deserialize config content
	sheriffconfig, err := validate.DeserializeProto(config.Content)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if err := validate.ValidateConfig(sheriffconfig); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	// Prepare subscription and alert data
	subscriptions := makeSubscriptions(sheriffconfig, config.Revision)
	saveRequests := makeSaveRequests(sheriffconfig.Subscriptions, config.Revision)

	return subscriptions, saveRequests, nil
}

// Create subscription entities to be inserted into DB based on Sheriff Config protos.
func makeSubscriptions(sheriffConfig *pb.SheriffConfig, revision string) []*subscription_pb.Subscription {
	subscriptions := []*subscription_pb.Subscription{}

	for _, subscription := range sheriffConfig.Subscriptions {
		subscriptionEntity := &subscription_pb.Subscription{
			Name:         subscription.Name,
			ContactEmail: subscription.ContactEmail,
			BugLabels:    subscription.BugLabels,
			Hotlists:     subscription.HotlistLabels,
			BugComponent: subscription.BugComponent,
			BugPriority:  getPriorityFromProto(subscription.BugPriority),
			BugSeverity:  getSeverityFromProto(subscription.BugSeverity),
			BugCcEmails:  subscription.BugCcEmails,
			Revision:     revision,
		}
		subscriptions = append(subscriptions, subscriptionEntity)
	}
	return subscriptions
}

// Create SaveRequest objects to be inserted into Alerts DB table.
func makeSaveRequests(subscriptions []*pb.Subscription, revision string) []*alerts.SaveRequest {

	saveRequests := []*alerts.SaveRequest{}
	for _, subscription := range subscriptions {
		for _, anomalyConfig := range subscription.AnomalyConfigs {
			for _, match := range anomalyConfig.Rules.Match {

				query := buildQueryFromRules(match, anomalyConfig.Rules.Exclude)
				cfg := createAlert(query, anomalyConfig, subscription, revision)

				saveRequest := &alerts.SaveRequest{
					Cfg: cfg,
					SubKey: &alerts.SubKey{
						SubName:     subscription.Name,
						SubRevision: revision,
					},
				}
				saveRequests = append(saveRequests, saveRequest)
			}
		}
	}
	return saveRequests
}

// Create Alert object.
func createAlert(query string, anomalyConfig *pb.AnomalyConfig, subscription *pb.Subscription, revision string) *alerts.Alert {
	// Apply defaults
	radius := defaultRadius
	if anomalyConfig.Radius != nil {
		radius = int(*anomalyConfig.Radius)
	}
	minimumNum := defaultMinimumNum
	if anomalyConfig.MinimumNum != nil {
		minimumNum = int(*anomalyConfig.MinimumNum)
	}
	sparse := defaultSparse
	if anomalyConfig.Sparse != nil {
		sparse = *anomalyConfig.Sparse
	}
	k := defaultK
	if anomalyConfig.K != nil {
		k = int(*anomalyConfig.K)
	}
	groupBy := defaultGroupBy
	if anomalyConfig.GroupBy != nil {
		groupBy = *anomalyConfig.GroupBy
	}

	cfg := &alerts.Alert{
		IDAsString:  "-1",
		DisplayName: query,
		Query:       query,
		Alert:       subscription.ContactEmail,
		Interesting: anomalyConfig.Threshold,
		Algo:        clusterAlgoMap[anomalyConfig.Algo.String()],
		Step:        stepAlgoMap[anomalyConfig.Step.String()],

		StateAsString: alerts.ACTIVE,
		Owner:         subscription.ContactEmail,
		StepUpOnly:    defaultStepUpOnly,

		DirectionAsString: directionMap[anomalyConfig.Direction.String()],
		Radius:            int(radius),
		K:                 k,
		GroupBy:           groupBy,
		Sparse:            sparse,
		MinimumNum:        minimumNum,

		Action: actionMap[anomalyConfig.Action.String()],

		SubscriptionName:     subscription.Name,
		SubscriptionRevision: revision,
	}
	return cfg
}

// Create query based on Sheriff Config rules.
func buildQueryFromRules(match *pb.Pattern, excludes []*pb.Pattern) string {
	var queryParts []string

	addValue := func(key string, value string, exclude bool) {
		if value != "" {
			if exclude {
				value = fmt.Sprintf("!%s", value)
			}
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// TODO(eduardoyap): Change schema to not have hardcoded values.
	// Instead use a flexible format that allows user to specify selection as a query.
	allPatterns := append([]*pb.Pattern{match}, excludes...)
	for _, pattern := range allPatterns {
		addValue("master", pattern.Master, pattern != match)
		addValue("bot", pattern.Bot, pattern != match)
		addValue("benchmark", pattern.Benchmark, pattern != match)
		addValue("test", pattern.Test, pattern != match)
		addValue("subtest_1", pattern.Subtest_1, pattern != match)
		addValue("subtest_2", pattern.Subtest_2, pattern != match)
		addValue("subtest_3", pattern.Subtest_3, pattern != match)
	}

	return strings.Join(queryParts, "&")
}

func getPriorityFromProto(pri pb.Subscription_Priority) int32 {
	if int32(pri) == 0 {
		return defaultBugPriority
	}
	return int32(pri) - 1
}

func getSeverityFromProto(sev pb.Subscription_Severity) int32 {
	if int32(sev) == 0 {
		return defaultBugSeverity
	}
	return int32(sev) - 1
}
