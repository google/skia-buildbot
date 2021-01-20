package roller

import (
	"strconv"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/config"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/protobuf/types/known/durationpb"
)

// AutoRollerConfigToProto converts an AutoRollerConfig a config.Config.
func AutoRollerConfigToProto(cfg *AutoRollerConfig) (*config.Config, error) {
	rv := &config.Config{
		RollerName:          cfg.RollerName,
		ChildDisplayName:    cfg.ChildDisplayName,
		ParentDisplayName:   cfg.ParentDisplayName,
		ParentWaterfall:     cfg.ParentWaterfall,
		OwnerPrimary:        cfg.OwnerPrimary,
		OwnerSecondary:      cfg.OwnerSecondary,
		Contacts:            cfg.Contacts,
		ServiceAccount:      cfg.ServiceAccount,
		IsInternal:          cfg.IsInternal,
		Reviewer:            cfg.Sheriff,
		ReviewerBackup:      cfg.SheriffBackup,
		TimeWindow:          cfg.TimeWindow,
		SupportsManualRolls: cfg.SupportsManualRolls,
	}

	if cfg.MaxRollFrequency != "" {
		cooldown, err := human.ParseDuration(cfg.MaxRollFrequency)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RollCooldown = durationpb.New(cooldown)
	}

	if cfg.CommitMsgConfig != nil {
		rv.CommitMsg = commit_msg.CommitMsgConfigToProto(cfg.CommitMsgConfig)
	}

	var gerrit *config.GerritConfig
	if cfg.Gerrit != nil {
		gerrit = codereview.GerritConfigToProto(cfg.Gerrit)
		rv.CodeReview = &config.Config_Gerrit{
			Gerrit: gerrit,
		}
	} else if cfg.Github != nil {
		rv.CodeReview = &config.Config_Github{
			Github: codereview.GithubConfigToProto(cfg.Github),
		}
	} else if cfg.Google3Review != nil {
		rv.CodeReview = &config.Config_Google3{
			Google3: codereview.Google3ConfigToProto(cfg.Google3Review),
		}
	}

	if cfg.Kubernetes != nil {
		rft, err := strconv.ParseInt(cfg.Kubernetes.ReadinessFailureThreshold, 10, 32)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rids, err := strconv.ParseInt(cfg.Kubernetes.ReadinessInitialDelaySeconds, 10, 32)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rps, err := strconv.ParseInt(cfg.Kubernetes.ReadinessPeriodSeconds, 10, 32)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.Kubernetes = &config.KubernetesConfig{
			Cpu:                          cfg.Kubernetes.CPU,
			Memory:                       cfg.Kubernetes.Memory,
			ReadinessFailureThreshold:    int32(rft),
			ReadinessInitialDelaySeconds: int32(rids),
			ReadinessPeriodSeconds:       int32(rps),
			Disk:                         cfg.Kubernetes.Disk,
		}
		for _, secret := range cfg.Kubernetes.Secrets {
			rv.Kubernetes.Secrets = append(rv.Kubernetes.Secrets, &config.KubernetesSecret{
				Name:      secret.Name,
				MountPath: secret.MountPath,
			})
		}
	}

	for _, notifier := range cfg.Notifiers {
		c, err := arb_notifier.ConfigToProto(notifier)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.Notifiers = append(rv.Notifiers, c)
	}

	if cfg.SafetyThrottle != nil {
		rv.SafetyThrottle = &config.ThrottleConfig{
			AttemptCount: int32(cfg.SafetyThrottle.AttemptCount),
			TimeWindow:   durationpb.New(cfg.SafetyThrottle.TimeWindow),
		}
	}

	// TODO(borenet): Why isn't this part of the repo manager config?
	for _, td := range cfg.TransitiveDeps {
		rv.TransitiveDeps = append(rv.TransitiveDeps, version_file_common.TransitiveDepConfigToProto(td))
	}

	if cfg.AndroidRepoManager != nil {
		rv.RepoManager = &config.Config_AndroidRepoManager{
			AndroidRepoManager: repo_manager.AndroidRepoManagerConfigToProto(cfg.AndroidRepoManager),
		}
	} else if cfg.CommandRepoManager != nil {
		rv.RepoManager = &config.Config_CommandRepoManager{
			CommandRepoManager: repo_manager.CommandRepoManagerConfigToProto(cfg.CommandRepoManager),
		}
	} else if cfg.CopyRepoManager != nil {
		rm, err := repo_manager.CopyRepoManagerConfigToProto(cfg.CopyRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.DEPSGitilesRepoManager != nil {
		rm, err := repo_manager.DEPSGitilesRepoManagerConfigToProto(cfg.DEPSGitilesRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.DEPSRepoManager != nil {
		rm, err := repo_manager.DEPSRepoManagerConfigToProto(cfg.DEPSRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.FreeTypeRepoManager != nil {
		rv.RepoManager = &config.Config_FreetypeRepoManager{
			FreetypeRepoManager: repo_manager.FreeTypeRepoManagerConfigToProto(cfg.FreeTypeRepoManager),
		}
	} else if cfg.FuchsiaSDKAndroidRepoManager != nil {
		rv.RepoManager = &config.Config_FuchsiaSdkAndroidRepoManager{
			FuchsiaSdkAndroidRepoManager: repo_manager.FuchsiaSDKAndroidRepoManagerConfigToProto(cfg.FuchsiaSDKAndroidRepoManager),
		}
	} else if cfg.FuchsiaSDKRepoManager != nil {
		rm, err := repo_manager.FuchsiaSDKRepoManagerConfigToProto(cfg.FuchsiaSDKRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.GithubRepoManager != nil {
		rm, err := repo_manager.GithubRepoManagerConfigToProto(cfg.GithubRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.GithubCipdDEPSRepoManager != nil {
		rm, err := repo_manager.GithubCipdDEPSRepoManagerConfigToProto(cfg.GithubCipdDEPSRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.GithubDEPSRepoManager != nil {
		rm, err := repo_manager.GithubDEPSRepoManagerConfigToProto(cfg.GithubDEPSRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.GitilesCIPDDEPSRepoManager != nil {
		rm, err := repo_manager.GitilesCIPDDEPSRepoManagerConfigToProto(cfg.GitilesCIPDDEPSRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.Google3RepoManager != nil {
		rv.RepoManager = &config.Config_Google3RepoManager{
			Google3RepoManager: &config.Google3RepoManagerConfig{
				ChildBranch: cfg.Google3RepoManager.ChildBranch,
				ChildRepo:   cfg.Google3RepoManager.ChildRepo,
			},
		}
	} else if cfg.NoCheckoutDEPSRepoManager != nil {
		rm, err := repo_manager.NoCheckoutDEPSRepoManagerConfigToProto(cfg.NoCheckoutDEPSRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	} else if cfg.SemVerGCSRepoManager != nil {
		rm, err := repo_manager.SemVerGCSRepoManagerConfigToProto(cfg.SemVerGCSRepoManager)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.RepoManager = &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: rm,
		}
	}

	return rv, nil
}

// ProtoToConfig converts a config.Config to an AutoRollerConfig.
func ProtoToConfig(cfg *config.Config) (*AutoRollerConfig, error) {
	rv := &AutoRollerConfig{
		RollerName:          cfg.RollerName,
		ChildDisplayName:    cfg.ChildDisplayName,
		ParentDisplayName:   cfg.ParentDisplayName,
		ParentWaterfall:     cfg.ParentWaterfall,
		OwnerPrimary:        cfg.OwnerPrimary,
		OwnerSecondary:      cfg.OwnerSecondary,
		Contacts:            cfg.Contacts,
		ServiceAccount:      cfg.ServiceAccount,
		IsInternal:          cfg.IsInternal,
		Sheriff:             cfg.Reviewer,
		SheriffBackup:       cfg.ReviewerBackup,
		TimeWindow:          cfg.TimeWindow,
		SupportsManualRolls: cfg.SupportsManualRolls,
	}
	if cfg.RollCooldown.AsDuration() > 0 {
		rv.MaxRollFrequency = strings.TrimSpace(human.Duration(cfg.RollCooldown.AsDuration()))
	}
	if cfg.CommitMsg != nil {
		rv.CommitMsgConfig = commit_msg.ProtoToCommitMsgConfig(cfg.CommitMsg)
	}
	if cfg.CodeReview != nil {
		if gerrit, ok := cfg.CodeReview.(*config.Config_Gerrit); ok {
			rv.Gerrit = codereview.ProtoToGerritConfig(gerrit.Gerrit)
		} else if github, ok := cfg.CodeReview.(*config.Config_Github); ok {
			rv.Github = codereview.ProtoToGithubConfig(github.Github)
		} else if google3, ok := cfg.CodeReview.(*config.Config_Google3); ok {
			rv.Google3Review = codereview.ProtoToGoogle3Config(google3.Google3)
		}
	}

	if cfg.Kubernetes != nil {
		rv.Kubernetes = &KubernetesConfig{
			CPU:                          cfg.Kubernetes.Cpu,
			Memory:                       cfg.Kubernetes.Memory,
			ReadinessFailureThreshold:    strconv.FormatInt(int64(cfg.Kubernetes.ReadinessFailureThreshold), 10),
			ReadinessInitialDelaySeconds: strconv.FormatInt(int64(cfg.Kubernetes.ReadinessInitialDelaySeconds), 10),
			ReadinessPeriodSeconds:       strconv.FormatInt(int64(cfg.Kubernetes.ReadinessPeriodSeconds), 10),
			Disk:                         cfg.Kubernetes.Disk,
		}
		for _, secret := range cfg.Kubernetes.Secrets {
			rv.Kubernetes.Secrets = append(rv.Kubernetes.Secrets, &KubernetesSecret{
				Name:      secret.Name,
				MountPath: secret.MountPath,
			})
		}
	}

	for _, notifier := range cfg.Notifiers {
		rv.Notifiers = append(rv.Notifiers, arb_notifier.ProtoToConfig(notifier))
	}

	if cfg.SafetyThrottle != nil {
		rv.SafetyThrottle = &ThrottleConfig{
			AttemptCount: int64(cfg.SafetyThrottle.AttemptCount),
			TimeWindow:   cfg.SafetyThrottle.TimeWindow.AsDuration(),
		}
	}

	// TODO(borenet): Why isn't this part of the repo manager config?
	for _, td := range cfg.TransitiveDeps {
		rv.TransitiveDeps = append(rv.TransitiveDeps, version_file_common.ProtoToTransitiveDepConfig(td))
	}

	var err error
	if rm, ok := cfg.RepoManager.(*config.Config_AndroidRepoManager); ok {
		rv.AndroidRepoManager, err = repo_manager.ProtoToAndroidRepoManagerConfig(rm.AndroidRepoManager)
	} else if rm, ok := cfg.RepoManager.(*config.Config_CommandRepoManager); ok {
		rv.CommandRepoManager, err = repo_manager.ProtoToCommandRepoManagerConfig(rm.CommandRepoManager)
	} else if rm, ok := cfg.RepoManager.(*config.Config_FreetypeRepoManager); ok {
		rv.FreeTypeRepoManager, err = repo_manager.ProtoToFreeTypeRepoManagerConfig(rm.FreetypeRepoManager)
	} else if rm, ok := cfg.RepoManager.(*config.Config_FuchsiaSdkAndroidRepoManager); ok {
		rv.FuchsiaSDKAndroidRepoManager, err = repo_manager.ProtoToFuchsiaSDKAndroidRepoManagerConfig(rm.FuchsiaSdkAndroidRepoManager)
	} else if rm, ok := cfg.RepoManager.(*config.Config_Google3RepoManager); ok {
		rv.Google3RepoManager = &Google3FakeRepoManagerConfig{
			ChildBranch: rm.Google3RepoManager.ChildBranch,
			ChildRepo:   rm.Google3RepoManager.ChildRepo,
		}
	} else if rm, ok := cfg.RepoManager.(*config.Config_ParentChildRepoManager); ok {
		child := rm.ParentChildRepoManager.Child
		parent := rm.ParentChildRepoManager.Parent
		if _, ok := parent.(*config.ParentChildRepoManagerConfig_CopyParent); ok {
			rv.CopyRepoManager, err = repo_manager.ProtoToCopyRepoManagerConfig(rm.ParentChildRepoManager)
		} else if _, ok := parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent); ok {
			if _, ok := child.(*config.ParentChildRepoManagerConfig_GitCheckoutChild); ok {
				rv.GithubDEPSRepoManager, err = repo_manager.ProtoToGithubDEPSRepoManagerConfig(rm.ParentChildRepoManager)
			} else if _, ok := child.(*config.ParentChildRepoManagerConfig_CipdChild); ok {
				rv.GithubCipdDEPSRepoManager, err = repo_manager.ProtoToGithubCipdDEPSRepoManagerConfig(rm.ParentChildRepoManager)
			}
		} else if _, ok := parent.(*config.ParentChildRepoManagerConfig_DepsLocalGerritParent); ok {
			if _, ok := child.(*config.ParentChildRepoManagerConfig_GitilesChild); ok {
				rv.DEPSGitilesRepoManager, err = repo_manager.ProtoToDEPSGitilesRepoManagerConfig(rm.ParentChildRepoManager)
			} else if _, ok := child.(*config.ParentChildRepoManagerConfig_GitCheckoutChild); ok {
				rv.DEPSRepoManager, err = repo_manager.ProtoToDEPSRepoManagerConfig(rm.ParentChildRepoManager)
			}
		} else if _, ok := parent.(*config.ParentChildRepoManagerConfig_GitCheckoutGithubFileParent); ok {
			rv.GithubRepoManager, err = repo_manager.ProtoToGithubRepoManagerConfig(rm.ParentChildRepoManager)
		} else if _, ok := parent.(*config.ParentChildRepoManagerConfig_GitilesParent); ok {
			if _, ok := child.(*config.ParentChildRepoManagerConfig_CipdChild); ok {
				rv.GitilesCIPDDEPSRepoManager, err = repo_manager.ProtoToGitilesCIPDDEPSRepoManagerConfig(rm.ParentChildRepoManager)
			} else if _, ok := child.(*config.ParentChildRepoManagerConfig_FuchsiaSdkChild); ok {
				rv.FuchsiaSDKRepoManager, err = repo_manager.ProtoToFuchsiaSDKRepoManagerConfig(rm.ParentChildRepoManager)
			} else if _, ok := child.(*config.ParentChildRepoManagerConfig_GitilesChild); ok {
				rv.NoCheckoutDEPSRepoManager, err = repo_manager.ProtoToNoCheckoutDEPSRepoManagerConfig(rm.ParentChildRepoManager)
			} else if _, ok := child.(*config.ParentChildRepoManagerConfig_SemverGcsChild); ok {
				rv.SemVerGCSRepoManager, err = repo_manager.ProtoToSemVerGCSRepoManagerConfig(rm.ParentChildRepoManager)
			}
		}
	}

	return rv, skerr.Wrap(err)
}
