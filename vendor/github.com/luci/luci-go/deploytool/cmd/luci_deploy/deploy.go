// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"sort"

	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/flag/flagenum"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/managedfs"

	"github.com/maruel/subcommands"
)

type deployStage int

const (
	// deployStaging is the staging stage, where local filesystem and build
	// artifacts are constructed. Build scripts are executed, and generated files
	// and directories are created. This will leave the user with a directory that
	// can be used to build from.
	deployStaging deployStage = iota
	// deployLocalBuild performs a local build of the components, offering both a
	// sanity check prior to engaging remote services and creating build artifacts
	// ready to push remotely.
	deployLocalBuild deployStage = iota
	// deployPush pushes local build artifacts to the remote service, but doesn't
	// enable them.
	deployPush deployStage = iota
	// deployCommit commits the pushed artifacts to the remote service, activating
	// them.
	deployCommit deployStage = iota
)

var deployStageFlagEnum = flagenum.Enum{
	"stage":      deployStaging,
	"localbuild": deployLocalBuild,
	"push":       deployPush,
	"commit":     deployCommit,
}

func (s *deployStage) Set(v string) error { return deployStageFlagEnum.FlagSet(s, v) }
func (s deployStage) String() string      { return deployStageFlagEnum.FlagString(s) }

type deployParams struct {
	// stage is the highest deployment stage to deploy through.
	stage deployStage

	// ignoreCurrentVersion, if true, instructs depoyment logic to proceed with
	// deployment even if the currently-deployed version matches the current
	// source vrsion.
	ignoreCurrentVersion bool
	// commitTainted, if true, instructs the deploy tool to perform commit
	// operations on tainted Components.
	commitTainted bool
}

var cmdDeploy = subcommands.Command{
	UsageLine: "deploy [DEPLOYMENT.[.COMPONENT]]...",
	ShortDesc: "Deploys some or all of a deployment.",
	LongDesc:  "Deploys a Deployment. If specific projects are named, only those projects will be deployed.",
	CommandRun: func() subcommands.CommandRun {
		var cmd cmdDeployRun
		cmd.dp.params.stage = deployCommit

		fs := cmd.GetFlags()
		fs.BoolVar(&cmd.checkout, "checkout", false,
			"Refresh the checkout prior to deployment.")
		fs.Var(&cmd.dp.params.stage, "stage",
			"Run the deployment sequence up through this stage ["+deployStageFlagEnum.Choices()+"].")
		fs.BoolVar(&cmd.dp.params.commitTainted, "commit-tainted", false,
			"Install new deployments even if their source is tained.")
		fs.BoolVar(&cmd.dp.params.ignoreCurrentVersion, "ignore-current-version", false,
			"Push/install new deployments even if the currently-deployed version would not change or could not be parsed.")
		return &cmd
	},
}

type cmdDeployRun struct {
	subcommands.CommandRunBase

	// checkout, if true, refreshes the checkout prior to deployment.
	checkout bool

	dp deploymentPlan
}

func (cmd *cmdDeployRun) Run(app subcommands.Application, args []string) int {
	a, c := app.(*application), cli.GetContext(app, cmd)

	err := a.runWork(c, func(w *work) error {
		// Perform our planned checkout.
		if cmd.checkout {
			// Perform a full checkout.
			if err := checkout(w, &a.layout, false); err != nil {
				return errors.Annotate(err).Reason("failed to checkout sources").Err()
			}
		}

		// Load our frozen checkout.
		if err := a.layout.loadFrozenLayout(w); err != nil {
			return errors.Annotate(err).Reason("failed to load frozen checkout").Err()
		}
		return nil
	})
	if err != nil {
		logError(c, err, "Failed to perform checkout.")
		return 1
	}

	// Figure out what we're going to deploy.
	for _, arg := range args {
		matched := false
		err := a.layout.matchDeploymentComponent(arg, func(dep *layoutDeployment, comp *layoutDeploymentComponent) {
			matched = true

			if comp != nil {
				cmd.dp.addProjectComponent(comp)
			} else {
				cmd.dp.addDeployment(dep)
			}
		})
		if err != nil {
			logError(c, err, "Failed during matching.")
			return 1
		}
		if !matched {
			log.Fields{
				"target": arg,
			}.Errorf(c, "Deploy target did not match any configured deployments or components.")
			return 1
		}
	}

	err = a.runWork(c, func(w *work) error {
		if err := cmd.dp.initialize(w, &a.layout); err != nil {
			return errors.Annotate(err).Reason("failed to initialize").Err()
		}

		return cmd.dp.deploy(w)
	})
	if err != nil {
		logError(c, err, "Failed to perform deployment.")
		return 1
	}
	return 0
}

type deploymentPlan struct {
	// comps maps DEPLOYMENT/PROJECT/COMPONENT to its specific deployable project
	// component.
	comps map[string]*layoutDeploymentComponent

	// deployments tracks which deployments we've specified.
	deployments map[title]struct{}

	// params are the deployment parameters to forward to deployment operations.
	params deployParams

	// reg is the registry of deployable Components.
	reg deploymentRegistry

	// layout is the deployment layout. It is initialized during "initialize".
	layout *deployLayout
}

func (dp *deploymentPlan) addProjectComponent(comp *layoutDeploymentComponent) {
	// Mark this specific component.
	if dp.comps == nil {
		dp.comps = make(map[string]*layoutDeploymentComponent)
	}
	dp.comps[comp.String()] = comp

	// Mark that we've used this deployment.
	if dp.deployments == nil {
		dp.deployments = make(map[title]struct{})
	}
	dp.deployments[comp.dep.title] = struct{}{}
}

func (dp *deploymentPlan) addDeployment(dep *layoutDeployment) {
	for _, comp := range dep.components {
		dp.addProjectComponent(comp)
	}
}

func (dp *deploymentPlan) deployedComponents() []string {
	comps := make([]string, 0, len(dp.comps))
	for path := range dp.comps {
		comps = append(comps, path)
	}
	sort.Strings(comps)
	return comps
}

func (dp *deploymentPlan) initialize(w *work, l *deployLayout) error {
	dp.layout = l

	// Load all deployable Components from checkout.
	for _, depName := range l.deploymentNames {
		dep := l.deployments[depName]

		if _, ok := dp.deployments[dep.title]; !ok {
			continue
		}

		// Load the application components. We will iterate through them in the
		// order in which they are defined in the Application's protobuf.
		for _, compName := range dep.componentNames {
			comp := dep.components[compName]

			// Only pass the registrar if this component is being deployed.
			var pReg componentRegistrar
			if _, ok := dp.comps[comp.String()]; ok {
				pReg = &dp.reg
			}
			if err := comp.loadSourceComponent(pReg); err != nil {
				return errors.Annotate(err).Reason("failed to load component %(comp)q").
					D("comp", comp.String()).Err()
			}
		}
		return nil
	}
	return nil
}

func (dp *deploymentPlan) deploy(w *work) error {
	// Create our working root and staging/build subdirectories.
	fs, err := dp.layout.workingFilesystem()
	if err != nil {
		return errors.Annotate(err).Reason("failed to create working directory").Err()
	}

	stagingDir, err := fs.Base().EnsureDirectory("staging")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create staging directory").Err()
	}

	// Stage: Staging
	if err := dp.reg.stage(w, stagingDir, &dp.params); err != nil {
		return errors.Annotate(err).Reason("failed to stage").Err()
	}
	if dp.params.stage <= deployStaging {
		return nil
	}

	// Stage: Local Build
	if err := dp.reg.localBuild(w); err != nil {
		return errors.Annotate(err).Reason("failed to perform local build").Err()
	}
	if dp.params.stage <= deployLocalBuild {
		return nil
	}

	// Stage: Push
	if err := dp.reg.push(w); err != nil {
		return errors.Annotate(err).Reason("failed to push components").Err()
	}
	if dp.params.stage <= deployPush {
		return nil
	}

	// Stage: Commit
	if err := dp.reg.commit(w); err != nil {
		return errors.Annotate(err).Reason("failed to commit").Err()
	}
	return nil
}

// deploymentRegistry is the registry of all prepared deployments.
//
// While deployment is specified and staged at a component level, actual
// deployment is allowed to happen at a practical level (e.g., a single
// cloud project's AppEngine configuration, a single Google Container Engine
// cluster, etc.).
//
// This will track individual components, as well as integrate them into larger
// action plans.
type deploymentRegistry struct {
	// components is a map of components that have been deployed.
	components map[string]*layoutDeploymentComponent
	// componentNames is the sorted list of component names in components.
	componentNames []string

	// gaeProjects maps Google AppEngine deployment state to specific GAE
	// projects.
	gaeProjects map[string]*gaeDeployment
	// gaeProjectNames is the sorted list of registered GAE project names.
	gaeProjectNames []string

	// gkeProjects is the set of Google Container Engine deployments for a given
	// cloud project.
	gkeProjects map[string]*containerEngineDeployment
	// gkeProjectNames is the sorted list of registered GKE project names.
	gkeProjectNames []string
}

func (reg *deploymentRegistry) addComponent(comp *layoutDeploymentComponent) {
	name := comp.comp.Name
	if _, has := reg.components[name]; has {
		return
	}

	if reg.components == nil {
		reg.components = make(map[string]*layoutDeploymentComponent)
	}
	reg.components[name] = comp
	reg.componentNames = append(reg.componentNames, name)
}

func (reg *deploymentRegistry) addGAEModule(module *layoutDeploymentGAEModule) {
	// Get/create the AppEngine project for this module.
	cloudProjectName := module.comp.dep.cloudProject.Name
	gaeD := reg.gaeProjects[cloudProjectName]
	if gaeD == nil {
		gaeD = makeGAEDeployment(module.comp.dep.cloudProject)

		if reg.gaeProjects == nil {
			reg.gaeProjects = make(map[string]*gaeDeployment)
		}
		reg.gaeProjects[cloudProjectName] = gaeD
		reg.gaeProjectNames = append(reg.gaeProjectNames, cloudProjectName)
	}

	gaeD.addModule(module)
	reg.addComponent(module.comp)
}

func (reg *deploymentRegistry) addGKEPod(pb *layoutDeploymentGKEPodBinding) {
	// Get/create the AppEngine project for this module.
	cloudProjectName := pb.cluster.cloudProject.Name
	gkeD := reg.gkeProjects[cloudProjectName]
	if gkeD == nil {
		gkeD = &containerEngineDeployment{
			project:  pb.cluster.cloudProject,
			clusters: make(map[string]*containerEngineDeploymentCluster),
		}

		if reg.gkeProjects == nil {
			reg.gkeProjects = make(map[string]*containerEngineDeployment)
		}
		reg.gkeProjects[cloudProjectName] = gkeD
		reg.gkeProjectNames = append(reg.gkeProjectNames, cloudProjectName)
	}

	// Register this pod in its cluster.
	gkeD.addCluster(pb.cluster).attachPod(pb)
}

// appEngineModulesOnly clears all deployment parameters that are not AppEngine
// modules.
func (reg *deploymentRegistry) appEngineModulesOnly() {
	reg.gkeProjects, reg.gkeProjectNames = nil, nil
}

// stage performs staging, preparing deployment structures on the local
// filesystem.
func (reg *deploymentRegistry) stage(w *work, stageDir *managedfs.Dir, params *deployParams) error {
	// Sort our name lists.
	sort.Strings(reg.componentNames)
	sort.Strings(reg.gaeProjectNames)
	sort.Strings(reg.gkeProjectNames)

	// All components can be independently staged in parallel.
	//
	// We will still enumerate over them in sorted order for determinism.
	return w.RunMulti(func(workC chan<- func() error) {
		// AppEngine Projects
		for _, name := range reg.gaeProjectNames {
			proj := reg.gaeProjects[name]
			workC <- func() error {
				depDir, err := stageDir.EnsureDirectory(string(proj.project.dep.title), "appengine")
				if err != nil {
					return errors.Annotate(err).Reason("failed to create deployment directory for %(deployment)q").
						D("deployment", proj.project.dep.title).Err()
				}
				return proj.stage(w, depDir, params)
			}
		}

		// Container Engine Projects
		for _, name := range reg.gkeProjectNames {
			proj := reg.gkeProjects[name]
			workC <- func() error {
				depDir, err := stageDir.EnsureDirectory(string(proj.project.dep.title), "container_engine")
				if err != nil {
					return errors.Annotate(err).Reason("failed to create deployment directory for %(deployment)q").
						D("deployment", proj.project.dep.title).Err()
				}
				return proj.stage(w, depDir, params)
			}
		}
	})
}

// build performs the build stage on staged components.
func (reg *deploymentRegistry) localBuild(w *work) error {
	// Enumerate over them in sorted order for determinism.
	return w.RunMulti(func(workC chan<- func() error) {
		// AppEngine Projects
		for _, name := range reg.gaeProjectNames {
			proj := reg.gaeProjects[name]
			workC <- func() error {
				return proj.localBuild(w)
			}
		}

		// Container Engine Projects
		for _, name := range reg.gkeProjectNames {
			proj := reg.gkeProjects[name]
			workC <- func() error {
				return proj.localBuild(w)
			}
		}
	})
}

// push builds and pushes the deployments to remote services, but does not
// commit to them.
func (reg *deploymentRegistry) push(w *work) error {
	// All components can be independently pushed in parallel.
	//
	// We will still enumerate over them in sorted order for determinism.
	return w.RunMulti(func(workC chan<- func() error) {
		// AppEngine Projects
		for _, name := range reg.gaeProjectNames {
			proj := reg.gaeProjects[name]
			workC <- func() error {
				return proj.push(w)
			}
		}

		// Container Engine Projects
		for _, name := range reg.gkeProjectNames {
			proj := reg.gkeProjects[name]
			workC <- func() error {
				return proj.push(w)
			}
		}
	})
}

// commit commits the deployment to the remote services.
func (reg *deploymentRegistry) commit(w *work) error {
	// All components can be independently committed in parallel.
	//
	// We will still enumerate over them in sorted order for determinism.
	return w.RunMulti(func(workC chan<- func() error) {
		// AppEngine Projects
		for _, name := range reg.gaeProjectNames {
			proj := reg.gaeProjects[name]
			workC <- func() error {
				return proj.commit(w)
			}
		}

		// Container Engine Projects
		for _, name := range reg.gkeProjectNames {
			proj := reg.gkeProjects[name]
			workC <- func() error {
				return proj.commit(w)
			}
		}
	})
}
