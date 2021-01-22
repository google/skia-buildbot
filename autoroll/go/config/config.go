package config

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. ./config.proto
//--go:generate mv ./go.skia.org/infra/autoroll/go/config/config.twirp.go ./config.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w config.pb.go
//--go:generate goimports -w config.twirp.go
//--go:generate protoc --twirp_typescript_out=../../modules/config ./config.proto

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
)

var (
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
	if c.TimeWindow.AsDuration() == 0 {
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
