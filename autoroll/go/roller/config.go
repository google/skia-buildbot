package roller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/codereview"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/time_window"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/util"
)

const (
	// Throttling parameters.
	DEFAULT_SAFETY_THROTTLE_ATTEMPT_COUNT = 3
	DEFAULT_SAFETY_THROTTLE_TIME_WINDOW   = 30 * time.Minute

	// Maximum roller name length. This is limited by Kubernetes, which has
	// a 63-character limit for various names. This length is derived from
	// that limit, accounting for the prefixes and suffixes which are
	// automatically added by our tooling, eg. the "autoroll-be-" prefix and
	// "-storage" suffix for disks, controller hashes, etc.
	MAX_ROLLER_NAME_LENGTH = 41

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

	// Code review settings.
	Gerrit        *codereview.GerritConfig  `json:"gerrit,omitempty"`
	Github        *codereview.GithubConfig  `json:"github,omitempty"`
	Google3Review *codereview.Google3Config `json:"google3Review,omitempty"`

	// RepoManager configs. Exactly one must be provided.
	AFDORepoManager              *repo_manager.AFDORepoManagerConfig              `json:"afdoRepoManager,omitempty"`
	AndroidRepoManager           *repo_manager.AndroidRepoManagerConfig           `json:"androidRepoManager,omitempty"`
	CopyRepoManager              *repo_manager.CopyRepoManagerConfig              `json:"copyRepoManager,omitempty"`
	DEPSRepoManager              *repo_manager.DEPSRepoManagerConfig              `json:"depsRepoManager,omitempty"`
	FreeTypeRepoManager          *repo_manager.FreeTypeRepoManagerConfig          `json:"freeTypeRepoManager"`
	FuchsiaSDKAndroidRepoManager *repo_manager.FuchsiaSDKAndroidRepoManagerConfig `json:"fuchsiaSDKAndroidRepoManager,omitempty"`
	FuchsiaSDKRepoManager        *repo_manager.FuchsiaSDKRepoManagerConfig        `json:"fuchsiaSDKRepoManager,omitempty"`
	GithubRepoManager            *repo_manager.GithubRepoManagerConfig            `json:"githubRepoManager,omitempty"`
	GithubCipdDEPSRepoManager    *repo_manager.GithubCipdDEPSRepoManagerConfig    `json:"githubCipdDEPSRepoManager,omitempty"`
	GithubDEPSRepoManager        *repo_manager.GithubDEPSRepoManagerConfig        `json:"githubDEPSRepoManager,omitempty"`
	Google3RepoManager           *Google3FakeRepoManagerConfig                    `json:"google3,omitempty"`
	ManifestRepoManager          *repo_manager.ManifestRepoManagerConfig          `json:"manifestRepoManager,omitempty"`
	NoCheckoutDEPSRepoManager    *repo_manager.NoCheckoutDEPSRepoManagerConfig    `json:"noCheckoutDEPSRepoManager,omitempty"`

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
	// Time window in which the roller is allowed to upload roll CLs. See
	// the go/time_window package for supported format.
	TimeWindow string `json:"timeWindow,omitempty"`
	// Throttling configuration to prevent uploading too many CLs within
	// too short a time period.
	SafetyThrottle *ThrottleConfig `json:"safetyThrottle,omitempty"`
	// If true, this roller supports one-click "manual" rolls.
	SupportsManualRolls bool `json:"supportsManualRolls,omitempty"`

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
	if c.ParentName == "" {
		return errors.New("ParentName is required.")
	}
	if c.ParentWaterfall == "" {
		return errors.New("ParentWaterfall is required.")
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

	rm := []util.Validator{}
	if c.AFDORepoManager != nil {
		rm = append(rm, c.AFDORepoManager)
	}
	if c.AndroidRepoManager != nil {
		rm = append(rm, c.AndroidRepoManager)
	}
	if c.CopyRepoManager != nil {
		rm = append(rm, c.CopyRepoManager)
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
	_, err := arb_notifier.New(context.Background(), "fake", "fake", "fake", nil, nil, nil, c.Notifiers)
	if err != nil {
		return err
	}

	// Verify that the TimeWindow is valid.
	_, err = time_window.Parse(c.TimeWindow)
	return err
}

// Return the "type" of this roller.
func (c *AutoRollerConfig) RollerType() string {
	if c.rollerType == "" {
		if c.AFDORepoManager != nil {
			c.rollerType = ROLLER_TYPE_AFDO
		} else if c.AndroidRepoManager != nil {
			c.rollerType = ROLLER_TYPE_ANDROID
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

// Return the code review config for the roller.
func (c *AutoRollerConfig) CodeReview() codereview.CodeReviewConfig {
	if c.Github != nil {
		return c.Github
	}
	if c.Google3Review != nil {
		return c.Google3Review
	}
	return c.Gerrit
}
