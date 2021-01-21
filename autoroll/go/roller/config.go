package roller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/config_vars"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/time_window"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// DEFAULT_SAFETY_THROTTLE_ATTEMPT_COUNT is the default attempt count for
	// safety throttling.
	DEFAULT_SAFETY_THROTTLE_ATTEMPT_COUNT = 3
	// DEFAULT_SAFETY_THROTTLE_TIME_WINDOW is the default time window for safety
	// throttling.
	DEFAULT_SAFETY_THROTTLE_TIME_WINDOW = 30 * time.Minute

	// MAX_ROLLER_NAME_LENGTH is the maximum roller name length. This is limited
	// by Kubernetes, which has a 63-character limit for various names. This
	// length is derived from that limit, accounting for the prefixes and
	// suffixes which are automatically added by our tooling, eg. the
	// "autoroll-be-" prefix and "-storage" suffix for disks, controller hashes,
	// etc.
	MAX_ROLLER_NAME_LENGTH = 41
)

var (
	// SAFETY_THROTTLE_CONFIG_DEFAULT is the default configuration for safety
	// throttling.
	SAFETY_THROTTLE_CONFIG_DEFAULT = &ThrottleConfig{
		AttemptCount: DEFAULT_SAFETY_THROTTLE_ATTEMPT_COUNT,
		TimeWindow:   DEFAULT_SAFETY_THROTTLE_TIME_WINDOW,
	}

	validK8sLabel = regexp.MustCompile(`^[a-zA-Z\._-]{1,63}$`)
)

// ThrottleConfig determines the throttling behavior for the roller.
type ThrottleConfig struct {
	AttemptCount int64
	TimeWindow   time.Duration
}

// Intermediate struct used for JSON [un]marshaling.
type throttleConfigJSON struct {
	AttemptCount int64  `json:"attemptCount"`
	TimeWindow   string `json:"timeWindow"`
}

// MarshalJSON implements json.Marshaler.
func (c *ThrottleConfig) MarshalJSON() ([]byte, error) {
	tc := throttleConfigJSON{
		AttemptCount: c.AttemptCount,
		TimeWindow:   strings.TrimSpace(human.Duration(c.TimeWindow)),
	}
	return json.Marshal(&tc)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ThrottleConfig) UnmarshalJSON(b []byte) error {
	var tc throttleConfigJSON
	if err := json5.Unmarshal(b, &tc); err != nil {
		return err
	}
	dur, err := human.ParseDuration(tc.TimeWindow)
	if err != nil {
		return err
	}
	c.AttemptCount = tc.AttemptCount
	c.TimeWindow = dur
	return nil
}

// Google3FakeRepoManagerConfig is a fake repo manager config used to indicate a
// Google3 roller.
type Google3FakeRepoManagerConfig struct {
	// Branch of the child repo to roll.
	ChildBranch string `json:"childBranch"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"`
}

// DefaultStrategy implements RepoManagerConfig.
func (c *Google3FakeRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// NoCheckout implements RepoManagerConfig.
func (c *Google3FakeRepoManagerConfig) NoCheckout() bool {
	return false
}

// ValidStrategies implements RepoManagerConfig.
func (c *Google3FakeRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// Validate implements util.Validator.
func (c *Google3FakeRepoManagerConfig) Validate() error {
	if c.ChildBranch == "" {
		return errors.New("ChildBranch is required.")
	}
	if c.ChildRepo == "" {
		return errors.New("ChildRepo is required.")
	}
	return nil
}

// KubernetesConfig contains configuration information for an AutoRoller running
// in Kubernetes.
type KubernetesConfig struct {
	// Requested number of CPUs.
	CPU string `json:"cpu"`
	// Requested memory, eg. "2Gi"
	Memory string `json:"memory"`
	// How many times the ready check may fail.
	ReadinessFailureThreshold string `json:"readinessFailureThreshold"`
	// Delay before starting to check whether the pod is ready.
	ReadinessInitialDelaySeconds string `json:"readinessInitialDelaySeconds"`
	// How often to perform the ready check after ReadinessInitialDelaySeconds.
	ReadinessPeriodSeconds string `json:"readinessPeriodSeconds"`
	// Requested persistent disk size, eg. "200Gi". If empty, no persistent
	// disk is used.
	Disk string `json:"disk,omitempty"`
	// Secrets provided to the pod.
	Secrets []*KubernetesSecret `json:"secrets,omitempty"`
}

// KubernetesSecret describes a secret provided to a Kubernetes pod.
type KubernetesSecret struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

// Validate implements util.Validator.
func (c *KubernetesConfig) Validate() error {
	if c.CPU == "" {
		return errors.New("CPU is required.")
	}
	if c.Memory == "" {
		return errors.New("Memory is required.")
	}
	return nil
}

// AutoRollerConfig contains configuration information for an AutoRoller.
type AutoRollerConfig struct {
	// Required Fields.

	// Display name of the child.
	ChildDisplayName string `json:"childDisplayName"`
	// List of email addresses of contacts for this roller, used for sending
	// PSAs, asking questions, etc.
	Contacts []string `json:"contacts"`
	// If true, the roller is only visible to Googlers.
	IsInternal bool `json:"isInternal"`
	// Primary owner of this roller.
	OwnerPrimary string `json:"ownerPrimary"`
	// Secondary owner of this roller.
	OwnerSecondary string `json:"ownerSecondary"`
	// Display name of the parent.
	ParentDisplayName string `json:"parentDisplayName"`
	// URL of the waterfall/status display for the parent.
	ParentWaterfall string `json:"parentWaterfall"`
	// Name of the roller, used for database keys.
	RollerName string `json:"rollerName"`
	// Full email address of the service account for this roller.
	ServiceAccount string `json:"serviceAccount"`
	// Email addresses to CC on rolls, or URL from which to obtain those
	// email addresses.
	Sheriff []string `json:"sheriff"`
	// Backup email addresses to CC on rolls, in case obtaining the email
	// addresses from the URL fails.  Only required if a URL is specified
	// for Sheriff.
	SheriffBackup []string `json:"sheriffBackup,omitempty"`

	// Commit message configuration.
	CommitMsgConfig *commit_msg.CommitMsgConfig `json:"commitMsg"`

	// Code review settings.
	Gerrit        *codereview.GerritConfig  `json:"gerrit,omitempty"`
	Github        *codereview.GithubConfig  `json:"github,omitempty"`
	Google3Review *codereview.Google3Config `json:"google3Review,omitempty"`

	// RepoManager configs. Exactly one must be provided.
	AndroidRepoManager           *repo_manager.AndroidRepoManagerConfig           `json:"androidRepoManager,omitempty"`
	CommandRepoManager           *repo_manager.CommandRepoManagerConfig           `json:"commandRepoManager,omitempty"`
	CopyRepoManager              *repo_manager.CopyRepoManagerConfig              `json:"copyRepoManager,omitempty"`
	DEPSGitilesRepoManager       *repo_manager.DEPSGitilesRepoManagerConfig       `json:"depsGitilesRepoManager,omitempty"`
	DEPSRepoManager              *repo_manager.DEPSRepoManagerConfig              `json:"depsRepoManager,omitempty"`
	FreeTypeRepoManager          *repo_manager.FreeTypeRepoManagerConfig          `json:"freeTypeRepoManager,omitempty"`
	FuchsiaSDKAndroidRepoManager *repo_manager.FuchsiaSDKAndroidRepoManagerConfig `json:"fuchsiaSDKAndroidRepoManager,omitempty"`
	FuchsiaSDKRepoManager        *repo_manager.FuchsiaSDKRepoManagerConfig        `json:"fuchsiaSDKRepoManager,omitempty"`
	GithubRepoManager            *repo_manager.GithubRepoManagerConfig            `json:"githubRepoManager,omitempty"`
	GithubCipdDEPSRepoManager    *repo_manager.GithubCipdDEPSRepoManagerConfig    `json:"githubCipdDEPSRepoManager,omitempty"`
	GithubDEPSRepoManager        *repo_manager.GithubDEPSRepoManagerConfig        `json:"githubDEPSRepoManager,omitempty"`
	GitilesCIPDDEPSRepoManager   *repo_manager.GitilesCIPDDEPSRepoManagerConfig   `json:"gitilesCIPDDEPSRepoManager,omitempty"`
	Google3RepoManager           *Google3FakeRepoManagerConfig                    `json:"google3,omitempty"`
	NoCheckoutDEPSRepoManager    *repo_manager.NoCheckoutDEPSRepoManagerConfig    `json:"noCheckoutDEPSRepoManager,omitempty"`
	SemVerGCSRepoManager         *repo_manager.SemVerGCSRepoManagerConfig         `json:"semVerGCSRepoManager,omitempty"`

	// Kubernetes config.
	Kubernetes *KubernetesConfig `json:"kubernetes"`

	// Optional Fields.

	// Limit to one successful roll within this time period.
	MaxRollFrequency string `json:"maxRollFrequency,omitempty"`
	// Any extra notification systems to be used for this roller.
	Notifiers []*notifier.Config `json:"notifiers,omitempty"`
	// Throttling configuration to prevent uploading too many CLs within
	// too short a time period.
	SafetyThrottle *ThrottleConfig `json:"safetyThrottle,omitempty"`
	// If true, this roller supports one-click "manual" rolls.
	SupportsManualRolls bool `json:"supportsManualRolls,omitempty"`
	// Time window in which the roller is allowed to upload roll CLs. See
	// the go/time_window package for supported format.
	TimeWindow string `json:"timeWindow,omitempty"`
	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the paths within the parent and child repo, respectively, where
	// those dependencies are versioned, eg. "DEPS".
	TransitiveDeps []*version_file_common.TransitiveDepConfig `json:"transitiveDeps,omitempty"`
}

// Validate the config.
func (c *AutoRollerConfig) Validate() error {
	if c.ChildDisplayName == "" {
		return errors.New("ChildDisplayName is required.")
	}
	if len(c.Contacts) < 1 {
		return errors.New("At least one contact is required.")
	}
	if c.ParentDisplayName == "" {
		return errors.New("ParentDisplayName is required.")
	}
	if c.ParentWaterfall == "" {
		return errors.New("ParentWaterfall is required.")
	}
	if !validK8sLabel.MatchString(c.OwnerPrimary) {
		return errors.New("OwnerPrimary is invalid.")
	}
	if !validK8sLabel.MatchString(c.OwnerSecondary) {
		return errors.New("OwnerSecondary is invalid.")
	}
	if c.RollerName == "" {
		return errors.New("RollerName is required.")
	}
	if len(c.RollerName) > MAX_ROLLER_NAME_LENGTH {
		return fmt.Errorf("RollerName length %d is greater than maximum %d", len(c.RollerName), MAX_ROLLER_NAME_LENGTH)
	}
	if c.ServiceAccount == "" {
		return errors.New("ServiceAccount is required.")
	}
	if c.Sheriff == nil || len(c.Sheriff) == 0 {
		return errors.New("Sheriff is required.")
	}
	if c.MaxRollFrequency != "" {
		maxRollFreq, err := human.ParseDuration(c.MaxRollFrequency)
		if err != nil {
			return skerr.Wrapf(err, "Failed to parse maxRollFrequency")
		}
		if maxRollFreq == 0 {
			c.MaxRollFrequency = ""
		}
	}

	if c.CommitMsgConfig == nil {
		return skerr.Fmt("CommitMsgConfig is required")
	}
	if err := c.CommitMsgConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}

	cr := []util.Validator{}
	if c.Gerrit != nil {
		cr = append(cr, c.Gerrit)
	}
	if c.Github != nil {
		cr = append(cr, c.Github)
	}
	if c.Google3Review != nil {
		cr = append(cr, c.Google3Review)
	}
	if len(cr) != 1 {
		return errors.New("Exactly one of Gerrit, Github, or Google3Review is required.")
	}
	if err := cr[0].Validate(); err != nil {
		return err
	}

	rm, err := c.repoManagerConfig()
	if err != nil {
		return err
	}
	if err := rm.Validate(); err != nil {
		return err
	}

	if c.Kubernetes == nil {
		return errors.New("Kubernetes config is required.")
	}
	if err := c.Kubernetes.Validate(); err != nil {
		return fmt.Errorf("KubernetesConfig validation failed: %s", err)
	}
	isNoCheckout := rm.NoCheckout()
	if isNoCheckout && c.Kubernetes.Disk != "" {
		return errors.New("kubernetes.disk is not valid for no-checkout repo managers.")
	} else if !isNoCheckout && c.Kubernetes.Disk == "" {
		return errors.New("kubernetes.disk is required for repo managers which use a checkout.")
	}

	// Verify that the notifier configs are valid.
	if _, err := arb_notifier.New(context.Background(), "fake", "fake", "fake", nil, nil, nil, c.Notifiers); err != nil {
		return err
	}

	// Verify that the TimeWindow is valid.
	_, err = time_window.Parse(c.TimeWindow)
	return err
}

// CodeReview returns the code review config for the roller.
func (c *AutoRollerConfig) CodeReview() codereview.CodeReviewConfig {
	if c.Github != nil {
		return c.Github
	}
	if c.Google3Review != nil {
		return c.Google3Review
	}
	return c.Gerrit
}

// Return the RepoManagerConfig for the roller.
func (c *AutoRollerConfig) repoManagerConfig() (RepoManagerConfig, error) {
	rm := []RepoManagerConfig{}
	if c.AndroidRepoManager != nil {
		rm = append(rm, c.AndroidRepoManager)
	}
	if c.CommandRepoManager != nil {
		rm = append(rm, c.CommandRepoManager)
	}
	if c.CopyRepoManager != nil {
		rm = append(rm, c.CopyRepoManager)
	}
	if c.DEPSGitilesRepoManager != nil {
		rm = append(rm, c.DEPSGitilesRepoManager)
	}
	if c.DEPSRepoManager != nil {
		rm = append(rm, c.DEPSRepoManager)
	}
	if c.FreeTypeRepoManager != nil {
		rm = append(rm, c.FreeTypeRepoManager)
	}
	if c.FuchsiaSDKAndroidRepoManager != nil {
		rm = append(rm, c.FuchsiaSDKAndroidRepoManager)
	}
	if c.FuchsiaSDKRepoManager != nil {
		rm = append(rm, c.FuchsiaSDKRepoManager)
	}
	if c.GithubRepoManager != nil {
		rm = append(rm, c.GithubRepoManager)
	}
	if c.GithubCipdDEPSRepoManager != nil {
		rm = append(rm, c.GithubCipdDEPSRepoManager)
	}
	if c.GithubDEPSRepoManager != nil {
		rm = append(rm, c.GithubDEPSRepoManager)
	}
	if c.GitilesCIPDDEPSRepoManager != nil {
		rm = append(rm, c.GitilesCIPDDEPSRepoManager)
	}
	if c.Google3RepoManager != nil {
		rm = append(rm, c.Google3RepoManager)
	}
	if c.NoCheckoutDEPSRepoManager != nil {
		rm = append(rm, c.NoCheckoutDEPSRepoManager)
	}
	if c.SemVerGCSRepoManager != nil {
		rm = append(rm, c.SemVerGCSRepoManager)
	}
	if len(rm) == 1 {
		return rm[0], nil
	}
	return nil, skerr.Fmt("Exactly one repo manager is expected but got %d", len(rm))
}

// DefaultStrategy returns the default strategy for the roller.
func (c *AutoRollerConfig) DefaultStrategy() string {
	rm, err := c.repoManagerConfig()
	if err != nil {
		sklog.Fatalf("Failed to obtain RepoManagerConfig; this should have been caught during validation! %s", err)
	}
	return rm.DefaultStrategy()
}

// ValidStrategies returns the valid strategies for the roller.
func (c *AutoRollerConfig) ValidStrategies() []string {
	rm, err := c.repoManagerConfig()
	if err != nil {
		sklog.Fatalf("Failed to obtain RepoManagerConfig; this should have been caught during validation! %s", err)
	}
	return rm.ValidStrategies()
}

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

// CreateRepoManager creates a RepoManager instance from the config.
// TODO(borenet): If we can't remove this after refactoring RepoManager configs,
// this should probably move into the repo_manager package.
func (c *AutoRollerConfig) CreateRepoManager(ctx context.Context, cr codereview.CodeReview, reg *config_vars.Registry, g *gerrit.Gerrit, githubClient *github.GitHub, workdir, recipesCfgFile, serverURL, rollerName string, gcsClient gcs.GCSClient, client *http.Client, local bool) (repo_manager.RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	var rm repo_manager.RepoManager
	var err error
	if c.AndroidRepoManager != nil {
		rm, err = repo_manager.NewAndroidRepoManager(ctx, c.AndroidRepoManager, reg, workdir, g, serverURL, c.ServiceAccount, client, cr, c.IsInternal, local)
	} else if c.CommandRepoManager != nil {
		rm, err = repo_manager.NewCommandRepoManager(ctx, *c.CommandRepoManager, reg, workdir, g, serverURL, cr)
	} else if c.CopyRepoManager != nil {
		rm, err = repo_manager.NewCopyRepoManager(ctx, c.CopyRepoManager, reg, workdir, g, serverURL, client, cr, local)
	} else if c.DEPSGitilesRepoManager != nil {
		rm, err = repo_manager.NewDEPSGitilesRepoManager(ctx, c.DEPSGitilesRepoManager, reg, workdir, g, recipesCfgFile, serverURL, client, cr)
	} else if c.DEPSRepoManager != nil {
		rm, err = repo_manager.NewDEPSRepoManager(ctx, c.DEPSRepoManager, reg, workdir, g, recipesCfgFile, serverURL, client, cr, local)
	} else if c.FuchsiaSDKAndroidRepoManager != nil {
		rm, err = repo_manager.NewFuchsiaSDKAndroidRepoManager(ctx, c.FuchsiaSDKAndroidRepoManager, reg, workdir, g, serverURL, client, cr, local)
	} else if c.FreeTypeRepoManager != nil {
		rm, err = repo_manager.NewFreeTypeRepoManager(ctx, c.FreeTypeRepoManager, reg, workdir, g, recipesCfgFile, serverURL, client, cr, local)
	} else if c.FuchsiaSDKRepoManager != nil {
		rm, err = repo_manager.NewFuchsiaSDKRepoManager(ctx, c.FuchsiaSDKRepoManager, reg, workdir, g, serverURL, client, cr, local)
	} else if c.GithubRepoManager != nil {
		rm, err = repo_manager.NewGithubRepoManager(ctx, c.GithubRepoManager, reg, workdir, rollerName, githubClient, recipesCfgFile, serverURL, client, cr, local)
	} else if c.GithubCipdDEPSRepoManager != nil {
		rm, err = repo_manager.NewGithubCipdDEPSRepoManager(ctx, c.GithubCipdDEPSRepoManager, reg, workdir, rollerName, githubClient, recipesCfgFile, serverURL, client, cr, local)
	} else if c.GithubDEPSRepoManager != nil {
		rm, err = repo_manager.NewGithubDEPSRepoManager(ctx, c.GithubDEPSRepoManager, reg, workdir, rollerName, githubClient, recipesCfgFile, serverURL, client, cr, local)
	} else if c.GitilesCIPDDEPSRepoManager != nil {
		rm, err = repo_manager.NewGitilesCIPDDEPSRepoManager(ctx, c.GitilesCIPDDEPSRepoManager, reg, workdir, g, recipesCfgFile, serverURL, client, cr, local)
	} else if c.NoCheckoutDEPSRepoManager != nil {
		rm, err = repo_manager.NewNoCheckoutDEPSRepoManager(ctx, c.NoCheckoutDEPSRepoManager, reg, workdir, g, recipesCfgFile, serverURL, client, cr, local)
	} else if c.SemVerGCSRepoManager != nil {
		rm, err = repo_manager.NewSemVerGCSRepoManager(ctx, c.SemVerGCSRepoManager, reg, workdir, g, serverURL, client, cr, local)
	} else {
		return nil, skerr.Fmt("Invalid roller config; no repo manager defined!")
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create RepoManager")
	}
	return rm, nil
}
