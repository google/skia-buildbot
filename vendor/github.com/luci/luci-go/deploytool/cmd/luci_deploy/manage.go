// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/api/deploy"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
)

var cmdManage = subcommands.Command{
	UsageLine: "manage [DEPLOYMENT.[.COMPONENT]]... -help|SUBCOMMANDS...",
	ShortDesc: "Management subcommands for deployed components.",
	LongDesc:  "Offers subcommands to manage a Deployment or Component.",
	CommandRun: func() subcommands.CommandRun {
		return &cmdManageRun{}
	},
}

type cmdManageRun struct {
	subcommands.CommandRunBase
}

func (cmd *cmdManageRun) Run(app subcommands.Application, args []string) int {
	a, c := app.(*application), cli.GetContext(app, cmd)

	if len(args) == 0 {
		log.Errorf(c, "Must supply a Deployment or Component to operate on.")
		return 1
	}

	// Load our frozen checkout.
	if err := a.layout.loadFrozenLayout(c); err != nil {
		logError(c, err, "Failed to load frozen checkout")
		return 1
	}

	// Resolve to the specified Deployment or Component.
	mApp, err := getManager(c, a, args[0])
	if err != nil {
		logError(c, err, "Failed to resolve management target.")
		return 1
	}
	if mApp == nil {
		fmt.Printf("no management options available for %q", args[0])
		return 0
	}
	return subcommands.Run(mApp, args[1:])
}

type manageApp struct {
	*application

	subcommands []*subcommands.Command
	dp          deploymentPlan
}

func (a *manageApp) GetName() string                     { return a.application.GetName() + " manage" }
func (a *manageApp) GetCommands() []*subcommands.Command { return a.subcommands }

func getManager(c context.Context, a *application, name string) (*manageApp, error) {
	mApp := manageApp{
		application: a,
		subcommands: []*subcommands.Command{
			subcommands.CmdHelp,
		},
	}

	switch dep, comp, err := a.layout.getDeploymentComponent(name); {
	case err != nil:
		return nil, err

	case comp != nil:
		mApp.dp.addProjectComponent(comp)
		err := a.runWork(c, func(w *work) error {
			return mApp.dp.initialize(w, &a.layout)
		})
		if err != nil {
			return nil, errors.Annotate(err).Reason("failed to initialize deployment plan").Err()
		}

		switch comp.GetComponent().(type) {
		case *deploy.Component_AppengineModule:
			break
		case *deploy.Component_GkePod:
			mApp.subcommands = append(mApp.subcommands, []*subcommands.Command{
				{
					UsageLine: "kubectl <args...>",
					ShortDesc: "run a kubectl command",
					LongDesc:  "Run a kubectl command in this component's Container Engine context.",
					CommandRun: func() subcommands.CommandRun {
						cr := manageGKEPodKubectlCommandRun{
							comp: comp,
						}
						cr.GetFlags().StringVar(&cr.cluster, "cluster", "",
							"The cluster to operate on. If only one cluster is bound, this can be left blank.")
						return &cr
					},
				},
			}...)
		default:
			break
		}

	default:
		mApp.dp.addDeployment(dep)
		err := a.runWork(c, func(w *work) error {
			return mApp.dp.initialize(w, &a.layout)
		})
		if err != nil {
			return nil, errors.Annotate(err).Reason("failed to initialize deployment plan").Err()
		}

		if cp := dep.cloudProject; cp != nil {
			mApp.subcommands = append(mApp.subcommands, &subcommands.Command{
				UsageLine: "update_appengine",
				ShortDesc: "update AppEngine parameters",
				LongDesc:  "Update AppEngine cron, index, and queue configurations.",
				CommandRun: func() subcommands.CommandRun {
					return &updateGAECommandRun{
						project: cp,
					}
				},
			})
		}
		break
	}

	return &mApp, nil
}

type manageGKEPodKubectlCommandRun struct {
	subcommands.CommandRunBase

	comp    *layoutDeploymentComponent
	cluster string
}

func (cmd *manageGKEPodKubectlCommandRun) Run(app subcommands.Application, args []string) int {
	a, c := app.(*manageApp), cli.GetContext(app, cmd)

	// Figure out which cluster to use.
	var bp *layoutDeploymentGKEPodBinding
	switch {
	case cmd.cluster != "":
		cluster := cmd.comp.dep.cloudProject.gkeClusters[cmd.cluster]
		if cluster == nil {
			log.Errorf(c, "Invalid GKE cluster name %q.", cmd.cluster)
			return 1
		}
		for _, gkeBP := range cmd.comp.gkePods {
			if gkeBP.cluster == cluster {
				bp = gkeBP
				break
			}
		}

	case len(cmd.comp.gkePods) == 0:
		log.Errorf(c, "Pod is not bound to any clusters.")
		return 1

	case len(cmd.comp.gkePods) == 1:
		bp = cmd.comp.gkePods[0]

	default:
		clusters := make([]string, len(cmd.comp.gkePods))
		for i, cluster := range cmd.comp.gkePods {
			clusters[i] = fmt.Sprintf("- %s", cluster.cluster.Name)
		}

		log.Errorf(c, "Kubernetes pod is bound to multiple clusters. Specify one with -cluster:\n",
			strings.Join(clusters, "\n"))
		return 1
	}
	if bp == nil {
		log.Errorf(c, "Could not identify cluster for pod.")
		return 1
	}

	var rv int
	err := a.runWork(c, func(w *work) error {
		kubeCtx, err := getContainerEngineKubernetesContext(w, bp.cluster)
		if err != nil {
			return errors.Annotate(err).Err()
		}

		kubectl, err := w.tools.kubectl(kubeCtx)
		if err != nil {
			return errors.Annotate(err).Err()
		}

		rv, err = kubectl.exec(args...).forwardOutput().run(w)
		return err
	})
	if err != nil {
		logError(c, err, "Failed to run kubectl command.")
	}
	return rv
}

type updateGAECommandRun struct {
	subcommands.CommandRunBase

	project *layoutDeploymentCloudProject
}

func (cmd *updateGAECommandRun) Run(app subcommands.Application, args []string) int {
	a, c := app.(*manageApp), cli.GetContext(app, cmd)

	// Do not deploy any actual GAE modules.
	a.dp.reg.appEngineModulesOnly()
	for _, gae := range a.dp.reg.gaeProjects {
		gae.clearModules()
	}

	var rv int
	err := a.runWork(c, func(w *work) error {
		a.dp.params.stage = deployCommit
		return a.dp.deploy(w)
	})
	if err != nil {
		logError(c, err, "Failed to update GAE parameters.")
		return 1
	}
	return rv
}
