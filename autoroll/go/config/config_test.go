package config

import (
	"testing"

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
