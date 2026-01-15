package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeConfig() *Config {
	gerritConfig := &GerritConfig{
		Url:     "fake.gerrit.url",
		Project: "fake-project",
		Config:  GerritConfig_CHROMIUM,
	}
	transitiveDeps := []*TransitiveDepConfig{
		{
			Child: &VersionFileConfig{
				Id: "transitive-dep-1",
				File: []*VersionFileConfig_File{
					{Path: "DEPS"},
				},
			},
			Parent: &VersionFileConfig{
				Id: "transitive-dep-1",
				File: []*VersionFileConfig_File{
					{Path: "DEPS"},
				},
			},
		},
		{
			Child: &VersionFileConfig{
				Id: "transitive-dep-2",
				File: []*VersionFileConfig_File{
					{Path: "DEPS"},
				},
			},
			Parent: &VersionFileConfig{
				Id: "transitive-dep-2",
				File: []*VersionFileConfig_File{
					{Path: "DEPS"},
				},
			},
		},
	}
	childDeps := []*VersionFileConfig{}
	for _, dep := range transitiveDeps {
		childDeps = append(childDeps, dep.Child.Copy())
	}
	return &Config{
		RollerName:        "test",
		ChildDisplayName:  "test",
		ParentDisplayName: "test",
		ParentWaterfall:   "https://test",
		OwnerPrimary:      "me",
		OwnerSecondary:    "you",
		Contacts:          []string{"us@google.com"},
		ServiceAccount:    "my-service-account",
		Reviewer:          []string{"me@google.com"},
		CommitMsg: &CommitMsgConfig{
			BuiltIn: CommitMsgConfig_DEFAULT,
		},
		CodeReview: &Config_Gerrit{
			Gerrit: gerritConfig,
		},
		Kubernetes: &KubernetesConfig{
			Cpu:    "1m",
			Memory: "10MB",
			Image:  "fake-image",
			Disk:   "10GB",
		},
		RepoManager: &Config_ParentChildRepoManager{
			ParentChildRepoManager: &ParentChildRepoManagerConfig{
				Parent: &ParentChildRepoManagerConfig_DepsLocalGerritParent{
					&DEPSLocalGerritParentConfig{
						DepsLocal: &DEPSLocalParentConfig{
							GitCheckout: &GitCheckoutParentConfig{
								GitCheckout: &GitCheckoutConfig{
									Branch:  "main",
									RepoUrl: "https://parent.repo.git",
								},
								Dep: &DependencyConfig{
									Primary: &VersionFileConfig{
										Id: "primary-dep-id",
										File: []*VersionFileConfig_File{
											{Path: "DEPS"},
										},
									},
									Transitive: transitiveDepConfigSlice(transitiveDeps).Copy(),
								},
							},
							ChildPath: "path/to/child",
						},
						Gerrit: gerritConfig,
					},
				},
				Child: &ParentChildRepoManagerConfig_GitCheckoutChild{
					&GitCheckoutChildConfig{
						GitCheckout: &GitCheckoutConfig{
							Branch:       "main",
							RepoUrl:      "https://child.repo.git",
							Dependencies: childDeps,
						},
					},
				},
			},
		},
		TransitiveDeps: transitiveDepConfigSlice(transitiveDeps).Copy(),
	}
}

func TestValidation_TransitiveDeps(t *testing.T) {
	// Verify that the vanilla config passes validation.
	t.Run("baseline", func(t *testing.T) {
		require.NoError(t, makeConfig().Validate())
	})
	t.Run("missing child dep", func(t *testing.T) {
		cfg := makeConfig()
		cfg.GetParentChildRepoManager().GetGitCheckoutChild().GitCheckout.Dependencies = cfg.GetParentChildRepoManager().GetGitCheckoutChild().GitCheckout.Dependencies[1:]
		require.ErrorContains(t, cfg.Validate(), "top level transitive dependency count 2 does not match dependency count 1 set on child")
	})
	t.Run("missing parent dep", func(t *testing.T) {
		cfg := makeConfig()
		cfg.GetParentChildRepoManager().GetDepsLocalGerritParent().DepsLocal.GitCheckout.Dep.Transitive = cfg.GetParentChildRepoManager().GetDepsLocalGerritParent().DepsLocal.GitCheckout.Dep.Transitive[1:]
		require.ErrorContains(t, cfg.Validate(), "top level transitive dependency count 2 does not match transitive dependency count 1 set on parent")
	})
	t.Run("missing top level dep", func(t *testing.T) {
		cfg := makeConfig()
		cfg.TransitiveDeps = cfg.TransitiveDeps[1:]
		require.ErrorContains(t, cfg.Validate(), "top level transitive dependency count 1 does not match transitive dependency count 2 set on parent")
	})
	t.Run("mismatched child dep", func(t *testing.T) {
		cfg := makeConfig()
		cfg.GetParentChildRepoManager().GetGitCheckoutChild().GitCheckout.Dependencies[0].Id += "oops"
		require.ErrorContains(t, cfg.Validate(), "top level transitive dependency differs from dependency set on child")
	})
	t.Run("mismatched parent dep", func(t *testing.T) {
		cfg := makeConfig()
		cfg.GetParentChildRepoManager().GetDepsLocalGerritParent().DepsLocal.GitCheckout.Dep.Transitive[0].LogUrlTmpl = "oops"
		require.ErrorContains(t, cfg.Validate(), "top level transitive dependency differs from transitive dependency set on parent")
	})
	t.Run("mismatched top level dep", func(t *testing.T) {
		cfg := makeConfig()
		cfg.TransitiveDeps[0].LogUrlTmpl = "oops"
		require.ErrorContains(t, cfg.Validate(), "top level transitive dependency differs from transitive dependency set on parent")
	})
}

func TestValidation_RollerName(t *testing.T) {
	t.Run("starts with dash", func(t *testing.T) {
		cfg := makeConfig()
		cfg.RollerName = "-" + cfg.RollerName
		require.ErrorContains(t, cfg.Validate(), "RollerName is invalid")
	})
	t.Run("ends with dash", func(t *testing.T) {
		cfg := makeConfig()
		cfg.RollerName = cfg.RollerName + "-"
		require.ErrorContains(t, cfg.Validate(), "RollerName is invalid")
	})
	t.Run("contains invalid chars", func(t *testing.T) {
		cfg := makeConfig()
		cfg.RollerName = "bad.name"
		require.ErrorContains(t, cfg.Validate(), "RollerName is invalid")
	})
}

func TestParseTrybotName(t *testing.T) {
	t.Run("luci.project.bucket:builder format", func(t *testing.T) {
		project, bucket, builders, err := ParseTrybotName("luci.chromium.try:dawn-linux-x64-deps-rel")
		assert.NoError(t, err)
		assert.Equal(t, "chromium", project)
		assert.Equal(t, "try", bucket)
		assert.Equal(t, []string{"dawn-linux-x64-deps-rel"}, builders)
	})
	t.Run("project/bucket:builder format", func(t *testing.T) {
		project, bucket, builders, err := ParseTrybotName("skia/skia.primary:Build-Ubuntu24.04-Clang-x86_64-Release-ANGLE")
		assert.NoError(t, err)
		assert.Equal(t, "skia", project)
		assert.Equal(t, "skia.primary", bucket)
		assert.Equal(t, []string{"Build-Ubuntu24.04-Clang-x86_64-Release-ANGLE"}, builders)
	})
	t.Run("multiple builders", func(t *testing.T) {
		project, bucket, builders, err := ParseTrybotName("luci.chromium.try:android-11-x86-rel,android-webview-10-x86-rel-tests")
		assert.NoError(t, err)
		assert.Equal(t, "chromium", project)
		assert.Equal(t, "try", bucket)
		assert.Equal(t, []string{"android-11-x86-rel", "android-webview-10-x86-rel-tests"}, builders)
	})

	// Failure cases.
	testFailure := func(name, input string) {
		t.Run(name, func(t *testing.T) {
			project, bucket, builders, err := ParseTrybotName(input)
			assert.Error(t, err, "got project=%s bucket=%s builders=%v", project, bucket, builders)
		})
	}
	testFailure("lone builder name", "lone-builder-name")
	testFailure("lone builder name with luci prefix", "luci.no-project-or-bucket")
	testFailure("builder without bucket", "luci.some-project:builder-without-bucket")
	testFailure("multiple builders without project or bucket", "multiple,builders,without,project")
	testFailure("empty name", "")
}
