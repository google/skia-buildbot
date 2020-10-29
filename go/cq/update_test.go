package cq

import (
	"regexp"
	"testing"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cv/api/config/v2"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils/unittest"
)

func fakeConfig() *config.Config {
	return &config.Config{
		DrainingStartTime: "2017-12-23T15:47:58Z",
		CqStatusHost:      "cq-status.com",
		SubmitOptions: &config.SubmitOptions{
			MaxBurst: 2,
			BurstDelay: &duration.Duration{
				Seconds: 60,
				Nanos:   3,
			},
		},
		ConfigGroups: []*config.ConfigGroup{
			{
				Name: git.DefaultBranch,
				Gerrit: []*config.ConfigGroup_Gerrit{
					{
						Url: "gerrit.com",
						Projects: []*config.ConfigGroup_Gerrit_Project{
							{
								Name:      "skia",
								RefRegexp: []string{MAIN_REF},
							},
						},
					},
				},
				CombineCls: &config.CombineCLs{
					StabilizationDelay: &duration.Duration{
						Seconds: 30,
						Nanos:   6,
					},
				},
				Verifiers: &config.Verifiers{
					GerritCqAbility: &config.Verifiers_GerritCQAbility{
						CommitterList:           []string{"committers"},
						DryRunAccessList:        []string{"dry-runners"},
						AllowSubmitWithOpenDeps: true,
						AllowOwnerIfSubmittable: 7,
					},
					TreeStatus: &config.Verifiers_TreeStatus{
						Url: "tree-status.com",
					},
					Tryjob: &config.Verifiers_Tryjob{
						Builders: []*config.Verifiers_Tryjob_Builder{
							{
								Name:           "fake-tryjob",
								IncludableOnly: true,
								LocationRegexp: []string{".*"},
							},
							{
								Name:                 "experimental-tryjob",
								ExperimentPercentage: 0.5,
							},
						},
						RetryConfig: &config.Verifiers_Tryjob_RetryConfig{
							SingleQuota:            1,
							GlobalQuota:            2,
							FailureWeight:          3,
							TransientFailureWeight: 4,
							TimeoutWeight:          5,
						},
					},
				},
			},
		},
	}
}

func TestCloneBranch(t *testing.T) {
	unittest.SmallTest(t)

	t.Run("clone all", func(t *testing.T) {
		expect := fakeConfig()
		cloneCg := fakeConfig().ConfigGroups[0] // main branch
		cloneCg.Name = "clone"
		cloneCg.Gerrit[0].Projects[0].RefRegexp[0] = "refs/heads/clone"
		expect.ConfigGroups = append(expect.ConfigGroups, cloneCg)
		actual := fakeConfig()
		require.NoError(t, CloneBranch(actual, git.DefaultBranch, "clone", true, true, nil))
		assertdeep.Equal(t, expect, actual)
	})

	t.Run("clone without experimental", func(t *testing.T) {
		expect := fakeConfig()
		cloneCg := fakeConfig().ConfigGroups[0] // main branch
		cloneCg.Name = "clone"
		cloneCg.Gerrit[0].Projects[0].RefRegexp[0] = "refs/heads/clone"
		cloneCg.Verifiers.Tryjob.Builders = cloneCg.Verifiers.Tryjob.Builders[:1]
		expect.ConfigGroups = append(expect.ConfigGroups, cloneCg)
		actual := fakeConfig()
		require.NoError(t, CloneBranch(actual, git.DefaultBranch, "clone", false, true, nil))
		assertdeep.Equal(t, expect, actual)
	})

	t.Run("clone without tree check", func(t *testing.T) {
		expect := fakeConfig()
		cloneCg := fakeConfig().ConfigGroups[0] // main branch
		cloneCg.Name = "clone"
		cloneCg.Gerrit[0].Projects[0].RefRegexp[0] = "refs/heads/clone"
		cloneCg.Verifiers.TreeStatus = nil
		expect.ConfigGroups = append(expect.ConfigGroups, cloneCg)
		actual := fakeConfig()
		require.NoError(t, CloneBranch(actual, git.DefaultBranch, "clone", true, false, nil))
		assertdeep.Equal(t, expect, actual)
	})

	t.Run("clone exclude regex", func(t *testing.T) {
		expect := fakeConfig()
		cloneCg := fakeConfig().ConfigGroups[0] // main branch
		cloneCg.Name = "clone"
		cloneCg.Gerrit[0].Projects[0].RefRegexp[0] = "refs/heads/clone"
		cloneCg.Verifiers.Tryjob.Builders = cloneCg.Verifiers.Tryjob.Builders[1:]
		expect.ConfigGroups = append(expect.ConfigGroups, cloneCg)
		excludeRe := regexp.MustCompile("^fake")
		actual := fakeConfig()
		require.NoError(t, CloneBranch(actual, git.DefaultBranch, "clone", true, true, []*regexp.Regexp{excludeRe}))
		assertdeep.Equal(t, expect, actual)
	})
}

func TestDeleteBranch(t *testing.T) {
	unittest.SmallTest(t)

	cloneCg := fakeConfig().ConfigGroups[0] // main branch
	cloneCg.Name = "clone"
	cloneCg.Gerrit[0].Projects[0].RefRegexp[0] = "refs/heads/clone"
	actual := fakeConfig()
	actual.ConfigGroups = append(actual.ConfigGroups, cloneCg)
	require.NoError(t, DeleteBranch(actual, "clone"))
	expect := fakeConfig()
	assertdeep.Equal(t, expect, actual)
}

func TestSerialize(t *testing.T) {
	unittest.SmallTest(t)

	for _, cfg := range []string{
		miniCfg,
		skiaCfg,
	} {
		newCfg, err := WithUpdateCQConfig([]byte(cfg), func(*config.Config) error {
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, cfg, string(newCfg))
	}
}

// miniCfg is a minimal commit-queue.cfg.
const miniCfg = `# See http://luci-config.appspot.com/schemas/projects:commit-queue.cfg for the
# documentation of this file format.

config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
    }
  }
}
`

// skiaCfg is Skia's commit-queue.cfg at the time of writing.
const skiaCfg = `# See http://luci-config.appspot.com/schemas/projects:commit-queue.cfg for the
# documentation of this file format.

cq_status_host: "chromium-cq-status.appspot.com"
submit_options {
  max_burst: 2
  burst_delay {
    seconds: 300
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/master"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tree_status {
      url: "https://tree-status.skia.org"
    }
    tryjob {
      builders {
        name: "chromium/try/linux-blink-rel"
        includable_only: true
      }
      builders {
        name: "chromium/try/linux_chromium_compile_dbg_ng"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-cf_x86_phone-eng-Android_Framework"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-host-sdk-Android_Framework"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Wuffs"
        location_regexp: ".+/[+]/src/codec/SkWuffs.*"
        location_regexp: ".+/[+]/DEPS"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Dawn"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Direct3D"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-arm64-Release-ANGLE"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Mac10.15-Clang-MacBookAir7.2-GPU-IntelHD6000-x86_64-Debug-All-Metal"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-TAP-Presubmit-G3_Framework"
        experiment_percentage: 100
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/android/next-release"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Wuffs"
        location_regexp: ".+/[+]/src/codec/SkWuffs.*"
        location_regexp: ".+/[+]/DEPS"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-arm64-Release-ANGLE"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Mac10.15-Clang-MacBookAir7.2-GPU-IntelHD6000-x86_64-Debug-All-Metal"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/skqp/release"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
        disable_reuse: true
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/skqp/dev"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Debug-Metal"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
        disable_reuse: true
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-iOS-Clang-iPhone7-GPU-PowerVRGT7600-arm64-Debug-All"
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/chrome/m80"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Wuffs"
        location_regexp: ".+/[+]/src/codec/SkWuffs.*"
        location_regexp: ".+/[+]/DEPS"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-arm64-Release-ANGLE"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
        disable_reuse: true
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Mac10.15-Clang-MacBookAir7.2-GPU-IntelHD6000-x86_64-Debug-All-Metal"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/chrome/m81"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Wuffs"
        location_regexp: ".+/[+]/src/codec/SkWuffs.*"
        location_regexp: ".+/[+]/DEPS"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-arm64-Release-ANGLE"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Mac10.15-Clang-MacBookAir7.2-GPU-IntelHD6000-x86_64-Debug-All-Metal"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/chrome/m82"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Wuffs"
        location_regexp: ".+/[+]/src/codec/SkWuffs.*"
        location_regexp: ".+/[+]/DEPS"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Direct3D"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-arm64-Release-ANGLE"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Mac10.15-Clang-MacBookAir7.2-GPU-IntelHD6000-x86_64-Debug-All-Metal"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
config_groups {
  gerrit {
    url: "https://skia-review.googlesource.com"
    projects {
      name: "skia"
      ref_regexp: "refs/heads/chrome/m83"
    }
  }
  verifiers {
    gerrit_cq_ability {
      committer_list: "project-skia-committers"
      dry_run_access_list: "project-skia-tryjob-access"
    }
    tree_status {
      url: "https://tree-status.skia.org"
    }
    tryjob {
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm-Release-Android_API26"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-arm64-Debug-Android"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Tidy"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-Clang-x86_64-Debug-Wuffs"
        location_regexp: ".+/[+]/src/codec/SkWuffs.*"
        location_regexp: ".+/[+]/DEPS"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-EMCC-wasm-Release-CanvasKit"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Debug-NoGPU_Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Debian10-GCC-x86_64-Release-Docker"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-arm64-Debug-iOS"
      }
      builders {
        name: "skia/skia.primary/Build-Mac-Clang-x86_64-Release"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86-Debug"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Dawn"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Direct3D"
      }
      builders {
        name: "skia/skia.primary/Build-Win-Clang-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-arm64-Release-ANGLE"
      }
      builders {
        name: "skia/skia.primary/Build-Win-MSVC-x86_64-Release-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-OnDemand-Presubmit"
      }
      builders {
        name: "skia/skia.primary/Housekeeper-PerCommit-InfraTests_Linux"
      }
      builders {
        name: "skia/skia.primary/Perf-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Android-Clang-GalaxyS6-GPU-MaliT760-arm64-Release-All-Android"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-GCE-CPU-AVX2-x86_64-Debug-All-BonusConfigs"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-asmjs-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Debian10-EMCC-GCE-CPU-AVX2-wasm-Release-All-PathKit"
      }
      builders {
        name: "skia/skia.primary/Test-Mac10.15-Clang-MacBookAir7.2-GPU-IntelHD6000-x86_64-Debug-All-Metal"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-ASAN"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Release-All"
      }
      builders {
        name: "skia/skia.primary/Test-Ubuntu18-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-DDL1"
      }
      builders {
        name: "skia/skia.primary/Test-Win10-Clang-NUC6i5SYK-GPU-IntelIris540-x86_64-Debug-All"
      }
      builders {
        name: "skia/skia.primary/Test-Win2019-Clang-GCE-CPU-AVX2-x86_64-Release-All"
      }
      retry_config {
        single_quota: 1
        global_quota: 2
        failure_weight: 2
        transient_failure_weight: 1
        timeout_weight: 2
      }
    }
  }
}
`
