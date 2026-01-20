package parent

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"context"
	"net/http"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/metrics2"
)

// Placeholders is a struct containing placeholder variables which may be
// substituted into various parts of commands.
type Placeholders struct {
	CIPDRoot      string
	ParentRepoDir string
	PathVar       string
	RollingFromID string
	RollingToID   string
}

// String substitutes the Placeholders into the given string.
func (p Placeholders) String(s string) string {
	s = strings.ReplaceAll(s, "${cipd_root}", p.CIPDRoot)
	s = strings.ReplaceAll(s, "${parent_dir}", p.ParentRepoDir)
	s = strings.ReplaceAll(s, "${PATH}", p.PathVar)
	s = strings.ReplaceAll(s, "${rolling_from}", p.RollingFromID)
	s = strings.ReplaceAll(s, "${rolling_to}", p.RollingToID)
	return s
}

// Strings substitutes the Placeholders into the given string slice.
func (p Placeholders) Strings(strs []string) []string {
	cpy := make([]string, 0, len(strs))
	for _, s := range strs {
		cpy = append(cpy, p.String(s))
	}
	return cpy
}

// CIPDPackage substitutes the Placeholders into the given CIPDPackage package
// config.
func (p Placeholders) CIPDPackage(pkg *config.PreUploadCIPDPackageConfig) (*cipd.Package, error) {
	pkgName := p.String(pkg.Name)
	pkgVersion := p.String(pkg.Version)
	if pkgVersion == "${use_pinned_version}" {
		builtin, err := cipd.GetPackage(pkgName)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		pkgVersion = builtin.Version
	}
	return &cipd.Package{
		Name:    pkgName,
		Path:    p.String(pkg.Path),
		Version: pkgVersion,
	}, nil
}

// CIPDPackages substitutes the Placeholders into the given CIPDPackage configs.
func (p Placeholders) CIPDPackages(pkgs []*config.PreUploadCIPDPackageConfig) ([]*cipd.Package, error) {
	rv := []*cipd.Package{}
	for _, pkgCfg := range pkgs {
		pkg, err := p.CIPDPackage(pkgCfg)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, pkg)
	}
	return rv, nil
}

// Command substitutes the Placeholders into the given Command.
func (p Placeholders) Command(ctx context.Context, cmd []string, cwd string, env []string) (*exec.Command, error) {
	env = p.Strings(env)
	pathVar := os.Getenv("PATH")
	for _, envVar := range env {
		split := strings.SplitN(envVar, "=", 2)
		if len(split) == 2 && split[0] == "PATH" {
			pathVar = split[1]
		}
	}
	cmd = p.Strings(cmd)
	executable, err := exec.LookPath(ctx, cmd[0], pathVar)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &exec.Command{
		Name: executable,
		Args: cmd[1:],
		Dir:  p.String(cwd),
		Env:  env,
	}, nil
}

// PreUploadConfig substitutes the Placeholders into the PreUploadConfig.
func (p Placeholders) PreUploadConfig(ctx context.Context, env []string, cfg *config.PreUploadConfig) ([]*cipd.Package, []*exec.Command, error) {
	cipdPkgs, err := p.CIPDPackages(cfg.CipdPackage)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	cmds := make([]*exec.Command, 0, len(cfg.Command))
	for _, cmdCfg := range cfg.Command {
		cmdEnv := exec.MergeEnv(env, cmdCfg.Env)
		cmd, err := p.Command(ctx, strings.Fields(cmdCfg.Command), cmdCfg.Cwd, cmdEnv)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		cmds = append(cmds, cmd)
	}
	return cipdPkgs, cmds, nil
}

// RunPreUploadStep runs a pre-upload step as specified by the given config.
func RunPreUploadStep(ctx context.Context, cfg *config.PreUploadConfig, env []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	if cfg == nil {
		return nil
	}

	defer metrics2.FuncTimer().Stop()
	preUploadStepFailure := int64(1)
	defer func() {
		metrics2.GetInt64Metric("pre_upload_step_failure", nil).Update(preUploadStepFailure)
	}()
	sklog.Info("Running pre-upload step...")

	cipdRoot := path.Join(os.TempDir(), "cipd")

	// "Magic" variables.
	path := os.Getenv("PATH")
	for _, envVar := range env {
		split := strings.SplitN(envVar, "=", 2)
		if len(split) == 2 && split[0] == "PATH" {
			path = split[1]
		}
	}
	p := Placeholders{
		CIPDRoot:      cipdRoot,
		ParentRepoDir: parentRepoDir,
		PathVar:       path,
		RollingFromID: from.Id,
		RollingToID:   to.Id,
	}
	cipdPkgs, cmds, err := p.PreUploadConfig(ctx, env, cfg)
	if err != nil {
		return skerr.Wrap(err)
	}
	if len(cipdPkgs) > 0 {
		sklog.Info("Installing CIPD packages...")
		if err := cipd.Ensure(ctx, cipdRoot, true, cipdPkgs...); err != nil {
			return skerr.Wrap(err)
		}
	}
	for idx, cmd := range cmds {
		sklog.Infof("Running command: %s %s", cmd.Name, strings.Join(cmd.Args, " "))
		output, err := exec.RunCommand(ctx, cmd)
		if err != nil {
			if cfg.Command[idx].IgnoreFailure {
				sklog.Errorf("%s", err)
			} else {
				return skerr.Wrap(err)
			}
		} else {
			dumpLogs := false
			for _, arg := range append([]string{cmd.Name}, cmd.Args...) {
				if strings.Contains(arg, "roll_chromium_deps") {
					dumpLogs = true
					break
				}
			}
			if dumpLogs {
				sklog.Info(output)
			}
		}
	}
	preUploadStepFailure = 0
	return nil
}
