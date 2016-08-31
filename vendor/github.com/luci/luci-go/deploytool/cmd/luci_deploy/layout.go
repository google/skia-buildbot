// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/api/deploy"
	"github.com/luci/luci-go/deploytool/managedfs"

	"golang.org/x/net/context"
)

const defaultLayoutFilename = "layout.cfg"

type layoutSource struct {
	*deploy.FrozenLayout_Source

	sg    *layoutSourceGroup
	title title
}

func (s *layoutSource) String() string { return joinPath(s.sg.title, s.title) }

func (s *layoutSource) checkoutPath() string {
	return s.sg.layout.workingPathTo(s.Relpath)
}

// pathTo resolves a path, p, specified in a directory with source-root-relative
// path relpath.
//
//	- If the path begins with "/", it is taken relative to the source root.
//	- If the path does not begin with "/", it is taken relative to "relpath"
//	  within the source root (e.g., "relpath" is prepended to it).
func (s *layoutSource) pathTo(p, relpath string) string {
	if s.Relpath == "" {
		panic(errors.Reason("source %(source)q is not checked out").D("source", s).Err())
	}

	// If this is absolute, take it relative to source root.
	if strings.HasPrefix(p, "/") {
		relpath = ""
	}

	// Convert "p" to a source-relative absolute path.
	p = strings.Trim(p, "/")
	if relpath != "" {
		relpath = strings.Trim(relpath, "/")
		p = relpath + "/" + p
	}

	return deployToNative(s.checkoutPath(), p)
}

// layoutSourceGroup is a group of named, associated Source entries.
type layoutSourceGroup struct {
	*deploy.FrozenLayout_SourceGroup

	// layout is the owning layout.
	layout *deployLayout

	title   title
	sources map[title]*layoutSource
}

func (sg *layoutSourceGroup) String() string { return string(sg.title) }

func (sg *layoutSourceGroup) allSources() []*layoutSource {
	keys := make([]string, 0, len(sg.sources))
	for key := range sg.sources {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	srcs := make([]*layoutSource, len(keys))
	for i, k := range keys {
		srcs[i] = sg.sources[title(k)]
	}
	return srcs
}

type layoutApp struct {
	*deploy.Application

	title      title
	components map[title]*layoutAppComponent
}

type layoutAppComponent struct {
	*deploy.Application_Component

	proj  *layoutApp
	title title
}

type layoutDeployment struct {
	*deploy.Deployment

	l     *deployLayout
	title title

	// app is the application that this deployment is bound to.
	app *layoutApp
	// sg is the source group that this deployment is bound to.
	sg *layoutSourceGroup

	// components is the set of deployed application components.
	components map[title]*layoutDeploymentComponent
	// componentNames is a list of keys in the components map, ordered by the
	// order in which these components appeared in the protobuf.
	componentNames []title
	// cloudProject is the deployment cloud project, if one is specified.
	cloudProject *layoutDeploymentCloudProject
}

func (d *layoutDeployment) String() string { return string(d.title) }

// substituteParams applies parameter substitution to the supplied string in a
// left-to-right manner.
func (d *layoutDeployment) substituteParams(vp *string) error {
	if err := substitute(vp, d.Parameter); err != nil {
		return errors.Annotate(err).Err()
	}
	return nil
}

type layoutDeploymentComponent struct {
	deploy.Component

	// reldir is the source-relative directory that relative paths in this
	// component will reference. This will be the parent directory of the
	// component's configuration file.
	reldir string

	// dep is the deployment that this component belongs to.
	dep *layoutDeployment
	// comp is the Component definition.
	comp *layoutAppComponent
	// sources is the set of sources required to build this Component. This will
	// always have at least one entry: the source where this Component is located.
	sources []*layoutSource

	// buildDirs is a map of build directory keys to their on-disk paths generated
	// for them. This is populated during `buildComponent()`.
	buildDirs map[string]string

	// buildPathMap is a map of the resolved on-disk paths for a given BuildPath.
	buildPathMap map[*deploy.BuildPath]string

	// gkePod is the set of cluster-bound GKE pods that this component is deployed
	// to.
	gkePod *layoutDeploymentGKEPod

	// gkePods is the set of pod/cluster bindings for this Component's pod.
	gkePods []*layoutDeploymentGKEPodBinding
}

func (comp *layoutDeploymentComponent) String() string {
	return joinPath(comp.dep.title, comp.comp.title)
}

func (comp *layoutDeploymentComponent) source() *layoutSource {
	return comp.sources[0]
}

func (comp *layoutDeploymentComponent) pathTo(relpath string) string {
	return comp.source().pathTo(relpath, comp.reldir)
}

func (comp *layoutDeploymentComponent) buildPath(bp *deploy.BuildPath) (string, error) {
	if path, ok := comp.buildPathMap[bp]; ok {
		return path, nil
	}
	return "", errors.Reason("no build path resolved for: %(bp)+v").D("bp", bp, "%+v").Err()
}

func (comp *layoutDeploymentComponent) loadSourceComponent(reg componentRegistrar) error {
	if err := unmarshalTextProtobuf(comp.source().pathTo(comp.comp.Path, ""), &comp.Component); err != nil {
		return errors.Annotate(err).Reason("failed to load source component %(component)q").D("component", comp).Err()
	}

	// Referenced build paths.
	for i, p := range comp.BuildPath {
		var msg deploy.Component_Build
		if err := unmarshalTextProtobuf(comp.pathTo(p), &msg); err != nil {
			return errors.Annotate(err).Reason("failed to load component Build #%(index)d from [%(path)s]").
				D("index", i).D("path", p).Err()
		}
		comp.Build = append(comp.Build, &msg)
	}

	// Load any referenced data and normalize it for internal usage.
	dep := comp.dep
	switch t := comp.GetComponent().(type) {
	case *deploy.Component_AppengineModule:
		// Normalize AppEngine module:
		// - Modules explicitly named "default" will have their ModuleName changed
		//   to the empty string.
		// - All referenced path parameters will be loaded and appended onto their
		//   non-path members.
		if dep.cloudProject == nil {
			return errors.Reason("AppEngine module %(comp)q requires a cloud project").
				D("comp", comp).Err()
		}

		aem := t.AppengineModule
		if aem.ModuleName == "default" {
			aem.ModuleName = ""
		}

		module := layoutDeploymentGAEModule{
			AppEngineModule: aem,
			comp:            comp,
		}

		// Referenced handler paths.
		for i, p := range aem.HandlerPath {
			var msg deploy.AppEngineModule_HandlerSet
			if err := unmarshalTextProtobuf(comp.pathTo(p), &msg); err != nil {
				return errors.Annotate(err).Reason("failed to load HandlerSet #%(index)d for %(component)q").
					D("index", i).D("component", comp).Err()
			}
			module.Handlers.Handler = append(module.Handlers.Handler, msg.Handler...)
		}

		// Append GAE Resources.
		if r := module.Resources; r != nil {
			dep.cloudProject.appendResources(r, &module)
		}

		for i, p := range module.ResourcePath {
			if err := comp.dep.substituteParams(&p); err != nil {
				return errors.Annotate(err).Reason("failed to substitute parameters for resource path").
					D("path", p).Err()
			}

			var res deploy.AppEngineResources
			if err := unmarshalTextProtobuf(comp.pathTo(p), &res); err != nil {
				return errors.Annotate(err).Reason("failed to load Resources #%(index)d for %(component)").
					D("index", i).D("path", p).D("component", comp).Err()
			}
			dep.cloudProject.appendResources(&res, &module)
		}

		// Add this module to our cloud project's AppEngine modules list.
		dep.cloudProject.appEngineModules = append(dep.cloudProject.appEngineModules, &module)
		if reg != nil {
			reg.addGAEModule(&module)
		}

	case *deploy.Component_GkePod:
		if len(comp.gkePods) == 0 {
			return errors.Reason("GKE Container %(comp)q is not bound to a GKE cluster").
				D("comp", comp.String()).Err()
		}

		comp.gkePod = &layoutDeploymentGKEPod{
			ContainerEnginePod: t.GkePod,
			comp:               comp,
		}

		// None of the labels may use our "deploytool" prefix.
		var invalidLabels []string
		for k := range comp.gkePod.KubePod.Labels {
			if isKubeDeployToolKey(k) {
				invalidLabels = append(invalidLabels, k)
			}
		}
		if len(invalidLabels) > 0 {
			sort.Strings(invalidLabels)
			return errors.Reason("user-supplied labels may not use deploytool prefix").
				D("labels", invalidLabels).Err()
		}

		for _, bp := range comp.gkePods {
			bp.pod = comp.gkePod
			if reg != nil {
				reg.addGKEPod(bp)
			}
		}
	}

	return nil
}

// expandPaths iterates through the Component-defined directory fields and
// expands their paths into actual filesystem paths.
//
// This must be performed after the build instructions have been executed so
// that the "build_dir" map will be available if needed by a Component.
func (comp *layoutDeploymentComponent) expandPaths() error {
	resolveBuildPath := func(bp *deploy.BuildPath) error {
		if _, ok := comp.buildPathMap[bp]; ok {
			// Already resolved.
			return nil
		}

		var resolved string
		if bp.DirKey != "" {
			dir, ok := comp.buildDirs[bp.DirKey]
			if !ok {
				return errors.Reason("Invalid `dir_key` value: %(dirKey)q").D("dirKey", bp.DirKey).Err()
			}
			resolved = deployToNative(dir, bp.Path)
		} else {
			resolved = comp.pathTo(bp.Path)
		}

		if comp.buildPathMap == nil {
			comp.buildPathMap = make(map[*deploy.BuildPath]string)
		}
		comp.buildPathMap[bp] = resolved
		return nil
	}

	switch t := comp.Component.Component.(type) {
	case *deploy.Component_AppengineModule:
		aem := t.AppengineModule
		if aem.Handlers != nil {
			for _, handler := range aem.Handlers.Handler {
				switch c := handler.Content.(type) {
				case *deploy.AppEngineModule_Handler_StaticFiles_:
					sf := c.StaticFiles
					switch bd := sf.BaseDir.(type) {
					case *deploy.AppEngineModule_Handler_StaticFiles_Path:
						// Normalize our static files directory to a BuildPath rooted at our
						// source.
						sf.BaseDir = &deploy.AppEngineModule_Handler_StaticFiles_Build{
							Build: &deploy.BuildPath{Path: bd.Path},
						}
						if err := resolveBuildPath(sf.GetBuild()); err != nil {
							return errors.Annotate(err).Err()
						}

					case *deploy.AppEngineModule_Handler_StaticFiles_Build:
						if err := resolveBuildPath(bd.Build); err != nil {
							return errors.Annotate(err).Err()
						}

					default:
						return errors.Reason("unknown `base_dir` type %(type)T").D("type", bd).Err()
					}

				case *deploy.AppEngineModule_Handler_StaticDir:
					// Normalize our static directory (source-relative) to a BuildPath
					// rooted at our source.
					handler.Content = &deploy.AppEngineModule_Handler_StaticBuildDir{
						StaticBuildDir: &deploy.BuildPath{Path: c.StaticDir},
					}
					if err := resolveBuildPath(handler.GetStaticBuildDir()); err != nil {
						return errors.Annotate(err).Err()
					}

				case *deploy.AppEngineModule_Handler_StaticBuildDir:
					if err := resolveBuildPath(c.StaticBuildDir); err != nil {
						return errors.Annotate(err).Err()
					}
				}
			}
		}

	case *deploy.Component_GkePod:
		for _, container := range t.GkePod.KubePod.Container {
			switch df := container.Dockerfile.(type) {
			case *deploy.KubernetesPod_Container_Path:
				// Convert to BuildPath.
				container.Dockerfile = &deploy.KubernetesPod_Container_Build{
					Build: &deploy.BuildPath{Path: df.Path},
				}
				if err := resolveBuildPath(container.GetBuild()); err != nil {
					return errors.Annotate(err).Err()
				}

			case *deploy.KubernetesPod_Container_Build:
				if err := resolveBuildPath(df.Build); err != nil {
					return errors.Annotate(err).Err()
				}

			default:
				return errors.Reason("unknown `dockerfile` type %(type)T").D("type", df).Err()
			}
		}
	}

	return nil
}

// layoutDeploymentCloudProject tracks a cloud project element, as well as
// any cloud project configurations referenced by this Deployment's Components.
type layoutDeploymentCloudProject struct {
	*deploy.Deployment_CloudProject

	// dep is the Deployment that owns this cloud project.
	dep *layoutDeployment
	// gkeCluster is a map of GKE cluster names to their definitions.
	gkeClusters map[string]*layoutDeploymentGKECluster
	// appEngineModules is the set of AppEngine modules in this Deployment that
	// reference this cloud project.
	appEngineModules []*layoutDeploymentGAEModule

	// resources is the accumulated set of AppEngine Resources, loaded from
	// component and deployment source configs.
	resources deploy.AppEngineResources
}

func (cp *layoutDeploymentCloudProject) String() string {
	return joinPath(title(cp.dep.String()), title(cp.Name))
}

func (cp *layoutDeploymentCloudProject) appendResources(res *deploy.AppEngineResources,
	module *layoutDeploymentGAEModule) {

	// Accumulate global (non-module-bound) resources in our cloud project's
	// resources protobuf.
	cp.resources.Index = append(cp.resources.Index, res.Index...)

	// Accumulate module-specific resources in our module's resources protobuf.
	module.resources.Dispatch = append(module.resources.Dispatch, res.Dispatch...)
	module.resources.TaskQueue = append(module.resources.TaskQueue, res.TaskQueue...)
	module.resources.Cron = append(module.resources.Cron, res.Cron...)
}

// layoutDeploymentGAEModule is a single configured AppEngine module.
type layoutDeploymentGAEModule struct {
	*deploy.AppEngineModule

	// comp is the component that describes this module.
	comp *layoutDeploymentComponent

	// resources is the set of module-bound resources.
	resources deploy.AppEngineResources
}

// layoutDeploymentGKECluster tracks a GKE cluster element.
type layoutDeploymentGKECluster struct {
	*deploy.Deployment_CloudProject_GKECluster

	// cloudProject is the cloud project that this GKE cluster belongs to.
	cloudProject *layoutDeploymentCloudProject
	// pods is the set of GKE pods in this Deployment.
	pods []*layoutDeploymentGKEPodBinding
}

func (c *layoutDeploymentGKECluster) String() string {
	return joinPath(title(c.cloudProject.String()), title(c.Name))
}

// layoutDeploymentGKEPodBinding tracks a GKE pod bound to a GKE cluster.
type layoutDeploymentGKEPodBinding struct {
	*deploy.Deployment_CloudProject_GKECluster_PodBinding

	// cluster is the cluster that the pod is deployed to.
	cluster *layoutDeploymentGKECluster
	// pod is the pod that is deployed. This is filled in when project contents
	// are loaded.
	pod *layoutDeploymentGKEPod
}

func (pb *layoutDeploymentGKEPodBinding) String() string {
	return joinPath(title(pb.pod.String()), title(pb.cluster.String()))
}

// layoutDeploymentGKEPod tracks a deployed pod component bound to a GKE
// cluster.
type layoutDeploymentGKEPod struct {
	// The container engine pod definition. This won't be filled in until the
	// deployment's source configuration is loaded in loadSourceComponent.
	*deploy.ContainerEnginePod

	// comp is the component that this pod was described by.
	comp *layoutDeploymentComponent
}

func (pb *layoutDeploymentGKEPod) String() string { return pb.comp.String() }

// deployLayout is the loaded and configured layout, populated with any state
// that has been loaded during operation.
type deployLayout struct {
	deploy.Layout

	// base is the base path of the layout file. By default, all other sources
	// will be relative to this path.
	basePath string

	// user is the user configuration protobuf.
	user deploy.UserConfig
	// userSourceOverrides is a map of user checkout URL overrides.
	userSourceOverrides map[string]*deploy.Source

	// sourceGroups is the set of source group files, mapped to their source group
	// name.
	sourceGroups map[title]*layoutSourceGroup
	// apps is the set of applications, mapped to their application name.
	apps map[title]*layoutApp

	// deployments is the set of deployments, mapped to their deployment name.
	deployments map[title]*layoutDeployment
	// deploymentNames is a list of keys in the deployments map, ordered by the
	// order in which the deployments were loaded.
	deploymentNames []title
	// map of cloud project names to their deployments.
	cloudProjects map[string]*layoutDeploymentCloudProject
}

// workingFilesystem creates a new managed filesystem at our working directory
// root.
//
// This adds layout-defined components to the filesystem.
func (l *deployLayout) workingFilesystem() (*managedfs.Filesystem, error) {
	fs, err := managedfs.New(l.WorkingPath)
	if err != nil {
		return nil, errors.Annotate(err).Err()
	}
	return fs, nil
}

func (l *deployLayout) workingPathTo(relpath string) string {
	return filepath.Join(l.WorkingPath, relpath)
}

func (l *deployLayout) getDeploymentComponent(v string) (*layoutDeployment, *layoutDeploymentComponent, error) {
	deployment, component := splitComponentPath(v)

	dep := l.deployments[deployment]
	if dep == nil {
		return nil, nil, errors.Reason("unknown Deployment %(dep)q").
			D("value", v).D("dep", deployment).Err()
	}

	// If a component was specified, only add that component.
	if component != "" {
		comp := dep.components[component]
		if comp == nil {
			return nil, nil, errors.Reason("unknown Deployment Component %(value)q").
				D("value", v).D("dep", deployment).D("comp", component).Err()
		}
		return dep, comp, nil
	}
	return dep, nil, nil
}

func (l *deployLayout) matchDeploymentComponent(m string, cb func(*layoutDeployment, *layoutDeploymentComponent)) error {
	for _, depName := range l.deploymentNames {
		dep := l.deployments[depName]

		matched, err := filepath.Match(m, dep.String())
		if err != nil {
			return errors.Annotate(err).Reason("failed to match %(pattern)q").D("pattern", m).Err()
		}
		if matched {
			// Matches entire deployment.
			cb(dep, nil)
			continue
		}

		// Try each of the deployment's components.
		for _, compName := range dep.componentNames {
			comp := dep.components[compName]

			matched, err := filepath.Match(m, comp.String())
			if err != nil {
				return errors.Annotate(err).Reason("failed to match %(pattern)q").D("pattern", m).Err()
			}
			if matched {
				cb(dep, comp)
			}
		}
	}

	return nil
}

func (l *deployLayout) load(c context.Context, path string) error {
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		path, err = findLayout(defaultLayoutFilename, wd)
		if err != nil {
			return err
		}
	}

	if err := unmarshalTextProtobuf(path, &l.Layout); err != nil {
		return err
	}

	// Load the user config, if available.
	if err := loadUserConfig(c, &l.user); err != nil {
		return errors.Annotate(err).Reason("failed to load user config").Err()
	}
	if len(l.user.SourceOverride) > 0 {
		l.userSourceOverrides = make(map[string]*deploy.Source, len(l.user.SourceOverride))
		for k, v := range l.user.SourceOverride {
			l.userSourceOverrides[k] = v
		}
	}

	// Populate with defaults.
	l.basePath = filepath.Dir(path)
	if l.SourcesPath == "" {
		l.SourcesPath = filepath.Join(l.basePath, "sources")
	}
	if l.ApplicationsPath == "" {
		l.ApplicationsPath = filepath.Join(l.basePath, "applications")
	}
	if l.DeploymentsPath == "" {
		l.DeploymentsPath = filepath.Join(l.basePath, "deployments")
	}

	if l.WorkingPath == "" {
		l.WorkingPath = filepath.Join(l.basePath, ".working")
	} else {
		if !filepath.IsAbs(l.WorkingPath) {
			l.WorkingPath = filepath.Join(l.basePath, l.WorkingPath)
		}
	}

	absWorkingPath, err := filepath.Abs(l.WorkingPath)
	if err != nil {
		return errors.Annotate(err).Reason("failed to resolve absolute path for %(path)q").
			D("path", l.WorkingPath).Err()
	}
	l.WorkingPath = absWorkingPath
	return nil
}

func (l *deployLayout) initFrozenCheckout(c context.Context) (*deploy.FrozenLayout, error) {
	fis, err := ioutil.ReadDir(l.SourcesPath)
	if err != nil {
		return nil, errors.Annotate(err).Reason("failed to read directory").Err()
	}

	// Build internal and frozen layout in parallel.
	var frozen deploy.FrozenLayout
	frozen.SourceGroup = make(map[string]*deploy.FrozenLayout_SourceGroup, len(fis))

	for _, fi := range fis {
		name := title(fi.Name())
		path := filepath.Join(l.SourcesPath, string(name))

		if !fi.IsDir() {
			log.Fields{
				"path": path,
			}.Warningf(c, "Skipping non-directory in source group directory.")
			continue
		}

		if err := name.validate(); err != nil {
			log.Fields{
				log.ErrorKey: err,
				"title":      name,
				"path":       path,
			}.Warningf(c, "Skipping invalid source group title.")
			continue
		}

		// Load the Sources.
		srcInfos, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fields{
				log.ErrorKey:  err,
				"sourceGroup": name,
				"path":        path,
			}.Warningf(c, "Could not read source group directory.")
			continue
		}

		sg := deploy.FrozenLayout_SourceGroup{
			Source: make(map[string]*deploy.FrozenLayout_Source, len(srcInfos)),
		}

		var srcBase deploy.Source
		err = unmarshalTextProtobufDir(path, srcInfos, &srcBase, func(name string) error {
			cpy := srcBase

			t, err := titleFromConfigPath(name)
			if err != nil {
				return errors.Annotate(err).Reason("invalid source title").Err()
			}

			src := deploy.FrozenLayout_Source{
				Source: &cpy,
			}
			sg.Source[string(t)] = &src
			return nil
		})
		if err != nil {
			return nil, errors.Annotate(err).Reason("failed to load source group %(name)q from [%(path)s]").
				D("name", name).D("path", path).Err()
		}

		frozen.SourceGroup[string(name)] = &sg
	}

	return &frozen, nil
}

func (l *deployLayout) loadFrozenLayout(c context.Context) error {
	// Load the frozen configuration file from disk.
	frozen, err := checkoutFrozen(l)
	if err != nil {
		return errors.Annotate(err).Reason("failed to load frozen checkout").Err()
	}

	// Load our source groups and sources.
	l.sourceGroups = make(map[title]*layoutSourceGroup, len(frozen.SourceGroup))
	for sgName, fsg := range frozen.SourceGroup {
		sg := layoutSourceGroup{
			FrozenLayout_SourceGroup: fsg,
			layout:  l,
			title:   title(sgName),
			sources: make(map[title]*layoutSource, len(fsg.Source)),
		}

		for srcName, fs := range fsg.Source {
			src := layoutSource{
				FrozenLayout_Source: fs,
				sg:                  &sg,
				title:               title(srcName),
			}
			sg.sources[src.title] = &src
		}

		l.sourceGroups[sg.title] = &sg
	}

	// Build our internally-connected structures from the frozen layout.
	if err := l.loadApps(c); err != nil {
		return errors.Annotate(err).Reason("failed to load applications from [%(path)s]").
			D("path", l.ApplicationsPath).Err()
	}
	if err := l.loadDeployments(c); err != nil {
		return errors.Annotate(err).Reason("failed to load deployments from [%(path)s]").
			D("path", l.DeploymentsPath).Err()
	}
	return nil
}

func (l *deployLayout) loadApps(c context.Context) error {
	fis, err := ioutil.ReadDir(l.ApplicationsPath)
	if err != nil {
		return errors.Annotate(err).Reason("failed to read directory").Err()
	}
	apps := make(map[title]*deploy.Application, len(fis))
	var appBase deploy.Application
	err = unmarshalTextProtobufDir(l.ApplicationsPath, fis, &appBase, func(name string) error {
		cpy := appBase

		t, err := titleFromConfigPath(name)
		if err != nil {
			return errors.Annotate(err).Reason("invalid application title").Err()
		}

		apps[t] = &cpy
		return nil
	})
	if err != nil {
		return err
	}

	l.apps = make(map[title]*layoutApp, len(apps))
	for t, app := range apps {
		proj := layoutApp{
			Application: app,
			title:       t,
			components:  make(map[title]*layoutAppComponent, len(app.Component)),
		}

		// Initialize components. These can't actually be populated until we have
		// loaded our sources.
		for _, comp := range proj.Component {
			compT := title(comp.Name)
			if err := compT.validate(); err != nil {
				return errors.Annotate(err).Reason("application %(app)q component %(component)q is not a valid component title").
					D("app", t).D("component", comp.Name).Err()
			}
			proj.components[compT] = &layoutAppComponent{
				Application_Component: comp,
				proj:  &proj,
				title: compT,
			}
		}

		l.apps[t] = &proj
	}
	return nil
}

func (l *deployLayout) loadDeployments(c context.Context) error {
	fis, err := ioutil.ReadDir(l.DeploymentsPath)
	if err != nil {
		return errors.Annotate(err).Reason("failed to read directory").Err()
	}
	deployments := make(map[title]*deploy.Deployment, len(fis))
	var deploymentBase deploy.Deployment
	err = unmarshalTextProtobufDir(l.DeploymentsPath, fis, &deploymentBase, func(name string) error {
		cpy := deploymentBase

		t, err := titleFromConfigPath(name)
		if err != nil {
			return errors.Annotate(err).Reason("invalid deployment title").Err()
		}

		deployments[t] = &cpy
		return nil
	})
	if err != nil {
		return err
	}

	l.deployments = make(map[title]*layoutDeployment, len(deployments))
	for t, d := range deployments {
		dep, err := l.loadDeployment(t, d)
		if err != nil {
			return errors.Annotate(err).Reason("failed to load deployment (%(deployment)q)").
				D("deployment", t).Err()
		}
		l.deployments[t] = dep
		l.deploymentNames = append(l.deploymentNames, dep.title)
	}

	return nil
}

func (l *deployLayout) loadDeployment(t title, d *deploy.Deployment) (*layoutDeployment, error) {
	dep := layoutDeployment{
		Deployment: d,
		l:          l,
		title:      t,
		sg:         l.sourceGroups[title(d.SourceGroup)],
	}
	if dep.sg == nil {
		return nil, errors.Reason("unknown source group %(sourceGroup)q").
			D("sourceGroup", d.SourceGroup).Err()
	}

	// Resolve our application.
	dep.app = l.apps[title(dep.Application)]
	if dep.app == nil {
		return nil, errors.Reason("unknown application %(app)q").D("app", dep.Application).Err()
	}

	// Initialize our Components. Their protobufs cannot not be loaded until
	// we have a checkout.
	dep.components = make(map[title]*layoutDeploymentComponent, len(dep.app.components))
	for compTitle, projComp := range dep.app.components {
		comp := layoutDeploymentComponent{
			reldir:  deployDirname(projComp.Path),
			dep:     &dep,
			comp:    projComp,
			sources: []*layoutSource{dep.sg.sources[title(projComp.Source)]},
		}
		if comp.sources[0] == nil {
			return nil, errors.Reason("application references non-existent source %(source)q").
				D("source", projComp.Source).Err()
		}
		for _, os := range projComp.OtherSource {
			src := dep.sg.sources[title(os)]
			if src == nil {
				return nil, errors.Reason("application references non-existent other source %(source)q").
					D("source", os).Err()
			}
			comp.sources = append(comp.sources, src)
		}

		dep.components[compTitle] = &comp
		dep.componentNames = append(dep.componentNames, compTitle)
	}

	// Build a map of cloud project names.
	if dep.CloudProject != nil {
		cp := layoutDeploymentCloudProject{
			Deployment_CloudProject: dep.CloudProject,
			dep: &dep,
		}
		if l.cloudProjects == nil {
			l.cloudProjects = make(map[string]*layoutDeploymentCloudProject)
		}
		if cur, ok := l.cloudProjects[cp.Name]; ok {
			return nil, errors.Reason("cloud project %(name)q defined by both %(curDep)q and %(thisDep)q").
				D("name", cp.Name).D("curDep", cur.dep.title).D("thisDep", dep.title).Err()
		}
		l.cloudProjects[cp.Name] = &cp

		if len(dep.CloudProject.GkeCluster) > 0 {
			cp.gkeClusters = make(map[string]*layoutDeploymentGKECluster, len(dep.CloudProject.GkeCluster))
			for _, gke := range dep.CloudProject.GkeCluster {
				gkeCluster := layoutDeploymentGKECluster{
					Deployment_CloudProject_GKECluster: gke,
					cloudProject:                       &cp,
				}

				// Bind Components to their GKE cluster.
				for _, b := range gke.Pod {
					comp := dep.components[title(b.Name)]
					switch {
					case comp == nil:
						return nil, errors.Reason("unknown component %(comp)q for cluster %(name)q").
							D("comp", b.Name).D("name", gke.Name).Err()

					case b.Replicas <= 0:
						return nil, errors.Reason("GKE component %(comp)q must have at least 1 replica").
							D("comp", b.Name).Err()
					}

					bp := &layoutDeploymentGKEPodBinding{
						Deployment_CloudProject_GKECluster_PodBinding: b,
						cluster: &gkeCluster,
					}
					comp.gkePods = append(comp.gkePods, bp)
					gkeCluster.pods = append(gkeCluster.pods, bp)
				}

				cp.gkeClusters[gke.Name] = &gkeCluster
			}
		}

		dep.cloudProject = &cp
	}

	return &dep, nil
}

// allSourceGroups returns a sorted list of all of the source group names
// in the layout.
func (l *deployLayout) allSourceGroups() []*layoutSourceGroup {
	titles := make([]string, 0, len(l.sourceGroups))
	for t := range l.sourceGroups {
		titles = append(titles, string(t))
	}
	sort.Strings(titles)

	result := make([]*layoutSourceGroup, len(titles))
	for i, t := range titles {
		result[i] = l.sourceGroups[title(t)]
	}
	return result
}

// findLayout looks in the current working directory and ascends towards the
// root filesystem looking for a "layout.cfg" file.
func findLayout(filename, dir string) (string, error) {
	for {
		path := filepath.Join(dir, filename)

		switch st, err := os.Stat(path); {
		case err == nil:
			if !st.IsDir() {
				return path, nil
			}

		case isNotExist(err):
			break

		default:
			return "", errors.Annotate(err).Reason("failed to state %(path)q").
				D("path", path).Err()
		}

		// Walk up one directory.
		oldDir := dir
		dir, _ = filepath.Split(dir)
		if oldDir == dir {
			return "", errors.Reason("could not find %(filename)q starting from %(dir)q").
				D("filename", filename).D("dir", dir).Err()
		}
	}
}

type componentRegistrar interface {
	addGAEModule(*layoutDeploymentGAEModule)
	addGKEPod(*layoutDeploymentGKEPodBinding)
}
