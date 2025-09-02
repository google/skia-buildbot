package config

// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=paths=source_relative --go_out=. ./config.proto
//go:generate rm -rf ./go.skia.org
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w config.pb.go
//go:generate bazelisk run --config=mayberemote //:protoc -- --twirp_typescript_out=../../modules/config ./config.proto

import (
	"context"
	"regexp"
	"strings"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/time_window"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	// MaxRollerNameLength is the maximum roller name length. This is limited
	// by Kubernetes, which has a 63-character limit for various names. This
	// length is derived from that limit, accounting for the prefixes and
	// suffixes which are automatically added by our tooling, eg. the
	// "autoroll-be-" prefix and "-storage" suffix for disks, controller hashes,
	// etc.
	MaxRollerNameLength = 41

	// DefaultSafetyThrottleAttemptCount is the default attempt count for
	// safety throttling.
	DefaultSafetyThrottleAttemptCount = 3
	// DefaultSafetyThrottleTimeWindow is the default time window for safety
	// throttling.
	DefaultSafetyThrottleTimeWindow = "30m"
)

var (
	// DefaultSafetyThrottleConfig is the default safety throttling config.
	DefaultSafetyThrottleConfig = &ThrottleConfig{
		AttemptCount: DefaultSafetyThrottleAttemptCount,
		TimeWindow:   DefaultSafetyThrottleTimeWindow,
	}

	// ValidK8sLabel matches valid labels for Kubernetes.
	ValidK8sLabel = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)

	// Gerrit hosts used by Chromium projects.
	chromiumGerritHosts = []string{
		"https://chromium-review.googlesource.com",
		"https://chrome-internal-review.googlesource.com",
	}
	// Valid Gerrit configs for Chromium projects.
	validChromiumGerritConfigs = []GerritConfig_Config{
		GerritConfig_CHROMIUM_BOT_COMMIT,
		GerritConfig_CHROMIUM_BOT_COMMIT_NO_CQ,
		GerritConfig_CHROMIUM_NO_CR,
	}
)

// RepoManagerConfig provides configuration information for RepoManagers.
type RepoManagerConfig interface {
	util.Validator

	// Return the default NextRollStrategy name.
	DefaultStrategy() string

	// Return true if the RepoManager does not use a local checkout.
	NoCheckout() bool

	// Return the list of valid NextRollStrategy names for this RepoManager.
	ValidStrategies() []string
}

// Validate implements util.Validator.
func (s PreUploadStep) Validate() error {
	if _, ok := PreUploadStep_name[int32(s)]; !ok {
		return skerr.Fmt("Unknown pre-upload step: %v", s)
	}
	return nil
}

// Validate implements util.Validator.
func (c CommitMsgConfig_BuiltIn) Validate() error {
	if _, ok := CommitMsgConfig_BuiltIn_name[int32(c)]; !ok {
		return skerr.Fmt("Unknown CommitMsgConfig_BuiltIn: %v", c)
	}
	return nil
}

// Validate implements util.Validator.
func (c GerritConfig_Config) Validate() error {
	if _, ok := GerritConfig_Config_name[int32(c)]; !ok {
		return skerr.Fmt("Unknown GerritConfig_Config: %v", c)
	}
	return nil
}

// Validate implements util.Validator.
func (c NotifierConfig_LogLevel) Validate() error {
	if _, ok := NotifierConfig_LogLevel_name[int32(c)]; !ok {
		return skerr.Fmt("Unknown NotifierConfig_LogLevel: %v", c)
	}
	return nil
}

// Validate implements util.Validator.
func (c NotifierConfig_MsgType) Validate() error {
	if _, ok := NotifierConfig_MsgType_name[int32(c)]; !ok {
		return skerr.Fmt("Unknown NotifierConfig_MsgType: %v", c)
	}
	return nil
}

// Validate implements util.Validator.
func (c *Config) Validate() error {
	if c.RollerName == "" {
		return skerr.Fmt("RollerName is required.")
	}
	if len(c.RollerName) > MaxRollerNameLength {
		return skerr.Fmt("RollerName length %d is greater than maximum %d", len(c.RollerName), MaxRollerNameLength)
	}
	if !ValidK8sLabel.MatchString(c.RollerName) {
		return skerr.Fmt("RollerName is invalid.")
	}
	if c.ChildDisplayName == "" {
		return skerr.Fmt("ChildDisplayName is required.")
	}
	if c.ParentDisplayName == "" {
		return skerr.Fmt("ParentDisplayName is required.")
	}
	if c.ParentWaterfall == "" {
		return skerr.Fmt("ParentWaterfall is required.")
	}
	if !ValidK8sLabel.MatchString(c.OwnerPrimary) {
		return skerr.Fmt("OwnerPrimary is invalid.")
	}
	if !ValidK8sLabel.MatchString(c.OwnerSecondary) {
		return skerr.Fmt("OwnerSecondary is invalid.")
	}
	if len(c.Contacts) < 1 {
		return skerr.Fmt("At least one contact is required.")
	}
	if c.ServiceAccount == "" {
		return skerr.Fmt("ServiceAccount is required.")
	}
	if c.Reviewer == nil || len(c.Reviewer) == 0 {
		return skerr.Fmt("Reviewer is required.")
	}
	if _, err := time_window.Parse(c.TimeWindow); err != nil {
		return skerr.Wrapf(err, "TimeWindow is invalid")
	}

	if c.CommitMsg == nil {
		return skerr.Fmt("CommitMsg is required")
	}
	if err := c.CommitMsg.Validate(); err != nil {
		return skerr.Wrap(err)
	}

	cr := []util.Validator{}
	if c.GetGerrit() != nil {
		cr = append(cr, c.GetGerrit())
		gerrit := c.GetGerrit()
		if gerrit != nil && util.In(gerrit.Url, chromiumGerritHosts) {
			isValid := false
			for _, gc := range validChromiumGerritConfigs {
				if gc == gerrit.Config {
					isValid = true
					break
				}
			}
			if !isValid {
				validConfigsStr := make([]string, 0, len(validChromiumGerritConfigs))
				for _, gc := range validChromiumGerritConfigs {
					validConfigsStr = append(validConfigsStr, gc.String())
				}
				return skerr.Fmt("Chromium rollers must use one of the following Gerrit configs: %v", validConfigsStr)
			}
		}
	}
	if c.GetGithub() != nil {
		cr = append(cr, c.GetGithub())
	}
	if c.GetGoogle3() != nil {
		cr = append(cr, c.GetGoogle3())
	}
	if len(cr) != 1 {
		return skerr.Fmt("Exactly one of Gerrit, Github, or Google3 is required.")
	}
	if err := cr[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	if c.Kubernetes == nil {
		return skerr.Fmt("Kubernetes config is required.")
	}
	if err := c.Kubernetes.Validate(); err != nil {
		return skerr.Wrapf(err, "kubernetes validation failed")
	}

	rm := []RepoManagerConfig{}
	var childTransitiveDeps []*VersionFileConfig
	var parentTransitiveDeps []*TransitiveDepConfig
	if c.GetParentChildRepoManager() != nil {
		pc := c.GetParentChildRepoManager()
		rm = append(rm, pc)
		if p := pc.GetDepsLocalGerritParent(); p != nil {
			parentTransitiveDeps = p.DepsLocal.GitCheckout.Dep.Transitive
		} else if p := pc.GetDepsLocalGithubParent(); p != nil {
			parentTransitiveDeps = p.DepsLocal.GitCheckout.Dep.Transitive
		} else if p := pc.GetGitCheckoutGithubFileParent(); p != nil {
			parentTransitiveDeps = p.GitCheckout.GitCheckout.Dep.Transitive
		} else if p := pc.GetGitilesParent(); p != nil {
			parentTransitiveDeps = p.Dep.Transitive
		} else if p := pc.GetGitCheckoutGerritParent(); p != nil {
			parentTransitiveDeps = p.GitCheckout.Dep.Transitive
		}
		if c := pc.GetGitCheckoutChild(); c != nil {
			childTransitiveDeps = c.GitCheckout.Dependencies
		} else if c := pc.GetGitCheckoutGithubChild(); c != nil {
			childTransitiveDeps = c.GitCheckout.GitCheckout.Dependencies
		} else if c := pc.GetGitilesChild(); c != nil {
			childTransitiveDeps = c.Gitiles.Dependencies
		}
	}
	if c.GetAndroidRepoManager() != nil {
		rm = append(rm, c.GetAndroidRepoManager())
	}
	if c.GetCommandRepoManager() != nil {
		rm = append(rm, c.GetCommandRepoManager())
	}
	if c.GetFreetypeRepoManager() != nil {
		rm = append(rm, c.GetFreetypeRepoManager())
	}
	if c.GetGoogle3RepoManager() != nil {
		rm = append(rm, c.GetGoogle3RepoManager())
	}
	if len(rm) != 1 {
		return skerr.Fmt("Exactly one RepoManager is required.")
	}
	if err := rm[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	isNoCheckout := rm[0].NoCheckout()
	if isNoCheckout && c.Kubernetes.Disk != "" {
		return skerr.Fmt("kubernetes.disk is not valid for no-checkout repo managers.")
	} else if !isNoCheckout && c.Kubernetes.Disk == "" {
		return skerr.Fmt("kubernetes.disk is required for repo managers which use a checkout.")
	}

	// Verify that the notifier configs are valid.
	for _, nc := range c.Notifiers {
		if err := nc.Validate(); err != nil {
			return skerr.Wrapf(err, "notifier config failed validation")
		}
	}

	if c.SafetyThrottle != nil {
		if err := c.SafetyThrottle.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}

	if len(c.TransitiveDeps) != len(parentTransitiveDeps) {
		return skerr.Fmt("top level transitive dependency count %d does not match transitive dependency count %d set on parent", len(c.TransitiveDeps), len(parentTransitiveDeps))
	}
	if len(c.TransitiveDeps) != len(childTransitiveDeps) {
		return skerr.Fmt("top level transitive dependency count %d does not match dependency count %d set on child", len(c.TransitiveDeps), len(childTransitiveDeps))
	}
	// Note: this assumes that the dependencies are listed in the same order.
	for idx, td := range c.TransitiveDeps {
		if err := td.Validate(); err != nil {
			return skerr.Wrapf(err, "transitive dependency config failed validation")
		}
		parentDep := parentTransitiveDeps[idx]
		if !deepequal.DeepEqual(td, parentDep) {
			return skerr.Fmt("top level transitive dependency differs from transitive dependency set on parent: %s", assertdeep.Diff(td, parentDep))
		}
		childDep := childTransitiveDeps[idx]
		if !deepequal.DeepEqual(td.Child, childDep) {
			return skerr.Fmt("top level transitive dependency differs from dependency set on child: %s", assertdeep.Diff(td.Child, childDep))
		}
	}

	return nil
}

// GetRepoManagerConfig returns the RepoManager config for the roller.
func (c *Config) GetRepoManagerConfig() RepoManagerConfig {
	if c.GetParentChildRepoManager() != nil {
		return c.GetParentChildRepoManager()
	}
	if c.GetAndroidRepoManager() != nil {
		return c.GetAndroidRepoManager()
	}
	if c.GetCommandRepoManager() != nil {
		return c.GetCommandRepoManager()
	}
	if c.GetFreetypeRepoManager() != nil {
		return c.GetFreetypeRepoManager()
	}
	if c.GetGoogle3RepoManager() != nil {
		return c.GetGoogle3RepoManager()
	}
	return nil
}

// ValidStrategies returns the valid strategies for this roller.
func (c *Config) ValidStrategies() []string {
	return c.GetRepoManagerConfig().ValidStrategies()
}

// DefaultStrategy returns the default strategy for this roller.
func (c *Config) DefaultStrategy() string {
	return c.GetRepoManagerConfig().DefaultStrategy()
}

// trybots may be specified as "luci.<project>.<bucket>:<builder>" or
// <project>/<bucket>:<builder>.
var trybotProjectBucketRegex = regexp.MustCompile(`^(?P<project>[a-zA-Z0-9_-]+)(?:\.|\/)(?P<bucket>[a-zA-Z0-9._-]+)$`)

// ParseTrybotName parses a trybot name, eg. "luci.chromium.try:some-trybot",
// and returns its project, bucket, and builder names, or any error which
// occurred. Requires a config_vars.Registry instance because trybot names might
// be templated.
func ParseTrybotName(reg *config_vars.Registry, trybot string) (string, string, []string, error) {
	tmpl, err := config_vars.NewTemplate(trybot)
	if err != nil {
		return "", "", nil, skerr.Wrap(err)
	}
	if err := reg.Register(tmpl); err != nil {
		return "", "", nil, skerr.Wrap(err)
	}
	trybot = strings.TrimPrefix(tmpl.String(), "luci.")
	split := strings.SplitN(trybot, ":", 2)
	if len(split) != 2 {
		return "", "", nil, skerr.Fmt("invalid trybot name %q, expected a colon", trybot)
	}
	m := trybotProjectBucketRegex.FindStringSubmatch(split[0])
	if len(m) == 0 {
		return "", "", nil, skerr.Fmt("invalid trybot project/bucket %q, expected `%s`", split[0], trybotProjectBucketRegex.String())
	}
	builders := strings.Split(split[1], ",")
	return m[trybotProjectBucketRegex.SubexpIndex("project")], m[trybotProjectBucketRegex.SubexpIndex("bucket")], builders, nil
}

// Validate implements util.Validator.
func (c *CommitMsgConfig) Validate() error {
	cbc := &mocks.Client{}
	fakeVars := config_vars.FakeVars()
	cbc.On("Get", mock.Anything).Return(fakeVars.Branches.Chromium, fakeVars.Branches.ActiveMilestones, nil)
	reg, err := config_vars.NewRegistry(context.TODO(), cbc)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, trybot := range c.CqExtraTrybots {
		if _, _, _, err := ParseTrybotName(reg, trybot); err != nil {
			return skerr.Wrap(err)
		}
	}
	// TODO(borenet): We should be ensuring that ChildLogUrlTmpl matches the
	// actual child repo.
	// TODO(borenet): We should be checking the ExtraFooters to ensure that they
	// are formatted correctly (see git.GetFootersMap).
	// TODO(borenet): The commit_msg package has a test which uses fake data to
	// execute the template and ensure that it is valid. We should make use of
	// that somehow.
	return nil
}

// Validate implements util.Validator.
func (c *GerritConfig) Validate() error {
	if c.Url == "" {
		return skerr.Fmt("URL is required.")
	}
	if c.Project == "" {
		return skerr.Fmt("Project is required.")
	}
	if _, ok := GerritConfig_Config_name[int32(c.Config)]; !ok {
		return skerr.Fmt("Unknown config: %v", c.Config)
	}
	return nil
}

// CanQueryTrybots implements CodeReviewConfig.
func (c *GerritConfig) CanQueryTrybots() bool {
	return c.Config != GerritConfig_ANDROID && c.Config != GerritConfig_ANDROID_NO_CR && c.Config != GerritConfig_ANDROID_NO_CR_NO_PR
}

// Validate implements util.Validator.
func (c *GitHubConfig) Validate() error {
	if c.RepoOwner == "" {
		return skerr.Fmt("RepoOwner is required.")
	}
	if c.RepoName == "" {
		return skerr.Fmt("RepoName is required.")
	}
	if c.TokenSecret == "" {
		return skerr.Fmt("TokenSecret is required.")
	}
	if c.SshKeySecret == "" {
		return skerr.Fmt("SshKeySecret is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *Google3Config) Validate() error {
	return nil
}

// Validate implements util.Validator.
func (c *KubernetesConfig) Validate() error {
	if c.Cpu == "" {
		return skerr.Fmt("CPU is required.")
	}
	if c.Memory == "" {
		return skerr.Fmt("Memory is required.")
	}
	if c.Image == "" {
		return skerr.Fmt("Image is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *NotifierConfig) Validate() error {
	if _, ok := NotifierConfig_LogLevel_name[int32(c.LogLevel)]; !ok {
		return skerr.Fmt("Unknown LogLevel: %v", c.LogLevel)
	}
	for _, msgType := range c.MsgType {
		if _, ok := NotifierConfig_MsgType_name[int32(msgType)]; !ok {
			return skerr.Fmt("Unknown MsgType: %v", c.MsgType)
		}
	}
	if len(c.MsgType) != 0 && c.LogLevel != 0 {
		return skerr.Fmt("LogLevel and MsgType are mutually exclusive")
	}
	cfg := []util.Validator{}
	if c.GetChat() != nil {
		cfg = append(cfg, c.GetChat())
	}
	if c.GetEmail() != nil {
		cfg = append(cfg, c.GetEmail())
	}
	if c.GetMonorail() != nil {
		cfg = append(cfg, c.GetMonorail())
	}
	if c.GetPubsub() != nil {
		cfg = append(cfg, c.GetPubsub())
	}
	if len(cfg) != 1 {
		return skerr.Fmt("Exactly one notifier type is required.")
	}
	return cfg[0].Validate()
}

// Validate implements util.Validator.
func (c *ChatNotifierConfig) Validate() error {
	if c.RoomId == "" {
		return skerr.Fmt("RoomId is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *EmailNotifierConfig) Validate() error {
	if len(c.Emails) == 0 {
		return skerr.Fmt("Emails are required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *MonorailNotifierConfig) Validate() error {
	if c.Project == "" {
		return skerr.Fmt("Project is required.")
	}
	if c.Owner == "" {
		return skerr.Fmt("Owner is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *PubSubNotifierConfig) Validate() error {
	if len(c.Topic) == 0 {
		return skerr.Fmt("Topic is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *ThrottleConfig) Validate() error {
	if c.AttemptCount < 1 {
		return skerr.Fmt("AttemptCount must be greater than zero.")
	}
	if c.TimeWindow == "" {
		return skerr.Fmt("TimeWindow must be greater than zero.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *TransitiveDepConfig) Validate() error {
	if c.Child == nil {
		return skerr.Fmt("Child is required.")
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Parent == nil {
		return skerr.Fmt("Parent is required.")
	}
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *VersionFileConfig) Validate() error {
	if c.Id == "" {
		return skerr.Fmt("Id is required.")
	}
	if len(c.File) == 0 {
		return skerr.Fmt("File is required.")
	}
	for _, file := range c.File {
		if file.Path == "" {
			return skerr.Fmt("Path is required.")
		}
		if file.Regex != "" {
			if _, err := regexp.Compile(file.Regex); err != nil {
				return skerr.Wrapf(err, "Invalid Regex")
			}
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *Google3RepoManagerConfig) Validate() error {
	if c.ChildBranch == "" {
		return skerr.Fmt("ChildBranch is required.")
	}
	if c.ChildRepo == "" {
		return skerr.Fmt("ChildRepo is required.")
	}
	return nil
}

// DefaultStrategy implements RepoManagerConfig.
func (c *Google3RepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *Google3RepoManagerConfig) NoCheckout() bool {
	return false
}

// ValidStrategies implements RepoManagerConfig.
func (c *Google3RepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// Validate implements util.Validator.
func (c *AndroidRepoManagerConfig) Validate() error {
	if c.ChildRepoUrl == "" {
		return skerr.Fmt("ChildRepoUrl is required.")
	}
	if c.ChildBranch == "" {
		return skerr.Fmt("ChildBranch is required.")
	}
	if c.ChildPath == "" {
		return skerr.Fmt("ChildPath is required.")
	}
	if c.ParentRepoUrl == "" {
		return skerr.Fmt("ParentRepoUrl is required.")
	}
	if c.ParentBranch == "" {
		return skerr.Fmt("ParentBranch is required.")
	}
	for _, step := range c.PreUploadSteps {
		if err := step.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	if c.Metadata != nil {
		if err := c.Metadata.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// DefaultStrategy implements RepoManagerConfig.
func (c *AndroidRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *AndroidRepoManagerConfig) NoCheckout() bool {
	return false
}

// ValidStrategies implements RepoManagerConfig.
func (c *AndroidRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_N_BATCH,
	}
}

// Validate implements util.Validator.
func (c *AndroidRepoManagerConfig_ProjectMetadataFileConfig) Validate() error {
	if c.FilePath == "" {
		return skerr.Fmt("FilePath is required.")
	}
	if c.Name == "" {
		return skerr.Fmt("Name is required.")
	}
	if c.Description == "" {
		return skerr.Fmt("Description is required.")
	}
	if c.HomePage == "" {
		return skerr.Fmt("HomePage is required.")
	}
	if c.GitUrl == "" {
		return skerr.Fmt("GitUrl is required.")
	}
	if c.LicenseType == "" {
		return skerr.Fmt("LicenseType is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *CommandRepoManagerConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.GetTipRev == nil {
		return skerr.Fmt("GetTipRev is required.")
	}
	if err := c.GetTipRev.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.GetPinnedRev == nil {
		return skerr.Fmt("GetPinnedRev is required.")
	}
	if err := c.GetPinnedRev.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.SetPinnedRev == nil {
		return skerr.Fmt("SetPinnedRev is required.")
	}
	if err := c.SetPinnedRev.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// DefaultStrategy implements RepoManagerConfig.
func (c *CommandRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *CommandRepoManagerConfig) NoCheckout() bool {
	return false
}

// ValidStrategies implements RepoManagerConfig.
func (c *CommandRepoManagerConfig) ValidStrategies() []string {
	return []string{strategy.ROLL_STRATEGY_BATCH}
}

// Validate implements util.Validator.
func (c *CommandRepoManagerConfig_CommandConfig) Validate() error {
	if len(c.Command) == 0 {
		return skerr.Fmt("Command is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *FreeTypeRepoManagerConfig) Validate() error {
	if c.Child == nil {
		return skerr.Fmt("Child is required.")
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Parent == nil {
		return skerr.Fmt("Parent is required.")
	}
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// DefaultStrategy implements RepoManagerConfig.
func (c *FreeTypeRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *FreeTypeRepoManagerConfig) NoCheckout() bool {
	return false
}

// ValidStrategies implements RepoManagerConfig.
func (c *FreeTypeRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}

// Validate implements util.Validator.
func (c *ParentChildRepoManagerConfig) Validate() error {
	children := []util.Validator{}
	if c.GetCipdChild() != nil {
		children = append(children, c.GetCipdChild())
	}
	if c.GetFuchsiaSdkChild() != nil {
		children = append(children, c.GetFuchsiaSdkChild())
	}
	if c.GetGitCheckoutChild() != nil {
		children = append(children, c.GetGitCheckoutChild())
	}
	if c.GetGitCheckoutGithubChild() != nil {
		children = append(children, c.GetGitCheckoutGithubChild())
	}
	if c.GetGitilesChild() != nil {
		children = append(children, c.GetGitilesChild())
	}
	if c.GetSemverGcsChild() != nil {
		children = append(children, c.GetSemverGcsChild())
	}
	if c.GetDockerChild() != nil {
		children = append(children, c.GetDockerChild())
	}
	if len(children) != 1 {
		return skerr.Fmt("Exactly one Child is required, config has %d.", len(children))
	}
	if err := children[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	parents := []util.Validator{}
	if c.GetCopyParent() != nil {
		parents = append(parents, c.GetCopyParent())
	}
	if c.GetDepsLocalGithubParent() != nil {
		parents = append(parents, c.GetDepsLocalGithubParent())
	}
	if c.GetDepsLocalGerritParent() != nil {
		parents = append(parents, c.GetDepsLocalGerritParent())
	}
	if c.GetGitCheckoutGithubFileParent() != nil {
		parents = append(parents, c.GetGitCheckoutGithubFileParent())
	}
	if c.GetGitCheckoutGerritParent() != nil {
		parents = append(parents, c.GetGitCheckoutGerritParent())
	}
	if c.GetGitilesParent() != nil {
		parents = append(parents, c.GetGitilesParent())
	}
	if c.GetGoModGerritParent() != nil {
		parents = append(parents, c.GetGoModGerritParent())
	}
	if len(parents) != 1 {
		return skerr.Fmt("Exactly one Parent is required.")
	}
	if err := parents[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	for _, rf := range c.GetBuildbucketRevisionFilter() {
		if err := rf.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	for _, rf := range c.GetCipdRevisionFilter() {
		if err := rf.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// DefaultStrategy implements RepoManagerConfig.
func (c *ParentChildRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *ParentChildRepoManagerConfig) NoCheckout() bool {
	if c.GetGitCheckoutChild() != nil {
		return false
	}
	if c.GetGitCheckoutGithubChild() != nil {
		return false
	}
	if c.GetDepsLocalGithubParent() != nil {
		return false
	}
	if c.GetDepsLocalGerritParent() != nil {
		return false
	}
	if c.GetGitCheckoutGithubFileParent() != nil {
		return false
	}
	if c.GetGitCheckoutGerritParent() != nil {
		return false
	}
	return true
}

// ValidStrategies implements RepoManagerConfig.
func (c *ParentChildRepoManagerConfig) ValidStrategies() []string {
	if c.GetCipdChild() != nil {
		return []string{strategy.ROLL_STRATEGY_BATCH}
	}
	if c.GetFuchsiaSdkChild() != nil {
		return []string{strategy.ROLL_STRATEGY_BATCH}
	}
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_N_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}

// Validate implements util.Validator.
func (c *GitCheckoutGitHubChildConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.RepoOwner == "" {
		return skerr.Fmt("RepoOwner is required.")
	}
	if c.RepoName == "" {
		return skerr.Fmt("RepoName is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitCheckoutConfig) Validate() error {
	if c.Branch == "" {
		return skerr.Fmt("Branch is required.")
	}
	if c.RepoUrl == "" {
		return skerr.Fmt("RepoUrl is required.")
	}
	for _, dep := range c.Dependencies {
		if err := dep.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitilesChildConfig) Validate() error {
	if c.Gitiles == nil {
		return skerr.Fmt("Gitiles is required.")
	}
	if err := c.Gitiles.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitilesConfig) Validate() error {
	if c.Branch == "" {
		return skerr.Fmt("Branch is required.")
	}
	if c.RepoUrl == "" {
		return skerr.Fmt("RepoUrl is required.")
	}
	for _, dep := range c.Dependencies {
		if err := dep.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *FreeTypeParentConfig) Validate() error {
	if c.Gitiles == nil {
		return skerr.Fmt("Gitiles is required.")
	}
	if err := c.Gitiles.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitilesParentConfig) Validate() error {
	if c.Gitiles == nil {
		return skerr.Fmt("Gitiles is required.")
	}
	if err := c.Gitiles.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Dep == nil {
		return skerr.Fmt("Dep is required.")
	}
	if err := c.Dep.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Gerrit == nil {
		return skerr.Fmt("Gerrit is required.")
	}
	if err := c.Gerrit.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *GoModGerritParentConfig) Validate() error {
	if c.Gerrit == nil {
		return skerr.Fmt("Gerrit is required.")
	}
	if err := c.Gerrit.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.GoMod == nil {
		return skerr.Fmt("GoMod is required.")
	}
	if err := c.GoMod.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *GoModParentConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ModulePath == "" {
		return skerr.Fmt("ModulePath is required.")
	}
	for _, step := range c.PreUploadSteps {
		if err := step.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *DependencyConfig) Validate() error {
	if c.Primary == nil {
		return skerr.Fmt("Primary is required.")
	}
	if err := c.Primary.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	for _, dep := range c.Transitive {
		if err := dep.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *FuchsiaSDKChildConfig) Validate() error {
	if c.GcsBucket == "" {
		return skerr.Fmt("GcsBucket is required.")
	}
	if c.LatestLinuxPath == "" {
		return skerr.Fmt("LatestLinuxPath is required.")
	}
	if c.IncludeMacSdk && c.LatestMacPath == "" {
		return skerr.Fmt("IncludeMacSdk is deprecated; use LatestMacPath instead.")
	}
	if c.TarballLinuxPathTmpl == "" {
		return skerr.Fmt("TarballLinuxPathTmpl is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *CIPDChildConfig) Validate() error {
	if c.Name == "" {
		return skerr.Fmt("Name is required.")
	}
	if c.Tag == "" {
		return skerr.Fmt("Tag is required.")
	}
	if c.SourceRepo != nil {
		if err := c.SourceRepo.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitCheckoutChildConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *SemVerGCSChildConfig) Validate() error {
	if c.Gcs == nil {
		return skerr.Fmt("Gcs is required.")
	}
	if err := c.Gcs.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.VersionRegex == "" {
		return skerr.Fmt("VersionRegex is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *GCSChildConfig) Validate() error {
	if c.GcsBucket == "" {
		return skerr.Fmt("GcsBucket is required.")
	}
	if c.GcsPath == "" {
		return skerr.Fmt("GcsPath is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *CopyParentConfig) Validate() error {
	if c.Gitiles == nil {
		return skerr.Fmt("Gitiles is required.")
	}
	if err := c.Gitiles.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if len(c.Copies) == 0 {
		return skerr.Fmt("Copies are required.")
	}
	for _, cpy := range c.Copies {
		if err := cpy.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *CopyParentConfig_CopyEntry) Validate() error {
	if c.SrcRelPath == "" {
		return skerr.Fmt("SrcRelPath is required.")
	}
	if c.DstRelPath == "" {
		return skerr.Fmt("DstRelPath is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *DEPSLocalGitHubParentConfig) Validate() error {
	if c.DepsLocal == nil {
		return skerr.Fmt("DepsLocal is required.")
	}
	if err := c.DepsLocal.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Github == nil {
		return skerr.Fmt("Github is required.")
	}
	if err := c.Github.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ForkRepoUrl == "" {
		return skerr.Fmt("ForkRepoUrl is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *DEPSLocalParentConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	for _, step := range c.PreUploadSteps {
		if err := step.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitCheckoutParentConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Dep == nil {
		return skerr.Fmt("Dep is required.")
	}
	if err := c.Dep.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *DEPSLocalGerritParentConfig) Validate() error {
	if c.DepsLocal == nil {
		return skerr.Fmt("DepsLocal is required.")
	}
	if err := c.DepsLocal.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Gerrit == nil {
		return skerr.Fmt("Gerrit is required.")
	}
	if err := c.Gerrit.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitCheckoutGitHubFileParentConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	for _, step := range c.PreUploadSteps {
		if err := step.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitCheckoutGitHubParentConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ForkRepoUrl == "" {
		return skerr.Fmt("ForkRepoUrl is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *GitCheckoutGerritParentConfig) Validate() error {
	if c.GitCheckout == nil {
		return skerr.Fmt("GitCheckout is required.")
	}
	if err := c.GitCheckout.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Validate implements util.Validator.
func (c *BuildbucketRevisionFilterConfig) Validate() error {
	if c.Project == "" {
		return skerr.Fmt("Project is required.")
	}
	if c.Bucket == "" {
		return skerr.Fmt("Bucket is required.")
	}
	if c.BuildsetCommitTmpl == "" {
		return skerr.Fmt("BuildsetCommitTmpl is required.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *CIPDRevisionFilterConfig) Validate() error {
	if len(c.Package) == 0 {
		return skerr.Fmt("At least one Package is required.")
	}
	if len(c.Platform) == 0 {
		return skerr.Fmt("At least one Platform is required.")
	}
	if strings.Contains(c.TagKey, ":") {
		return skerr.Fmt("TagKey cannot contain ':'.")
	}
	return nil
}

// Validate implements util.Validator.
func (c *DockerChildConfig) Validate() error {
	if c.Registry == "" {
		return skerr.Fmt("Registry is required.")
	}
	if c.Repository == "" {
		return skerr.Fmt("Repository is required.")
	}
	if c.Tag == "" {
		return skerr.Fmt("Tag is required.")
	}
	return nil
}

// Copy returns a deep copy.
func (c *TransitiveDepConfig) Copy() *TransitiveDepConfig {
	return &TransitiveDepConfig{
		Child:      c.Child.Copy(),
		Parent:     c.Parent.Copy(),
		LogUrlTmpl: c.LogUrlTmpl,
	}
}

// Copy returns a deep copy.
func (c *VersionFileConfig) Copy() *VersionFileConfig {
	var files []*VersionFileConfig_File
	if len(c.File) > 0 {
		files = make([]*VersionFileConfig_File, 0, len(c.File))
	}
	for _, file := range c.File {
		files = append(files, &VersionFileConfig_File{
			Path:  file.Path,
			Regex: file.Regex,
		})
	}
	return &VersionFileConfig{
		Id:   c.Id,
		File: files,
	}
}

type transitiveDepConfigSlice []*TransitiveDepConfig

// Copy returns a deep copy.
func (s transitiveDepConfigSlice) Copy() []*TransitiveDepConfig {
	rv := make([]*TransitiveDepConfig, 0, len(s))
	for _, c := range s {
		rv = append(rv, c.Copy())
	}
	return rv
}
