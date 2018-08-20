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
		TimeWindow:   human.Duration(c.TimeWindow),
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
type google3FakeRepoManagerConfig struct{}

// See documentation for util.Validator interface.
func (c *google3FakeRepoManagerConfig) Validate() error {
	return nil
}

// AutoRollerConfig contains configuration information for an AutoRoller.
type AutoRollerConfig struct {
	// Required Fields.

	// User friendly name of the child repo.
	ChildName string `json:"childName"`
	// Gerrit URL the roller will be uploading issues to.
	GerritURL string `json:"gerritURL"`
	// User friendly name of the parent repo.
	ParentName string `json:"parentName"`
	// URL of the waterfall/status display for the parent repo.
	ParentWaterfall string `json:"parentWaterfall"`
	// Email addresses to CC on rolls, or URL from which to obtain those
	// email addresses.
	Sheriff []string `json:"sheriff"`
	// Backup email addresses to CC on rolls, in case obtaining the email
	// addresses from the URL fails.  Only required if a URL is specified
	// for Sheriff.
	SheriffBackup []string `json:"sheriffBackup"`

	// Github code review flags.
	GithubRepoOwner string `json:"githubRepoOwner"`
	GithubRepoName  string `json:"githubRepoName"`
	GithubChecksNum int    `json:"githubChecksNum"`

	// RepoManager configs. Exactly one must be provided.
	AFDORepoManager           *repo_manager.AFDORepoManagerConfig           `json:"afdoRepoManager"`
	AndroidRepoManager        *repo_manager.AndroidRepoManagerConfig        `json:"androidRepoManager"`
	CopyRepoManager           *repo_manager.CopyRepoManagerConfig           `json:"copyRepoManager"`
	DEPSRepoManager           *repo_manager.DEPSRepoManagerConfig           `json:"depsRepoManager"`
	FuchsiaSDKRepoManager     *repo_manager.FuchsiaSDKRepoManagerConfig     `json:"fuchsiaSDKRepoManager"`
	GithubRepoManager         *repo_manager.GithubRepoManagerConfig         `json:"githubRepoManager"`
	GithubDEPSRepoManager     *repo_manager.GithubDEPSRepoManagerConfig     `json:"githubDEPSRepoManager"`
	Google3RepoManager        *google3FakeRepoManagerConfig                 `json:"google3"`
	ManifestRepoManager       *repo_manager.ManifestRepoManagerConfig       `json:"manifestRepoManager"`
	NoCheckoutDEPSRepoManager *repo_manager.NoCheckoutDEPSRepoManagerConfig `json:"noCheckoutDEPSRepoManager"`

	// Optional Fields.

	// Comma-separated list of trybots to add to roll CLs, in addition to
	// the default set of commit queue trybots.
	CqExtraTrybots []string `json:"cqExtraTrybots"`
	// Limit to one successful roll within this time period.
	MaxRollFrequency string `json:"maxRollFrequency"`
	// Any extra notification systems to be used for this roller.
	Notifiers []*notifier.Config `json:"notifiers"`
	// Throttling configuration to prevent uploading too many CLs within
	// too short a time period.
	SafetyThrottle *ThrottleConfig `json:"safetyThrottle"`

	// Private.
	rollerType string // Set by RollerType().
}

// Validate the config.
func (c *AutoRollerConfig) Validate() error {
	if c.ChildName == "" {
		return errors.New("ChildName is required.")
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

	// Verify that the notifier configs are valid.
	a := arb_notifier.New("fake", "fake", nil)
	return a.Router().AddFromConfigs(context.Background(), c.Notifiers)
}

// Return a metrics-friendly name for the roller based on the config.
func (c *AutoRollerConfig) RollerName() string {
	return strings.ToLower(c.ChildName) + "-" + strings.ToLower(c.ParentName)
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
