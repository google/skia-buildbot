package main

import (
	"fmt"

	"go.skia.org/infra/autoroll/go/config"
	"google.golang.org/protobuf/encoding/prototext"
)

func main() {
	cfg := &config.Config{
		RollerName:          "skia-autoroll",
		ChildDisplayName:    "Skia",
		ParentDisplayName:   "Chromium",
		OwnerPrimary:        "borenet",
		OwnerSecondary:      "rmistry",
		Contacts:            []string{"borenet@google.com"},
		ServiceAccount:      "chromium-autoroll@skia-public.iam.gserviceaccount.com",
		IsInternal:          false,
		Reviewer:            []string{"https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener"},
		SupportsManualRolls: true,
		CommitMsg: &config.CommitMsgConfig{
			BugProject:      "chromium",
			ChildLogUrlTmpl: "https://skia.googlesource.com/skia.git/+log/{{.RollingFrom}}..{{.RollingTo}}",
			CqExtraTrybots: []string{
				"luci.chromium.try:android_optional_gpu_tests_rel",
				"luci.chromium.try:linux-blink-rel",
				"luci.chromium.try:linux-chromeos-compile-dbg",
				"luci.chromium.try:linux_optional_gpu_tests_rel",
				"luci.chromium.try:mac_optional_gpu_tests_rel",
				"luci.chromium.try:win_optional_gpu_tests_rel",
			},
			CqDoNotCancelTrybots: false,
			IncludeLog:           true,
			IncludeRevisionCount: true,
			IncludeTbrLine:       true,
			IncludeTests:         true,
			Template: &config.CommitMsgConfig_BuiltIn_{
				BuiltIn: config.CommitMsgConfig_DEFAULT,
			},
		},
		CodeReview: &config.Config_Gerrit{
			// TODO(borenet): This is duplicated!
			Gerrit: &config.GerritConfig{
				Url:     "https://chromium-review.googlesource.com",
				Project: "chromium/src",
				Config:  config.GerritConfig_CHROMIUM,
			},
		},
		Kubernetes: &config.KubernetesConfig{
			Cpu:                          "1",
			Memory:                       "2Gi",
			ReadinessFailureThreshold:    30,
			ReadinessInitialDelaySeconds: 30,
			ReadinessPeriodSeconds:       10,
		},
		RepoManager: &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: &config.ParentChildRepoManagerConfig{
				Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
					GitilesParent: &config.GitilesParentConfig{
						Gitiles: &config.GitilesConfig{
							Branch:  "master",
							RepoUrl: "https://chromium.googlesource.com/chromium/src.git",
						},
						Dep: &config.DependencyConfig{
							Primary: &config.VersionFileConfig{
								Id:   "https://skia.googlesource.com/skia.git",
								Path: "src/third_party/skia",
							},
						},
						// TODO(borenet): This is duplicated!
						Gerrit: &config.GerritConfig{
							Url:     "https://chromium-review.googlesource.com",
							Project: "chromium/src",
							Config:  config.GerritConfig_CHROMIUM,
						},
					},
				},
				Child: &config.ParentChildRepoManagerConfig_GitilesChild{
					GitilesChild: &config.GitilesChildConfig{
						Gitiles: &config.GitilesConfig{
							Branch:  "master",
							RepoUrl: "https://skia.googlesource.com/skia.git",
						},
					},
				},
			},
		},
		Notifiers: []*config.NotifierConfig{
			{
				Filter: &config.NotifierConfig_LogLevel_{
					LogLevel: config.NotifierConfig_WARNING,
				},
				Config: &config.NotifierConfig_Email{
					Email: &config.EmailNotifierConfig{
						Emails: []string{"borenet@google.com"},
					},
				},
			},
		},
	}

	b, err := prototext.MarshalOptions{
		Multiline: true,
	}.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
