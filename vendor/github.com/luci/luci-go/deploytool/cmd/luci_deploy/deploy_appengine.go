// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/api/deploy"
	"github.com/luci/luci-go/deploytool/managedfs"
)

// gaeDeployment is a consolidated AppEngine deployment configuration. It
// includes staged configurations for specifically-deployed components, as well
// as global AppEngine state (e.g., index, queue, etc.).
type gaeDeployment struct {
	// project is the cloud project that this deployment is targeting.
	project *layoutDeploymentCloudProject

	// modules is the set of staged AppEngine modules that are being deployed. The
	// map is keyed on the module names.
	modules map[string]*stagedGAEModule
	// moduleNames is the sorted set of modules. It is generated during staging.
	moduleNames []string

	// versionModuleMap is a map of AppEngine module versions to the module names.
	//
	// For deployments whose modules all originate from the same Source, this
	// will have one entry. However, for multi-source deployments, this may have
	// more. Each entry translates to a "set_default_version" call on commit.
	versionModuleMap map[string][]string
	// yamlDir is the directory containing the AppEngine-wide YAMLs.
	yamlDir string
}

func makeGAEDeployment(project *layoutDeploymentCloudProject) *gaeDeployment {
	return &gaeDeployment{
		project: project,
		modules: make(map[string]*stagedGAEModule),
	}
}

func (d *gaeDeployment) addModule(module *layoutDeploymentGAEModule) {
	d.modules[module.ModuleName] = &stagedGAEModule{
		layoutDeploymentGAEModule: module,
		gaeDep: d,
	}
	d.moduleNames = append(d.moduleNames, module.ModuleName)
}

func (d *gaeDeployment) clearModules() {
	d.modules, d.moduleNames = nil, nil
}

func (d *gaeDeployment) stage(w *work, root *managedfs.Dir, params *deployParams) error {
	sort.Strings(d.moduleNames)

	// Generate a directory for our deployment's modules.
	moduleBaseDir, err := root.EnsureDirectory("modules")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create modules directory").Err()
	}

	// Stage each module in parallel. Also, generate AppEngine-wide YAMLs.
	err = w.RunMulti(func(workC chan<- func() error) {
		// Generate our AppEngine-wide YAML files.
		workC <- func() error {
			return d.generateYAMLs(w, root)
		}

		// Stage each AppEngine module.
		for _, name := range d.moduleNames {
			module := d.modules[name]
			workC <- func() error {
				moduleDir, err := moduleBaseDir.EnsureDirectory(string(module.comp.comp.title))
				if err != nil {
					return errors.Annotate(err).Reason("failed to create module directory for %(module)q").
						D("module", module.comp.comp.title).Err()
				}
				if err := module.stage(w, moduleDir); err != nil {
					return errors.Annotate(err).Reason("failed to stage module %(name)q").
						D("name", module.comp.comp.title).Err()
				}
				return nil
			}
		}
	})
	if err != nil {
		return errors.Annotate(err).Reason("failed to stage modules").Err()
	}

	// Build our verison/module map for commit.
	for _, name := range d.moduleNames {
		module := d.modules[name]
		if module.comp.dep.sg.Tainted && !params.commitTainted {
			log.Fields{
				"component": module.comp.String(),
			}.Warningf(w, "Not committing tainted component.")
			continue
		}

		if d.versionModuleMap == nil {
			d.versionModuleMap = make(map[string][]string)
		}

		version := module.version.String()
		moduleName := module.ModuleName
		if moduleName == "" {
			moduleName = "default"
		}
		d.versionModuleMap[version] = append(d.versionModuleMap[version], moduleName)
	}
	return nil
}

func (d *gaeDeployment) localBuild(w *work) error {
	// During the build phase, we simply assert that builds work as a sanity
	// check. This prevents us from having to engage remote deployment services
	// only to find that some fundamental build problem has occurred.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, name := range d.moduleNames {
			module := d.modules[name]
			workC <- func() error {
				// Run the module's build function.
				if module.localBuildFn == nil {
					return nil
				}
				return module.localBuildFn(w)
			}
		}
	})
}

func (d *gaeDeployment) push(w *work) error {
	// Run push  operations on modules in parallel.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, name := range d.moduleNames {
			module := d.modules[name]
			if module.pushFn != nil {
				workC <- func() error {
					return module.pushFn(w)
				}
			}
		}
	})
}

func (d *gaeDeployment) commit(w *work) error {
	appcfg, err := w.tools.appcfg()
	if err != nil {
		return errors.Annotate(err).Err()
	}

	// Set default modules for each module version.
	versions := make([]string, 0, len(d.versionModuleMap))
	for v := range d.versionModuleMap {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	err = w.RunMulti(func(workC chan<- func() error) {
		for _, v := range versions {
			modules := d.versionModuleMap[v]
			workC <- func() error {
				appcfgArgs := []string{
					"--application", d.project.Name,
					"--version", v,
					"--module", strings.Join(modules, ","),
				}
				if err := appcfg.exec("set_default_version", appcfgArgs...).check(w); err != nil {
					return errors.Annotate(err).Reason("failed to set default version").
						D("version", v).Err()
				}
				return nil
			}
		}
	})
	if err != nil {
		return errors.Annotate(err).Reason("failed to set default versions").Err()
	}

	for _, updateCmd := range []string{"update_dispatch", "update_indexes", "update_queues", "update_cron"} {
		if err := appcfg.exec(updateCmd, "--application", d.project.Name, d.yamlDir).check(w); err != nil {
			return errors.Annotate(err).Reason("failed to update %(cmd)q").D("cmd", updateCmd).Err()
		}
	}
	return nil
}

func (d *gaeDeployment) generateYAMLs(w *work, root *managedfs.Dir) error {
	// Get ALL AppEngine modules for this cloud project.
	var (
		err   error
		yamls = make(map[string]interface{}, 3)
	)

	yamls["index.yaml"] = gaeBuildIndexYAML(d.project)
	yamls["cron.yaml"] = gaeBuildCronYAML(d.project)
	if yamls["dispatch.yaml"], err = gaeBuildDispatchYAML(d.project); err != nil {
		return errors.Annotate(err).Reason("failed to generate dispatch.yaml").Err()
	}
	if yamls["queue.yaml"], err = gaeBuildQueueYAML(d.project); err != nil {
		return errors.Annotate(err).Reason("failed to generate index.yaml").Err()
	}

	for k, v := range yamls {
		f := root.File(k)
		if err := f.GenerateYAML(w, v); err != nil {
			return errors.Annotate(err).Reason("failed to generate %(yaml)q").D("yaml", k).Err()
		}
	}

	d.yamlDir = root.String()
	return nil
}

// stagedGAEModule is a single staged AppEngine module.
type stagedGAEModule struct {
	*layoutDeploymentGAEModule

	// gaeDep is the GAE deployment that this module belongs to.
	gaeDep *gaeDeployment

	// version is the calculated version for this module. It is populated during
	// staging.
	version *cloudProjectVersion

	// For Go AppEngine modules, the generated GOPATH.
	goPath []string

	localBuildFn func(*work) error
	pushFn       func(*work) error
}

// stage creates a staging space for an AppEngine module.
//
// root is the component module's root directory.
//
// The specific layout of the staging directory is based on the language and
// type of module.
//
// Go / Classic:
// -------------
// <root>/component/* (Contains shallow symlinks to
//                     <checkout>/path/to/component/*)
// <root>/component/{index,queue,cron}.yaml (Generated)
// <root>/component/source (Link to Component's source)
//
// Go / Managed VM (Same as Go/Classic, plus):
// -------------------------------------------
// <root>/component/Dockerfile (Generated Docker file)
func (m *stagedGAEModule) stage(w *work, root *managedfs.Dir) error {
	// Calculate our version.
	var err error
	m.version, err = makeCloudProjectVersion(m.comp.dep.cloudProject, m.comp.source())
	if err != nil {
		return errors.Annotate(err).Reason("failed to calculate cloud project version").Err()
	}

	// The directory where the base YAML and other depoyment-relative data will be
	// written.
	base := root

	// appYAMLPath is used in the immediate "switch" statement as a
	// parameter to some inline functions. It will be populated later in this
	// staging function, well before those inline functions are evaluated.
	//
	// It is a pointer so that if, somehow, this does not end up being the case,
	// we will panic instead of silently using an empty string.
	var appYAMLPath *string

	// Build each Component. We will delete any existing contents and leave it
	// unmanaged to allow our build system to put whatever files it wants in
	// there.
	buildDir, err := root.EnsureDirectory("build")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create build directory").Err()
	}
	if err := buildDir.CleanUp(); err != nil {
		return errors.Annotate(err).Reason("failed to cleanup build directory").Err()
	}
	buildDir.Ignore()

	// Build our Component into this directory.
	if err := buildComponent(w, m.comp, buildDir); err != nil {
		return errors.Annotate(err).Reason("failed to build component").Err()
	}

	switch t := m.GetRuntime().(type) {
	case *deploy.AppEngineModule_GoModule_:
		gom := t.GoModule

		// Construct a GOPATH for this module.
		goPath, err := root.EnsureDirectory("gopath")
		if err != nil {
			return errors.Annotate(err).Reason("failed to create GOPATH base").Err()
		}
		if err := stageGoPath(w, m.comp, goPath); err != nil {
			return errors.Annotate(err).Reason("failed to stage GOPATH").Err()
		}

		// Generate a stub Go package, which we will populate with an entry point.
		goSrcDir, err := root.EnsureDirectory("src")
		if err != nil {
			return errors.Annotate(err).Reason("failed to create stub source directory").Err()
		}

		mainPkg := fmt.Sprintf("%s/main", m.comp.comp.title)
		mainPkgParts := strings.Split(mainPkg, "/")
		mainPkgDir, err := goSrcDir.EnsureDirectory(mainPkgParts[0], mainPkgParts[1:]...)
		if err != nil {
			return errors.Annotate(err).Reason("failed to create directory for main package %(pkg)q").
				D("pkg", mainPkg).Err()
		}
		m.goPath = []string{root.String(), goPath.String()}

		// Choose how to push based on whether or not this is a Managed VM.
		if m.ManagedVm != nil {
			// If this is a Managed VM, symlink files from the main package.
			//
			// NOTE: This has the effect of prohibiting the entry point package for
			// GAE Managed VMs from importing "internal" directories, since this stub
			// space is outside of the main package space.
			pkgPath := findGoPackage(t.GoModule.EntryPackage, m.goPath)
			if pkgPath == "" {
				return errors.Reason("unable to find path for %(package)q").D("package", t.GoModule.EntryPackage).Err()
			}

			if err := mainPkgDir.ShallowSymlinkFrom(pkgPath, true); err != nil {
				return errors.Annotate(err).Reason("failed to create shallow symlink of main module").Err()
			}

			m.localBuildFn = func(w *work) error {
				return m.localBuildGo(w, t.GoModule.EntryPackage)
			}
			m.pushFn = func(w *work) error {
				return m.pushGoMVM(w, *appYAMLPath)
			}
		} else {
			// Generate a classic GAE stub. Since GAE works through "init()", all this
			// stub has to do is import the actual entry point package.
			if err := m.writeGoClassicGAEStub(w, mainPkgDir, gom.EntryPackage); err != nil {
				return errors.Annotate(err).Reason("failed to generate GAE classic entry stub").Err()
			}

			m.localBuildFn = func(w *work) error {
				return m.localBuildGo(w, mainPkg)
			}
			m.pushFn = func(w *work) error {
				return m.pushClassic(w, *appYAMLPath)
			}
		}

		// Write artifacts into the Go stub package path.
		base = mainPkgDir

	case *deploy.AppEngineModule_StaticModule_:
		m.pushFn = func(w *work) error {
			return m.pushClassic(w, *appYAMLPath)
		}
	}

	// Build our static files map.
	//
	// For each static files directory, symlink a generated directory immediately
	// under our source.
	staticMap := make(map[string]string)
	staticBuildPathMap := make(map[*deploy.BuildPath]string)
	if handlerSet := m.Handlers; handlerSet != nil {
		for _, h := range handlerSet.Handler {
			var bp *deploy.BuildPath
			switch t := h.GetContent().(type) {
			case *deploy.AppEngineModule_Handler_StaticBuildDir:
				bp = t.StaticBuildDir
			case *deploy.AppEngineModule_Handler_StaticFiles_:
				bp = t.StaticFiles.GetBuild()
			}
			if bp == nil {
				continue
			}

			// Have we already mapped this BuildPath?
			if _, ok := staticBuildPathMap[bp]; ok {
				continue
			}

			// Get the actual path for this BuildPath entry.
			dirPath, err := m.comp.buildPath(bp)
			if err != nil {
				return errors.Annotate(err).Reason("cannot resolve static directory").Err()
			}

			// Do we already have a static map entry for this filesystem source path?
			staticName, ok := staticMap[dirPath]
			if !ok {
				sd := base.File(fmt.Sprintf("_static_%d", len(staticMap)))
				if err := sd.SymlinkFrom(dirPath, true); err != nil {
					return errors.Annotate(err).Reason("failed to symlink static content for [%(path)s]").
						D("path", dirPath).Err()
				}
				staticName = sd.Name()
				staticMap[dirPath] = staticName
			}
			staticBuildPathMap[bp] = staticName
		}
	}

	// "app.yaml" / "module.yaml"
	appYAML, err := gaeBuildAppYAML(m.AppEngineModule, staticBuildPathMap)
	if err != nil {
		return errors.Annotate(err).Reason("failed to generate module YAML").Err()
	}

	appYAMLName := "module.yaml"
	if m.ModuleName == "" {
		// This is the default module, so name it "app.yaml".
		appYAMLName = "app.yaml"
	}
	f := base.File(appYAMLName)
	if err := f.GenerateYAML(w, appYAML); err != nil {
		return errors.Annotate(err).Reason("failed to generate %(filename)q file").
			D("filename", appYAMLName).Err()
	}
	s := f.String()
	appYAMLPath = &s

	// Cleanup our staging filesystem.
	if err := root.CleanUp(); err != nil {
		return errors.Annotate(err).Reason("failed to cleanup component directory").Err()
	}
	return nil
}

func (m *stagedGAEModule) writeGoClassicGAEStub(w *work, mainDir *managedfs.Dir, pkg string) error {
	main := mainDir.File("stub.go")
	return main.GenerateGo(w, fmt.Sprintf(``+
		`
// Package entry is a stub entry package for classic AppEngine application.
//
// The Go GAE module requires some .go files to be present, even if it is a pure
// static module, and the gae.py tool does not support different runtimes in the
// same deployment.
package entry

// Import our real package. This will cause its "init()" method to be invoked,
// registering its handlers with the AppEngine runtime.
import _ %q
`, pkg))
}

// pushClassic pushes a classic AppEngine module using "appcfg.py".
func (m *stagedGAEModule) pushClassic(w *work, appYAMLPath string) error {
	// Deploy classic.
	appcfg, err := w.tools.appcfg()
	if err != nil {
		return errors.Annotate(err).Err()
	}

	appPath, appYAML := filepath.Split(appYAMLPath)
	x := appcfg.exec(
		"--application", m.gaeDep.project.Name,
		"--version", m.version.String(),
		"--verbose",
		"update", appYAML,
	).cwd(appPath)
	x = addGoEnv(m.goPath, x)
	if err := x.check(w); err != nil {
		return errors.Annotate(err).Reason("failed to deploy classic GAE module").Err()
	}
	return nil
}

// localBuildGo performs a local build against a Go binary.
func (m *stagedGAEModule) localBuildGo(w *work, mainPkg string) error {
	gt, err := w.goTool(m.goPath)
	if err != nil {
		return errors.Annotate(err).Reason("failed to get Go tool").Err()
	}
	if err := gt.build(w, "", mainPkg); err != nil {
		return errors.Annotate(err).Reason("failed to build %(pkg)q").D("pkg", mainPkg).Err()
	}
	return nil
}

// pushGoMVM pushes a Go Managed VM version to the AppEngine instance.
func (m *stagedGAEModule) pushGoMVM(w *work, appYAMLPath string) error {
	appDir, appYAML := filepath.Split(appYAMLPath)

	// Deploy Managed VM.
	aedeploy, err := w.tools.aedeploy(m.goPath)
	if err != nil {
		return errors.Annotate(err).Err()
	}

	gcloud, err := w.tools.gcloud(m.gaeDep.project.Name)
	if err != nil {
		return errors.Annotate(err).Err()
	}

	// Deploy via: aedeploy gcloud preview app deploy
	//
	// We will promote it later on commit.
	gcloudArgs := []string{
		"preview", "app", "deploy", appYAML,
		"--version", m.version.String(),
		"--no-promote",
	}

	// Set verbosity based on current logging level.
	logLevel := log.GetLevel(w)
	switch logLevel {
	case log.Info:
		gcloudArgs = append(gcloudArgs, []string{"--verbosity", "info"}...)

	case log.Debug:
		gcloudArgs = append(gcloudArgs, []string{"--verbosity", "debug"}...)
	}

	x := aedeploy.bootstrap(gcloud.exec(gcloudArgs[0], gcloudArgs[1:]...)).outputAt(logLevel).cwd(appDir)
	if err := x.check(w); err != nil {
		return errors.Annotate(err).Reason("failed to deploy managed VM").Err()
	}
	return nil
}

// gaeAppYAML is a YAML struct for an AppEngine "app.yaml".
type gaeAppYAML struct {
	Module     string      `yaml:"module,omitempty"`
	Runtime    string      `yaml:"runtime,omitempty"`
	ThreadSafe *bool       `yaml:"threadsafe,omitempty"`
	APIVersion interface{} `yaml:"api_version,omitempty"`
	VM         bool        `yaml:"vm,omitempty"`

	BetaSettings *gaeAppYAMLBetaSettings `yaml:"beta_settings,omitempty"`

	Handlers []*gaeAppYAMLHandler `yaml:"handlers,omitempty"`
}

// gaeAppYAMLBetaSettings is a YAML struct for an AppEngine "app.yaml"
// beta settings.
type gaeAppYAMLBetaSettings struct {
	ServiceAccountScopes string `yaml:"service_account_scopes,omitempty"`
}

// gaeAppYAMLHAndler is a YAML struct for an AppEngine "app.yaml"
// handler entry.
type gaeAppYAMLHandler struct {
	URL    string `yaml:"url"`
	Script string `yaml:"script,omitempty"`

	Secure string `yaml:"secure,omitempty"`
	Login  string `yaml:"login,omitempty"`

	StaticDir   string `yaml:"static_dir,omitempty"`
	StaticFiles string `yaml:"static_files,omitempty"`
	Upload      string `yaml:"upload,omitempty"`
}

// gaeCronYAML is a YAML struct for an AppEngine "cron.yaml".
type gaeCronYAML struct {
	Cron []*gaeCronYAMLEntry `yaml:"cron,omitempty"`
}

// gaeCronYAMLEntry is a YAML struct for an AppEngine "cron.yaml" entry.
type gaeCronYAMLEntry struct {
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url"`
	Schedule    string `yaml:"schedule"`
	Target      string `yaml:"target,omitempty"`
}

// gaeIndexYAML is a YAML struct for an AppEngine "index.yaml".
type gaeIndexYAML struct {
	Indexes []*gaeIndexYAMLEntry `yaml:"indexes,omitempty"`
}

// gaeIndexYAMLEntry is a YAML struct for an AppEngine "index.yaml" entry.
type gaeIndexYAMLEntry struct {
	Kind       string                       `yaml:"kind"`
	Ancestor   string                       `yaml:"ancestor,omitempty"`
	Properties []*gaeIndexYAMLEntryProperty `yaml:"properties"`
}

// gaeIndexYAMLEntryProperty is a YAML struct for an AppEngine "index.yaml"
// entry's property.
type gaeIndexYAMLEntryProperty struct {
	Name      string `yaml:"name"`
	Direction string `yaml:"direction,omitempty"`
}

// gaeDispatchYAML is a YAML struct for an AppEngine "dispatch.yaml".
type gaeDispatchYAML struct {
	Dispatch []*gaeDispatchYAMLEntry `yaml:"dispatch,omitempty"`
}

// gaeDispatchYAMLEntry is a YAML struct for an AppEngine "dispatch.yaml" entry.
type gaeDispatchYAMLEntry struct {
	Module string `yaml:"module"`
	URL    string `yaml:"url"`
}

// gaeQueueYAML is a YAML struct for an AppEngine "queue.yaml".
type gaeQueueYAML struct {
	Queue []*gaeQueueYAMLEntry `yaml:"queue,omitempty"`
}

// gaeQueueYAMLEntry is a YAML struct for an AppEngine "queue.yaml" entry.
type gaeQueueYAMLEntry struct {
	Name string `yaml:"name"`
	Mode string `yaml:"mode"`

	Rate                  string                            `yaml:"rate,omitempty"`
	BucketSize            int                               `yaml:"bucket_size,omitempty"`
	MaxConcurrentRequests int                               `yaml:"max_concurrent_requests,omitempty"`
	RetryParameters       *gaeQueueYAMLEntryRetryParameters `yaml:"retry_parameters,omitempty"`
	Target                string                            `yaml:"target,omitempty"`
}

// gaeQueueYAMLEntryRetryParameters is a YAML struct for an AppEngine
// "queue.yaml" entry push queue retry parameters.
type gaeQueueYAMLEntryRetryParameters struct {
	TaskAgeLimit      string `yaml:"task_age_limit,omitempty"`
	MinBackoffSeconds int    `yaml:"min_backoff_seconds,omitempty"`
	MaxBackoffSeconds int    `yaml:"max_backoff_seconds,omitempty"`
	MaxDoublings      int    `yaml:"max_doublings,omitempty"`
}

func gaeBuildAppYAML(aem *deploy.AppEngineModule, staticMap map[*deploy.BuildPath]string) (*gaeAppYAML, error) {
	appYAML := gaeAppYAML{
		Module: aem.ModuleName,
	}

	var (
		defaultScript = ""
		isStub        = false
	)

	switch aem.GetRuntime().(type) {
	case *deploy.AppEngineModule_GoModule_:
		appYAML.Runtime = "go"

		if aem.ManagedVm != nil {
			appYAML.APIVersion = 1
			appYAML.VM = true
		} else {
			appYAML.APIVersion = "go1"
		}
		defaultScript = "_go_app"

	case *deploy.AppEngineModule_StaticModule_:
		// A static module presents itself as an empty Python AppEngine module. This
		// doesn't actually need any Python code to support it.
		appYAML.Runtime = "python27"
		appYAML.APIVersion = "1"
		threadSafe := true
		appYAML.ThreadSafe = &threadSafe
		isStub = true

	default:
		return nil, errors.Reason("unsupported runtime %(runtime)q").D("runtime", aem.Runtime).Err()
	}

	if mvm := aem.ManagedVm; mvm != nil {
		if len(mvm.Scopes) > 0 {
			appYAML.BetaSettings = &gaeAppYAMLBetaSettings{
				ServiceAccountScopes: strings.Join(mvm.Scopes, ","),
			}
		}
	}

	if handlerSet := aem.Handlers; handlerSet != nil {
		appYAML.Handlers = make([]*gaeAppYAMLHandler, len(handlerSet.Handler))
		for i, handler := range handlerSet.Handler {
			entry := gaeAppYAMLHandler{
				URL:    handler.Url,
				Secure: handler.Secure.AppYAMLString(),
				Login:  handler.Login.AppYAMLString(),
			}

			// Fill in static dir/file paths.
			switch t := handler.GetContent().(type) {
			case nil:
				entry.Script = defaultScript

			case *deploy.AppEngineModule_Handler_Script:
				entry.Script = t.Script
				if entry.Script == "" {
					entry.Script = defaultScript
				}

			case *deploy.AppEngineModule_Handler_StaticBuildDir:
				entry.StaticDir = staticMap[t.StaticBuildDir]

			case *deploy.AppEngineModule_Handler_StaticFiles_:
				sf := t.StaticFiles

				relDir := staticMap[sf.GetBuild()]
				entry.Upload = filepath.Join(relDir, sf.Upload)
				entry.StaticFiles = filepath.Join(relDir, sf.UrlMap)

			default:
				return nil, errors.Reason("don't know how to handle content %(content)T").
					D("content", t).D("url", handler.Url).Err()
			}

			if entry.Script != "" && isStub {
				return nil, errors.Reason("stub module cannot have entry script").Err()
			}

			appYAML.Handlers[i] = &entry
		}
	}

	return &appYAML, nil
}

func gaeBuildCronYAML(cp *layoutDeploymentCloudProject) *gaeCronYAML {
	var cronYAML gaeCronYAML

	for _, m := range cp.appEngineModules {
		for _, cron := range m.resources.Cron {
			cronYAML.Cron = append(cronYAML.Cron, &gaeCronYAMLEntry{
				Description: cron.Description,
				URL:         cron.Url,
				Schedule:    cron.Schedule,
				Target:      m.ModuleName,
			})
		}
	}
	return &cronYAML
}

func gaeBuildIndexYAML(cp *layoutDeploymentCloudProject) *gaeIndexYAML {
	var indexYAML gaeIndexYAML

	for _, index := range cp.resources.Index {
		entry := gaeIndexYAMLEntry{
			Kind:       index.Kind,
			Properties: make([]*gaeIndexYAMLEntryProperty, len(index.Property)),
		}
		if index.Ancestor {
			entry.Ancestor = "yes"
		}
		for i, prop := range index.Property {
			entry.Properties[i] = &gaeIndexYAMLEntryProperty{
				Name:      prop.Name,
				Direction: prop.Direction.AppYAMLString(),
			}
		}
		indexYAML.Indexes = append(indexYAML.Indexes, &entry)
	}
	return &indexYAML
}

func gaeBuildDispatchYAML(cp *layoutDeploymentCloudProject) (*gaeDispatchYAML, error) {
	var dispatchYAML gaeDispatchYAML

	for _, m := range cp.appEngineModules {
		for _, url := range m.resources.Dispatch {
			dispatchYAML.Dispatch = append(dispatchYAML.Dispatch, &gaeDispatchYAMLEntry{
				Module: m.ModuleName,
				URL:    url,
			})
		}
	}
	if count := len(dispatchYAML.Dispatch); count > 10 {
		return nil, errors.Reason("dispatch file has more than 10 routing rules (%(count)d)").
			D("count", count).Err()
	}
	return &dispatchYAML, nil
}

func gaeBuildQueueYAML(cp *layoutDeploymentCloudProject) (*gaeQueueYAML, error) {
	var queueYAML gaeQueueYAML

	for _, m := range cp.appEngineModules {
		for _, queue := range m.resources.TaskQueue {
			entry := gaeQueueYAMLEntry{
				Name: queue.Name,
			}
			switch t := queue.GetType().(type) {
			case *deploy.AppEngineResources_TaskQueue_Push_:
				push := t.Push

				entry.Target = m.ModuleName
				entry.Mode = "push"
				entry.Rate = push.Rate
				entry.BucketSize = int(push.BucketSize)
				entry.RetryParameters = &gaeQueueYAMLEntryRetryParameters{
					TaskAgeLimit:      push.RetryTaskAgeLimit,
					MinBackoffSeconds: int(push.RetryMinBackoffSeconds),
					MaxBackoffSeconds: int(push.RetryMaxBackoffSeconds),
					MaxDoublings:      int(push.RetryMaxDoublings),
				}

			default:
				return nil, errors.Reason("unknown task queue type %(type)T").D("type", t).Err()
			}

			queueYAML.Queue = append(queueYAML.Queue, &entry)
		}
	}
	return &queueYAML, nil
}
