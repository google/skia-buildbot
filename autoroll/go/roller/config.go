package roller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flynn/json5"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/util"
)

const (
	// Throttling parameters.
	DEFAULT_SAFETY_THROTTLE_ATTEMPT_COUNT = 3
	DEFAULT_SAFETY_THROTTLE_TIME_WINDOW   = 30 * time.Minute

	ROLLER_TYPE_ASSET            = "asset"
	ROLLER_TYPE_AFDO             = "afdo"
	ROLLER_TYPE_ANDROID          = "android"
	ROLLER_TYPE_COPY             = "copy"
	ROLLER_TYPE_DEPS             = "deps"
	ROLLER_TYPE_DEPS_NO_CHECKOUT = "noCheckoutDEPS"
	ROLLER_TYPE_FUCHSIA_SDK      = "fuchsiaSDK"
	ROLLER_TYPE_GITHUB           = "github"
	ROLLER_TYPE_GITHUB_DEPS      = "githubDEPS"
	ROLLER_TYPE_GOOGLE3          = "google3"
	ROLLER_TYPE_INVALID          = "INVALID"
	ROLLER_TYPE_MANIFEST         = "manifest"
)

var (
	SAFETY_THROTTLE_CONFIG_DEFAULT = &ThrottleConfig{
		AttemptCount: DEFAULT_SAFETY_THROTTLE_ATTEMPT_COUNT,
		TimeWindow:   DEFAULT_SAFETY_THROTTLE_TIME_WINDOW,
	}
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

// See documentation for json.Marshaler interface.
func (c *ThrottleConfig) MarshalJSON() ([]byte, error) {
	tc := throttleConfigJSON{
		AttemptCount: c.AttemptCount,
		TimeWindow:   strings.TrimSpace(human.Duration(c.TimeWindow)),
	}
	return json.Marshal(&tc)
}

// See documentation for json.Unmarshaler interface.
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

// Dummy repo manager config used to indicate a Google3 roller.
type Google3FakeRepoManagerConfig struct {
	// Branch of the child repo to roll.
	ChildBranch string `json:"childBranch"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"`
}

// See documentation for util.Validator interface.
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
	// Requested disk size, eg. "200Gi"
	Disk string `json:"disk"`
}

// Validate the KubernetesConfig.
func (c *KubernetesConfig) Validate() error {
	// KubernetesConfig is optional for now.
	if c == nil {
		return nil
	}
	if c.CPU == "" {
		return errors.New("CPU is required.")
	}
	if c.Memory == "" {
		return errors.New("Memory is required.")
	}
	if c.Disk == "" {
		return errors.New("Disk is required.")
	}
	return nil
}

// AutoRollerConfig contains configuration information for an AutoRoller.
type AutoRollerConfig struct {
	// Required Fields.

	// User friendly name of the child repo.
	ChildName string `json:"childName"`
	// List of email addresses of contacts for this roller, used for sending
	// PSAs, asking questions, etc.
	Contacts []string `json:"contacts"`
	// Gerrit URL the roller will be uploading issues to.
	GerritURL string `json:"gerritURL,omitempty"`
	// If true, the roller is only visible to Googlers.
	IsInternal bool `json:"isInternal"`
	// User friendly name of the parent repo.
	ParentName string `json:"parentName"`
	// URL of the waterfall/status display for the parent repo.
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

	// Github code review flags.
	GithubRepoOwner      string   `json:"githubRepoOwner,omitempty"`
	GithubRepoName       string   `json:"githubRepoName,omitempty"`
	GithubChecksNum      int      `json:"githubChecksNum,omitempty"`
	GithubChecksWaitFor  []string `json:"githubChecksWaitFor,omitempty"`
	GithubMergeMethodURL string   `json:"githubMergeMethodURL,omitempty"`

	// RepoManager configs. Exactly one must be provided.
	AFDORepoManager           *repo_manager.AFDORepoManagerConfig           `json:"afdoRepoManager,omitempty"`
	AndroidRepoManager        *repo_manager.AndroidRepoManagerConfig        `json:"androidRepoManager,omitempty"`
	AssetRepoManager          *repo_manager.AssetRepoManagerConfig          `json:"assetRepoManager,omitempty"`
	CopyRepoManager           *repo_manager.CopyRepoManagerConfig           `json:"copyRepoManager,omitempty"`
	DEPSRepoManager           *repo_manager.DEPSRepoManagerConfig           `json:"depsRepoManager,omitempty"`
	FuchsiaSDKRepoManager     *repo_manager.FuchsiaSDKRepoManagerConfig     `json:"fuchsiaSDKRepoManager,omitempty"`
	GithubRepoManager         *repo_manager.GithubRepoManagerConfig         `json:"githubRepoManager,omitempty"`
	GithubDEPSRepoManager     *repo_manager.GithubDEPSRepoManagerConfig     `json:"githubDEPSRepoManager,omitempty"`
	Google3RepoManager        *Google3FakeRepoManagerConfig                 `json:"google3,omitempty"`
	ManifestRepoManager       *repo_manager.ManifestRepoManagerConfig       `json:"manifestRepoManager,omitempty"`
	NoCheckoutDEPSRepoManager *repo_manager.NoCheckoutDEPSRepoManagerConfig `json:"noCheckoutDEPSRepoManager,omitempty"`

	// Kubernetes config.
	// TODO(borenet): Optional right now, but will eventually be required.
	Kubernetes *KubernetesConfig `json:"kubernetes"`

	// Optional Fields.

	// Comma-separated list of trybots to add to roll CLs, in addition to
	// the default set of commit queue trybots.
	CqExtraTrybots []string `json:"cqExtraTrybots,omitempty"`
	// Limit to one successful roll within this time period.
	MaxRollFrequency string `json:"maxRollFrequency,omitempty"`
	// Any extra notification systems to be used for this roller.
	Notifiers []*notifier.Config `json:"notifiers,omitempty"`
	// Throttling configuration to prevent uploading too many CLs within
	// too short a time period.
	SafetyThrottle *ThrottleConfig `json:"safetyThrottle,omitempty"`

	// Private.
	rollerType string // Set by RollerType().
}

// Validate the config.
func (c *AutoRollerConfig) Validate() error {
	if c.ChildName == "" {
		return errors.New("ChildName is required.")
	}
	if len(c.Contacts) < 1 {
		return errors.New("At least one contact is required.")
	}
	if c.GerritURL == "" && (c.GithubRepoOwner == "" || c.GithubRepoName == "") {
		return errors.New("Either GerritURL OR both GithubRepoOwner/GithubRepoName is required.")
	}
	if c.ParentName == "" {
		return errors.New("ParentName is required.")
	}
	if c.ParentWaterfall == "" {
		return errors.New("ParentWaterfall is required.")
	}
	if c.RollerName == "" {
		return errors.New("RollerName is required.")
	}
	// TODO(borenet): Make ServiceAccount required for all rollers once
	// they're moved to k8s.
	if c.ServiceAccount == "" && c.Kubernetes != nil {
		return errors.New("ServiceAccount is required.")
	}
	if c.Sheriff == nil || len(c.Sheriff) == 0 {
		return errors.New("Sheriff is required.")
	}

	rm := []util.Validator{}
	if c.AFDORepoManager != nil {
		rm = append(rm, c.AFDORepoManager)
	}
	if c.AndroidRepoManager != nil {
		rm = append(rm, c.AndroidRepoManager)
	}
	if c.AssetRepoManager != nil {
		rm = append(rm, c.AssetRepoManager)
	}
	if c.CopyRepoManager != nil {
		rm = append(rm, c.CopyRepoManager)
	}
	if c.DEPSRepoManager != nil {
		rm = append(rm, c.DEPSRepoManager)
	}
	if c.FuchsiaSDKRepoManager != nil {
		rm = append(rm, c.FuchsiaSDKRepoManager)
	}
	if c.GithubRepoManager != nil {
		rm = append(rm, c.GithubRepoManager)
	}
	if c.GithubDEPSRepoManager != nil {
		rm = append(rm, c.GithubDEPSRepoManager)
	}
	if c.Google3RepoManager != nil {
		rm = append(rm, c.Google3RepoManager)
	}
	if c.ManifestRepoManager != nil {
		rm = append(rm, c.ManifestRepoManager)
	}
	if c.NoCheckoutDEPSRepoManager != nil {
		rm = append(rm, c.NoCheckoutDEPSRepoManager)
	}
	if len(rm) != 1 {
		return fmt.Errorf("Exactly one repo manager must be supplied, but got %d", len(rm))
	}
	if err := rm[0].Validate(); err != nil {
		return err
	}

	if err := c.Kubernetes.Validate(); err != nil {
		return fmt.Errorf("KubernetesConfig validation failed: %s", err)
	}

	// Verify that the notifier configs are valid.
	_, err := arb_notifier.New(context.Background(), "fake", "fake", nil, c.Notifiers)
	return err
}

// Return the "type" of this roller.
func (c *AutoRollerConfig) RollerType() string {
	if c.rollerType == "" {
		if c.AFDORepoManager != nil {
			c.rollerType = ROLLER_TYPE_AFDO
		} else if c.AndroidRepoManager != nil {
			c.rollerType = ROLLER_TYPE_ANDROID
		} else if c.AssetRepoManager != nil {
			c.rollerType = ROLLER_TYPE_ASSET
		} else if c.CopyRepoManager != nil {
			c.rollerType = ROLLER_TYPE_COPY
		} else if c.DEPSRepoManager != nil {
			c.rollerType = ROLLER_TYPE_DEPS
		} else if c.FuchsiaSDKRepoManager != nil {
			c.rollerType = ROLLER_TYPE_FUCHSIA_SDK
		} else if c.GithubRepoManager != nil {
			c.rollerType = ROLLER_TYPE_GITHUB
		} else if c.GithubDEPSRepoManager != nil {
			c.rollerType = ROLLER_TYPE_GITHUB_DEPS
		} else if c.Google3RepoManager != nil {
			c.rollerType = ROLLER_TYPE_GOOGLE3
		} else if c.ManifestRepoManager != nil {
			c.rollerType = ROLLER_TYPE_MANIFEST
		} else if c.NoCheckoutDEPSRepoManager != nil {
			c.rollerType = ROLLER_TYPE_DEPS_NO_CHECKOUT
		} else {
			c.rollerType = ROLLER_TYPE_INVALID
		}
	}
	return c.rollerType
}
