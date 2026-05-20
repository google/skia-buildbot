package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/luciconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/alerts"
	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
	"go.skia.org/infra/perf/go/sheriffconfig/validate"
	"go.skia.org/infra/perf/go/subscription"
	subscription_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/encoding/prototext"
)

// Custom default values for Alert and Subscription parameters.
const (
	defaultBugPriority = 2
	defaultBugSeverity = 2
	defaultRadius      = 4 // minimal for Cohen and medians computation
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
	"STEPINESS":      types.Stepiness,
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

// ConfigProvider fetches project configurations from a backing store (e.g., Gitiles or LUCI Config).
type ConfigProvider interface {
	// Given a config path, retrieve all matching configs to that path.
	GetProjectConfigs(ctx context.Context, path string) ([]*luciconfig.ProjectConfig, error)
}

type gitilesConfigProvider struct {
	repo              gitiles.GitilesRepo
	sheriffConfigPath string
}

func NewGitilesConfigProvider(repo gitiles.GitilesRepo, sheriffConfigPath string) ConfigProvider {
	return &gitilesConfigProvider{
		repo:              repo,
		sheriffConfigPath: sheriffConfigPath,
	}
}

func (c *gitilesConfigProvider) GetProjectConfigs(ctx context.Context, path string) ([]*luciconfig.ProjectConfig, error) {
	if path != "skia-sheriff-configs.cfg" {
		sklog.Infof("Unsupported path for gitilesConfigProvider: %s", path)
		return nil, skerr.Fmt("Unsupported path for gitilesConfigProvider: %s", path)
	}

	repoPath := c.sheriffConfigPath
	sklog.Debugf("Fetching sheriff config from gitiles repo path: %s", repoPath)

	commits, err := c.repo.Log(ctx, git.MainBranch, gitiles.LogPath(repoPath), gitiles.LogLimit(1))
	if err != nil {
		sklog.Warningf("Failed to get gitiles log for %s: %s", repoPath, err)
		return nil, skerr.Wrap(err)
	}
	if len(commits) == 0 {
		err := skerr.Fmt("No commit history found for %s", repoPath)
		sklog.Warningf(err.Error())
		return nil, err
	}
	revision := commits[0].Hash

	b, err := c.repo.ReadFileAtRef(ctx, repoPath, revision)
	if err != nil {
		// Gitiles can return a massive stack trace on 404s. Truncate it if it's too long.
		errMsg := err.Error()
		if len(errMsg) > 400 {
			errMsg = errMsg[:400] + "... (truncated)"
		}
		sklog.Warningf("Failed to read file %s from gitiles at revision %s: %s", repoPath, revision, errMsg)
		// Return a new error with the truncated message so callers don't log the massive stack trace
		return nil, skerr.Fmt("gitiles fetch failed: %s", errMsg)
	}

	content := string(b)
	if err := ValidateContent(content); err != nil {
		sklog.Warningf("Invalid sheriff config from gitiles: %s", err)
		return nil, skerr.Wrapf(err, "invalid sheriff config from gitiles")
	}

	sklog.Infof("Successfully fetched and validated sheriff config from gitiles at revision: %s", revision)

	return []*luciconfig.ProjectConfig{
		{
			Content:  content,
			Revision: revision,
		},
	}, nil
}

type migrationConfigProvider struct {
	primary  ConfigProvider
	fallback ConfigProvider
}

func NewMigrationConfigProvider(primary, fallback ConfigProvider) ConfigProvider {
	return &migrationConfigProvider{
		primary:  primary,
		fallback: fallback,
	}
}

func (m *migrationConfigProvider) GetProjectConfigs(ctx context.Context, path string) ([]*luciconfig.ProjectConfig, error) {
	configs, err := m.primary.GetProjectConfigs(ctx, path)
	if err != nil {
		sklog.Warningf("Primary config provider failed (path: %s): %s. Falling back to secondary.", path, err)
		return m.fallback.GetProjectConfigs(ctx, path)
	}
	return configs, nil
}

type localFileConfigProvider struct {
	path string
}

func (l *localFileConfigProvider) GetProjectConfigs(ctx context.Context, path string) ([]*luciconfig.ProjectConfig, error) {
	b, err := os.ReadFile(l.path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	content := string(b)
	if err := ValidateContent(content); err != nil {
		return nil, skerr.Wrap(err)
	}
	sklog.Infof("Validation passed for local file: %s", l.path)
	return []*luciconfig.ProjectConfig{
		{
			Content:  content,
			Revision: "local-file-revision",
		},
	}, nil
}

// CreateConfigProvider creates a ConfigProvider based on the given configuration.
func CreateConfigProvider(ctx context.Context, isLocal bool, gitilesRepoUrl string, sheriffConfigPath string, fallbackToLucicfg bool) (ConfigProvider, error) {
	if isLocal && gitilesRepoUrl == "" && sheriffConfigPath != "" {
		sklog.Infof("Using local file for Sheriff Configs: %s", sheriffConfigPath)
		return &localFileConfigProvider{path: sheriffConfigPath}, nil
	}

	var primaryClient ConfigProvider
	var fallbackClient ConfigProvider
	var primaryErr error
	var fallbackErr error

	if gitilesRepoUrl != "" && sheriffConfigPath != "" {
		// Setup primary Gitiles client
		ts, err := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
		if err != nil {
			primaryErr = skerr.Wrapf(err, "Failed to create token source for gitiles")
		} else {
			client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
			repo := gitiles.NewRepo(gitilesRepoUrl, client)
			primaryClient = NewGitilesConfigProvider(repo, sheriffConfigPath)
		}
	} else {
		primaryErr = skerr.Fmt("gitilesRepoUrl or sheriffConfigPath is empty")
	}

	if fallbackToLucicfg {
		// Setup fallback LUCI Config client
		var err error
		fallbackClient, err = luciconfig.NewApiClient(ctx, isLocal)
		if err != nil {
			fallbackErr = skerr.Wrapf(err, "Failed to build LUCI Config fallback client")
		}
	} else {
		fallbackErr = skerr.Fmt("fallbackToLucicfg is false")
	}

	// Determine the final config provider structure
	if primaryClient != nil && fallbackClient != nil {
		sklog.Infof("Using Gitiles Config with LUCI Config fallback for Sheriff Configs (Migration Mode)")
		return NewMigrationConfigProvider(primaryClient, fallbackClient), nil
	} else if primaryClient != nil {
		sklog.Infof("Using Gitiles Config for Sheriff Configs")
		return primaryClient, nil
	} else if fallbackClient != nil {
		sklog.Infof("Using LUCI Config for Sheriff Configs")
		return fallbackClient, nil
	}

	return nil, skerr.Fmt("Failed to create config provider: primary error: %v, fallback error: %v", primaryErr, fallbackErr)
}

type sheriffconfigService struct {
	db                pool.Pool
	subscriptionStore subscription.Store
	alertStore        alerts.Store
	configProvider    ConfigProvider
	instance          string
}

// Create new SheriffConfig service.
func New(ctx context.Context,
	db pool.Pool,
	subscriptionStore subscription.Store,
	alertStore alerts.Store,
	configProvider ConfigProvider,
	instance string) (*sheriffconfigService, error) {

	if configProvider == nil {
		var err error
		configProvider, err = luciconfig.NewApiClient(ctx, false)
		if err != nil {
			return nil, skerr.Fmt("Failed to create new LUCI Config client: %s.", err)
		}
	}

	return &sheriffconfigService{
		db:                db,
		subscriptionStore: subscriptionStore,
		alertStore:        alertStore,
		configProvider:    configProvider,
		instance:          instance,
	}, nil
}

// Fetches specified path config from LUCI Config, transforms it and stores it in the Spanner
// in Subscription and Alert tables.
func (s *sheriffconfigService) ImportSheriffConfig(ctx context.Context, path string) error {

	configs, err := s.configProvider.GetProjectConfigs(ctx, path)
	if err != nil {
		return skerr.Wrap(err)
	}

	if len(configs) == 0 {
		return skerr.Fmt("Couldn't find any configs under path: %s,", path)
	}

	saveRequests := []*alerts.SaveRequest{}
	subscriptions := []*subscription_pb.Subscription{}

	for _, config := range configs {
		ss, srs, err := s.processConfig(ctx, config)
		if err != nil {
			return skerr.Wrap(err)
		}
		subscriptions = append(subscriptions, ss...)
		saveRequests = append(saveRequests, srs...)

	}

	// Insert subscriptions and alerts in 1 transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	if len(subscriptions) != 0 {
		if err := s.subscriptionStore.InsertSubscriptions(ctx, subscriptions, tx); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return err
		}
	}

	if len(saveRequests) != 0 {
		if err := s.alertStore.ReplaceAll(ctx, saveRequests, tx); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Imported %d changed subscriptions and %d changed alerts.", len(subscriptions), len(saveRequests))
	return nil
}

// processConfig handles validation and transformation of a single config.
func (s *sheriffconfigService) processConfig(ctx context.Context, config *luciconfig.ProjectConfig) ([]*subscription_pb.Subscription, []*alerts.SaveRequest, error) {
	// Validate and deserialize config content
	sheriffconfig := &pb.SheriffConfig{}
	err := prototext.Unmarshal([]byte(config.Content), sheriffconfig)
	if err != nil {
		return nil, nil, skerr.Fmt("Failed to unmarshal prototext: %s", err)
	}
	if err := validate.ValidateConfig(sheriffconfig); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	subscriptionEntities := []*subscription_pb.Subscription{}
	saveRequests := []*alerts.SaveRequest{}

	instanceSubCount := 0
	totalAlertsCount := 0

	// Prepare subscription and alert data
	for _, subscription := range sheriffconfig.Subscriptions {
		if subscription.Instance != s.instance {
			continue
		}
		instanceSubCount++
		for _, anomalyConfig := range subscription.AnomalyConfigs {
			totalAlertsCount += len(anomalyConfig.Rules.Match)
		}

		subscriptionEntity := makeSubscriptionEntity(subscription, config.Revision)

		// We check if the entity already exists in the DB. If it is, there's no need to re-insert it.
		// We only want to update the DB when there's an actual revision change in the configurations.
		sub, err := s.subscriptionStore.GetSubscription(ctx, subscriptionEntity.Name, subscriptionEntity.Revision)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		if sub == nil {
			subscriptionEntities = append(subscriptionEntities, subscriptionEntity)

			subSaveRequests, err := makeSaveRequests(subscription, config.Revision)
			if err != nil {
				return nil, nil, skerr.Wrap(err)
			}
			saveRequests = append(saveRequests, subSaveRequests...)
		}
	}

	unchangedSubs := instanceSubCount - len(subscriptionEntities)
	unchangedAlerts := totalAlertsCount - len(saveRequests)
	sklog.Infof("Found %d subscriptions with %d alerts for this instance (%s), %d/%d subscriptions and %d/%d alerts unchanged",
		instanceSubCount, totalAlertsCount, s.instance, unchangedSubs, instanceSubCount, unchangedAlerts, totalAlertsCount)

	return subscriptionEntities, saveRequests, nil
}

// makeSubscriptionEntity creates subscription entitiy to be inserted into DB based on Sheriff Config protos.
func makeSubscriptionEntity(subscription *pb.Subscription, revision string) *subscription_pb.Subscription {
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

	return subscriptionEntity
}

// makeSaveRequests creates SaveRequest objects for a given subscription to be inserted into Alerts DB table.
func makeSaveRequests(subscription *pb.Subscription, revision string) ([]*alerts.SaveRequest, error) {

	saveRequests := []*alerts.SaveRequest{}
	for _, anomalyConfig := range subscription.AnomalyConfigs {
		for _, match := range anomalyConfig.Rules.Match {

			query, err := buildQueryFromRules(match, anomalyConfig.Rules.Exclude)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
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

	return saveRequests, nil
}

// createAlert creates Alert object.
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

		DetectionRule: parseAnomalyDetectionRule(anomalyConfig.DetectionRule),

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

// parseAnomalyDetectionRule converts the protobuf AnomalyDetectionRule to the alerts.AnomalyDetectionRule domain model.
func parseAnomalyDetectionRule(rule *pb.AnomalyDetectionRule) *alerts.AnomalyDetectionRule {
	if rule == nil {
		return nil
	}

	ret := &alerts.AnomalyDetectionRule{}
	if cr := rule.GetComplexRule(); cr != nil {
		alertsRules := make([]*alerts.AnomalyDetectionRule, 0, len(cr.Rules))
		for _, r := range cr.Rules {
			alertsRules = append(alertsRules, parseAnomalyDetectionRule(r))
		}
		ret.ComplexRule = &alerts.ComplexRule{
			Op:    cr.Op.String(),
			Rules: alertsRules,
		}
	} else if sr := rule.GetSimpleRule(); sr != nil {
		ret.SimpleRule = &alerts.AlgorithmCheck{
			Step:      stepAlgoMap[sr.Step.String()],
			Threshold: sr.Threshold,
		}
	}

	return ret
}

// buildQueryFromRules creates query based on Sheriff Config rules.
func buildQueryFromRules(match string, excludes []string) (string, error) {
	var queryParts []string
	matchQuery, err := url.ParseQuery(match)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	for key, values := range matchQuery {
		for _, value := range values {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, value))
		}

	}

	for _, exclude := range excludes {
		excludeQuery, err := url.ParseQuery(exclude)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		for key, values := range excludeQuery {
			// Use values[0] as there should only be 1 valid key value pair in an exclude pattern.
			queryParts = append(queryParts, fmt.Sprintf("%s=!%s", key, values[0]))
		}
	}
	sort.Strings(queryParts)
	return strings.Join(queryParts, "&"), nil
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

func (s *sheriffconfigService) StartImportRoutine(period time.Duration) {
	go func() {
		s.ImportSheriffConfigOnce()
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for range ticker.C {
			s.ImportSheriffConfigOnce()
		}
	}()
}

func (s *sheriffconfigService) ImportSheriffConfigOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sklog.Infof("Importing sheriff configs.")
	if err := s.ImportSheriffConfig(ctx, "skia-sheriff-configs.cfg"); err != nil {
		sklog.Errorf("Failed to import configs: %s", err)
	}
}
