package config

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --go_out=. ./config.proto
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w config.pb.go
//go:generate protoc --twirp_typescript_out=../../modules/config ./config.proto

import (
	"fmt"
	"regexp"

	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/time_window"
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
	ValidK8sLabel = regexp.MustCompile(`^[a-zA-Z\._-]{1,63}$`)
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
		return fmt.Errorf("RollerName length %d is greater than maximum %d", len(c.RollerName), MaxRollerNameLength)
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
		return fmt.Errorf("Kubernetes validation failed: %s", err)
	}

	rm := []RepoManagerConfig{}
	if c.GetParentChildRepoManager() != nil {
		rm = append(rm, c.GetParentChildRepoManager())
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
	if c.GetFuchsiaSdkAndroidRepoManager() != nil {
		rm = append(rm, c.GetFuchsiaSdkAndroidRepoManager())
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

	for _, td := range c.TransitiveDeps {
		if err := td.Validate(); err != nil {
			return skerr.Wrapf(err, "transitive dep config failed validation")
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
	if c.GetFuchsiaSdkAndroidRepoManager() != nil {
		return c.GetFuchsiaSdkAndroidRepoManager()
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

// Validate implements util.Validator.
func (c *CommitMsgConfig) Validate() error {
	if c.Template == nil {
		return skerr.Fmt("Template is required.")
	}
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
	return c.Config != GerritConfig_ANDROID && c.Config != GerritConfig_ANDROID_NO_CR
}

// Validate implements util.Validator.
func (c *GitHubConfig) Validate() error {
	if c.RepoOwner == "" {
		return skerr.Fmt("RepoOwner is required.")
	}
	if c.RepoName == "" {
		return skerr.Fmt("RepoName is required.")
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
	for _, secret := range c.Secrets {
		if err := secret.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Validate implements util.Validator.
func (c *KubernetesSecret) Validate() error {
	if c.Name == "" {
		return skerr.Fmt("Secret name is required.")
	}
	if c.MountPath == "" {
		return skerr.Fmt("Secret MountPath is required.")
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
	if c.Path == "" {
		return skerr.Fmt("Path is required.")
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
func (c *FuchsiaSDKAndroidRepoManagerConfig) Validate() error {
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
	if c.GenSdkBpRepo == "" {
		return skerr.Fmt("GenSdkBpRepo is required.")
	}
	if c.GenSdkBpBranch == "" {
		return skerr.Fmt("GenSdkBpBranch is required.")
	}
	return nil
}

// DefaultStrategy implements RepoManagerConfig.
func (c *FuchsiaSDKAndroidRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *FuchsiaSDKAndroidRepoManagerConfig) NoCheckout() bool {
	return false
}

// ValidStrategies implements RepoManagerConfig.
func (c *FuchsiaSDKAndroidRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
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
	if len(children) != 1 {
		return skerr.Fmt("Exactly one Child is required.")
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
	if c.GetGitilesParent() != nil {
		parents = append(parents, c.GetGitilesParent())
	}
	if len(parents) != 1 {
		return skerr.Fmt("Exactly one Parent is required.")
	}
	if err := parents[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	if c.GetBuildbucketRevisionFilter() != nil {
		if err := c.GetBuildbucketRevisionFilter().Validate(); err != nil {
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
func (c *BuildbucketRevisionFilterConfig) Validate() error {
	if c.Project == "" {
		return skerr.Fmt("Project is required.")
	}
	if c.Bucket == "" {
		return skerr.Fmt("Bucket is required.")
	}
	return nil
}
