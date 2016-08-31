// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/api/deploy"
	"github.com/luci/luci-go/deploytool/managedfs"
)

// kubeDepoyedByMe is the "luci:managedBy" Kubernetes Deployment annotation
// indicating that a given Kubernetes Deployment is managed by this deployment
// tool.
const (
	kubeManagedByKey = "luci.managedBy"
	kubeManagedByMe  = "luci-deploytool"

	kubeDeployToolPrefix = "luci.deploytool/"
	kubeVersionKey       = kubeDeployToolPrefix + "version"
	kubeSourceVersionKey = kubeDeployToolPrefix + "sourceVersion"
)

func isKubeDeployToolKey(v string) bool { return strings.HasPrefix(v, kubeDeployToolPrefix) }

// containerEngineDeployment is a consolidated Google Container Engine
// deployment configuration. It includes staged configurations for specific
// components, as well as global Google Container Engine state for a single
// cloud project.
//
// A container engine deployment is made up of specific clusters within
// Container Engine to independently manage/deploy. Each of these clusters is
// further made up of the Kubernetes pods that are configured to be deployed to
// those clusters. Finally, each pod is broken into specific Kubernetes
// components (Docker images) that the pod is composed of.
//
// Pods/containers are specified as Project Components, and are staged in
// component subdirectories.
//
// Clusters are configured independently. During deployment, pods/containers
// are deployed to clusters.
type containerEngineDeployment struct {
	// project is the cloud project that this deployment is targeting.
	project *layoutDeploymentCloudProject

	// clusters is a map of the container engine clusters that have components
	// being deployed.
	clusters map[string]*containerEngineDeploymentCluster
	// clusterNames is the sorted list of cluster names. It is prepared during
	// staging.
	clusterNames []string

	// pods is the set of staged pods. Each pod corresponds to a single Project
	// Component. This is separate from Clusters, since a single Pod may be
	// deployed to multiple Clusters.
	//
	// There will be one Pod entry here per declaration, regardless of how many
	// times it's deployed.
	pods []*stagedGKEPod

	// podMap is a map of pods to the clusters that they are deployed to.
	podMap map[*layoutDeploymentGKEPod]*stagedGKEPod

	// ignoreCurrentVersion is true if deployment should generate and push a new
	// version even if its base parameters match the currently-deployed version.
	ignoreCurrentVersion bool

	// timestampSuffix is timestamp suffix appended to Docker image names. This is
	// used to differentiate Docker images from the same version.
	timestampSuffix string
}

func (d *containerEngineDeployment) addCluster(cluster *layoutDeploymentGKECluster) *containerEngineDeploymentCluster {
	if c := d.clusters[cluster.Name]; c != nil {
		return c
	}

	c := containerEngineDeploymentCluster{
		gke:     d,
		cluster: cluster,
	}
	d.clusters[cluster.Name] = &c
	d.clusterNames = append(d.clusterNames, cluster.Name)
	return &c
}

func (d *containerEngineDeployment) maybeRegisterPod(pod *layoutDeploymentGKEPod) *stagedGKEPod {
	// If this is the first time we've seen this pod, add it to our pods list.
	if sp := d.podMap[pod]; sp != nil {
		return sp
	}

	sp := &stagedGKEPod{
		ContainerEnginePod: pod.ContainerEnginePod,
		gke:                d,
		pod:                pod,
	}
	if d.podMap == nil {
		d.podMap = make(map[*layoutDeploymentGKEPod]*stagedGKEPod)
	}
	d.podMap[pod] = sp
	d.pods = append(d.pods, sp)
	return sp
}

func (d *containerEngineDeployment) stage(w *work, root *managedfs.Dir, params *deployParams) error {
	d.ignoreCurrentVersion = params.ignoreCurrentVersion

	// Build a common timestamp suffix for our Docker images.
	d.timestampSuffix = strconv.FormatInt(clock.Now(w).Unix(), 10)

	podRoot, err := root.EnsureDirectory("pods")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create pods directory").Err()
	}

	// Stage in parallel. We will stage all pods before we stage any containers,
	// as container staging requires some pod staging values to be populated.
	err = w.RunMulti(func(workC chan<- func() error) {
		// Check and get all Kubernetes contexts in series.
		//
		// These all share the same Kubernetes configuration file, so we don't want
		// them to stomp each other if we did them in parallel.
		workC <- func() error {
			for _, name := range d.clusterNames {
				cluster := d.clusters[name]

				var err error
				if cluster.kubeCtx, err = getContainerEngineKubernetesContext(w, cluster.cluster); err != nil {
					return errors.Annotate(err).Reason("failed to get Kubernetes context for %(cluster)q").
						D("cluster", cluster.cluster.Name).Err()
				}
			}
			return nil
		}

		for _, pod := range d.pods {
			pod := pod
			workC <- func() error {
				// Use the name of this Pod's Component for staging directory.
				name := pod.pod.comp.comp.Name
				podDir, err := podRoot.EnsureDirectory(name)
				if err != nil {
					return errors.Annotate(err).Reason("failed to create pod directory for %(pod)q").
						D("pod", name).Err()
				}

				return pod.stage(w, podDir)
			}
		}
	})
	if err != nil {
		return err
	}

	// Now that pods are deployed, deploy our clusters.
	clusterRoot, err := root.EnsureDirectory("clusters")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create clusters directory").Err()
	}

	return w.RunMulti(func(workC chan<- func() error) {
		// Stage each cluster and pod in parallel.
		for _, name := range d.clusterNames {
			cluster := d.clusters[name]

			workC <- func() error {
				clusterDir, err := clusterRoot.EnsureDirectory(cluster.cluster.Name)
				if err != nil {
					return errors.Annotate(err).Reason("failed to create cluster directory for %(cluster)q").
						D("cluster", cluster.cluster.Name).Err()
				}

				return cluster.stage(w, clusterDir)
			}
		}
	})
}

func (d *containerEngineDeployment) localBuild(w *work) error {
	return w.RunMulti(func(workC chan<- func() error) {
		for _, pod := range d.pods {
			pod := pod
			workC <- func() error {
				return pod.build(w)
			}
		}
	})
}

func (d *containerEngineDeployment) push(w *work) error {
	return w.RunMulti(func(workC chan<- func() error) {
		for _, pod := range d.pods {
			pod := pod
			workC <- func() error {
				return pod.push(w)
			}
		}
	})
}

func (d *containerEngineDeployment) commit(w *work) error {
	// Push all clusters in parallel.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, name := range d.clusterNames {
			cluster := d.clusters[name]
			workC <- func() error {
				return cluster.commit(w)
			}
		}
	})
}

// containerEngineDeploymentCluster is the deployment configuration for a single
// Google Compute Engine cluster. This includes the cluster's aggregate global
// configuration, as well as any staged pods that are being deployed to this
// cluster.
type containerEngineDeploymentCluster struct {
	// gke is the containerEngineDeployment that owns this cluster.
	gke *containerEngineDeployment

	// cluster is the underlying cluster configuration.
	cluster *layoutDeploymentGKECluster

	// pods is the sorted list of pod deployments in this cluster.
	pods []*containerEngineBoundPod

	// scopes is the set of all scopes across all pods registered to this cluster,
	// regardless of which are being deployed.
	scopes []string

	// kubeCtx is the name of the Kubernetes context, as defined/installed by
	// gcloud.
	kubeCtx string
}

func (c *containerEngineDeploymentCluster) attachPod(pod *layoutDeploymentGKEPodBinding) {
	c.pods = append(c.pods, &containerEngineBoundPod{
		sp:      c.gke.maybeRegisterPod(pod.pod),
		c:       c,
		binding: pod,
	})
}

func (c *containerEngineDeploymentCluster) stage(w *work, root *managedfs.Dir) error {
	// Determine which scopes this cluster will need. This is across ALL pods
	// registered with the cluster, not just deployed ones.
	scopeMap := make(map[string]struct{})
	for _, bp := range c.pods {
		for _, scope := range bp.sp.pod.Scopes {
			scopeMap[scope] = struct{}{}
		}
	}
	c.scopes = make([]string, 0, len(scopeMap))
	for scope := range scopeMap {
		c.scopes = append(c.scopes, scope)
	}
	sort.Strings(c.scopes)

	// Stage for each deploymend pod.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, bp := range c.pods {
			bp := bp
			workC <- func() error {
				stageDir, err := root.EnsureDirectory(string(bp.sp.pod.comp.comp.title))
				if err != nil {
					return errors.Annotate(err).Reason("failed to create staging directory").Err()
				}

				return bp.stage(w, stageDir)
			}
		}
	})
}

func (c *containerEngineDeploymentCluster) commit(w *work) error {
	// Push all pods in parallel.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, bp := range c.pods {
			bp := bp
			workC <- func() error {
				return bp.commit(w)
			}
		}
	})
}

func (c *containerEngineDeploymentCluster) kubectl(w *work) (*kubeTool, error) {
	return w.tools.kubectl(c.kubeCtx)
}

// containerEngineBoundPod is a single staged pod deployed to a
// specific GKE cluster.
type containerEngineBoundPod struct {
	// sp is the staged pod.
	sp *stagedGKEPod

	// cluster is the cluster that this pod is deployed to.
	c *containerEngineDeploymentCluster

	// binding is the binding between sp and c.
	binding *layoutDeploymentGKEPodBinding

	// deploymentYAMLPath is the filesystem path to the deployment YAML.
	deploymentYAMLPath string
}

func (bp *containerEngineBoundPod) stage(w *work, root *managedfs.Dir) error {
	comp := bp.sp.pod.comp

	// Build our pod-wide deployment YAML.
	// Generate our deployment YAML.
	depYAML := kubeBuildDeploymentYAML(bp.binding, bp.sp.deploymentName, bp.sp.imageMap)
	depYAML.Metadata.addAnnotation(kubeManagedByKey, kubeManagedByMe)
	depYAML.Metadata.addAnnotation(kubeVersionKey, bp.sp.version.String())
	depYAML.Metadata.addAnnotation(kubeSourceVersionKey, comp.source().Revision)
	depYAML.Spec.Template.Metadata.addLabel("luci/project", string(comp.comp.proj.title))
	depYAML.Spec.Template.Metadata.addLabel("luci/component", string(comp.comp.title))

	deploymentYAML := root.File("deployment.yaml")
	if err := deploymentYAML.GenerateYAML(w, depYAML); err != nil {
		return errors.Annotate(err).Reason("failed to generate deployment YAML").Err()
	}
	bp.deploymentYAMLPath = deploymentYAML.String()
	return nil
}

func (bp *containerEngineBoundPod) commit(w *work) error {
	kubectl, err := bp.c.kubectl(w)
	if err != nil {
		return errors.Annotate(err).Err()
	}

	// Get the current deployment status for this pod.
	var (
		kd             kubeDeployment
		currentVersion string
	)
	switch err := kubectl.getResource(w, fmt.Sprintf("deployments/%s", bp.sp.deploymentName), &kd); err {
	case nil:
		// Got deployment status.
		md := kd.Metadata
		if md == nil {
			return errors.Reason("current deployment has no metadata").Err()
		}

		// Make sure the current deployment is managed by this tool.
		v, ok := md.Annotations[kubeManagedByKey].(string)
		if !ok {
			return errors.Reason("missing '" + kubeManagedByKey + "' annotation").Err()
		}
		if v != kubeManagedByMe {
			log.Fields{
				"managedBy":  v,
				"deployment": bp.sp.deploymentName,
			}.Errorf(w, "Current deployment is not managed.")
			return errors.Reason("unknown manager %(managedBy)q").D("managedBy", v).Err()
		}

		// Is the current deployment tagged at the current version?
		currentVersion, ok = md.Annotations[kubeVersionKey].(string)
		if !ok {
			return errors.Reason("missing '" + kubeVersionKey + "' annotation").Err()
		}
		cloudVersion, err := parseCloudProjectVersion(bp.c.gke.project.VersionScheme, currentVersion)
		switch {
		case err != nil:
			if !bp.c.gke.ignoreCurrentVersion {
				return errors.Annotate(err).Reason("failed to parse current version %(version)q").
					D("version", currentVersion).Err()
			}

			log.Fields{
				log.ErrorKey:     err,
				"currentVersion": currentVersion,
			}.Warningf(w, "Could not parse current version, but configured to ignore this failure.")

		case cloudVersion.Equals(bp.sp.version):
			if !bp.c.gke.ignoreCurrentVersion {
				log.Fields{
					"version": currentVersion,
				}.Infof(w, "Deployed version matches deployment version; not committing.")
				return nil
			}

			log.Fields{
				"version": currentVersion,
			}.Infof(w, "Deployed version matches deployment version, but configured to deploy anyway.")
		}

		// fallthrough to "kubectl apply" the new configuration.
		fallthrough

	case errKubeResourceNotFound:
		// No current deployment, create a new one.
		log.Fields{
			"currentVersion": currentVersion,
			"deployVersion":  bp.sp.version,
		}.Infof(w, "Deploying new pod configuration.")
		if err := kubectl.exec("apply", "-f", bp.deploymentYAMLPath).check(w); err != nil {
			return errors.Annotate(err).Reason("failed to create new deployment configuration").Err()
		}
		return nil

	default:
		return errors.Annotate(err).Reason("failed to get status for deployment %(deployment)q").
			D("deployment", bp.sp.deploymentName).Err()
	}
}

// stagedGKEPod is staging information for a Google Container Engine deployed
// Kubernetes Pod.
type stagedGKEPod struct {
	*deploy.ContainerEnginePod

	// gke is the container engine deployment that owns this pod.
	gke *containerEngineDeployment
	// pod is the deployment pod that this is staging.
	pod *layoutDeploymentGKEPod

	// version is the calculated cloud project version.
	version *cloudProjectVersion
	// The name of the deployment for thie Component.
	deploymentName string
	// containers is the set of staged Kubernetes containers.
	containers []*stagedKubernetesContainer
	// goPath is the generate GOPATH for this container's sources.
	goPath []string

	// imageMap maps container names to their Docker image names.
	imageMap map[string]string
}

func (sp *stagedGKEPod) cloudProject() *layoutDeploymentCloudProject {
	return sp.pod.comp.dep.cloudProject
}

func (sp *stagedGKEPod) stage(w *work, root *managedfs.Dir) error {
	// Calculate the cloud project version for this pod.
	var err error
	sp.version, err = makeCloudProjectVersion(sp.cloudProject(), sp.pod.comp.source())
	if err != nil {
		return errors.Annotate(err).Reason("failed to get cloud version").Err()
	}

	comp := sp.pod.comp
	sp.deploymentName = fmt.Sprintf("%s--%s", comp.comp.proj.title, comp.comp.title)

	sp.imageMap = make(map[string]string, len(sp.KubePod.Container))
	sp.containers = make([]*stagedKubernetesContainer, len(sp.KubePod.Container))
	for i, kc := range sp.KubePod.Container {
		skc := stagedKubernetesContainer{
			KubernetesPod_Container: kc,
			pod: sp,
			image: fmt.Sprintf("gcr.io/%s/%s:%s-%s",
				sp.gke.project.Name, kc.Name, sp.version.String(), sp.gke.timestampSuffix),
		}

		sp.imageMap[kc.Name] = skc.image
		sp.containers[i] = &skc
	}

	// All files in this pod will share a GOPATH. Generate it, if any of our
	// containers use Go.
	needsGoPath := false
	for _, skc := range sp.containers {
		if skc.needsGoPath() {
			needsGoPath = true
			break
		}
	}
	if needsGoPath {
		// Build a GOPATH from our sources.
		// Construct a GOPATH for this module.
		goPath, err := root.EnsureDirectory("gopath")
		if err != nil {
			return errors.Annotate(err).Reason("failed to create GOPATH base").Err()
		}
		if err := stageGoPath(w, comp, goPath); err != nil {
			return errors.Annotate(err).Reason("failed to stage GOPATH").Err()
		}
		sp.goPath = []string{goPath.String()}
	}

	// Stage each of our containers.
	containersDir, err := root.EnsureDirectory("containers")
	if err != nil {
		return errors.Annotate(err).Err()
	}
	err = w.RunMulti(func(workC chan<- func() error) {
		// Stage each component.
		for _, skc := range sp.containers {
			skc := skc
			workC <- func() error {
				containerDir, err := containersDir.EnsureDirectory(skc.Name)
				if err != nil {
					return errors.Annotate(err).Err()
				}

				if err := skc.stage(w, containerDir); err != nil {
					return errors.Annotate(err).Reason("failed to stage container %(container)q").
						D("container", skc.Name).Err()
				}
				return nil
			}
		}
	})
	if err != nil {
		return err
	}

	if err := root.CleanUp(); err != nil {
		return errors.Annotate(err).Reason("failed to cleanup staging area").Err()
	}
	return nil
}

func (sp *stagedGKEPod) build(w *work) error {
	// Build any containers within this pod.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, cont := range sp.containers {
			workC <- func() error {
				return cont.build(w)
			}
		}
	})
}

func (sp *stagedGKEPod) push(w *work) error {
	// Build any containers within this pod.
	return w.RunMulti(func(workC chan<- func() error) {
		for _, cont := range sp.containers {
			workC <- func() error {
				return cont.push(w)
			}
		}
	})
}

// stagedKubernetesContainer is staging information for a single Kubernetes
// Container.
type stagedKubernetesContainer struct {
	*deploy.KubernetesPod_Container

	// pod is the pod that owns this container.
	pod *stagedGKEPod

	// image is the Docker image URI for this container.
	image string
	// remoteImageExists is true if the image already exists on the remote. This
	// is checked during the "build" phase.
	remoteImageExists bool

	buildFn func(*work) error
}

func (skc *stagedKubernetesContainer) needsGoPath() bool {
	switch skc.Type {
	case deploy.KubernetesPod_Container_GO:
		return true
	default:
		return false
	}
}

func (skc *stagedKubernetesContainer) stage(w *work, root *managedfs.Dir) error {
	// Build each Component.
	buildDir, err := root.EnsureDirectory("build")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create build directory").Err()
	}
	if err := buildComponent(w, skc.pod.pod.comp, buildDir); err != nil {
		return errors.Annotate(err).Reason("failed to build component").Err()
	}

	switch skc.Type {
	case deploy.KubernetesPod_Container_GO:
		// Specify how we are to be built.
		skc.buildFn = func(w *work) error {
			path, err := skc.pod.pod.comp.buildPath(skc.GetBuild())
			if err != nil {
				return errors.Annotate(err).Err()
			}
			return skc.buildGo(w, path)
		}

	default:
		return errors.Reason("unknown Kubernetes pod type %(type)T").D("type", skc.Type).Err()
	}
	return nil
}

func (skc *stagedKubernetesContainer) build(w *work) error {
	if f := skc.buildFn; f != nil {
		return f(w)
	}
	return nil
}

// build builds the image associated with this container.
func (skc *stagedKubernetesContainer) buildGo(w *work, entryPath string) error {
	gcloud, err := w.tools.gcloud(skc.pod.cloudProject().Name)
	if err != nil {
		return errors.Annotate(err).Reason("could not get gcloud tool").Err()
	}

	// Use "aedeploy" to gather GOPATH and build against our root.
	aedeploy, err := w.tools.aedeploy(skc.pod.goPath)
	if err != nil {
		return errors.Annotate(err).Err()
	}

	x := gcloud.exec("docker", "--", "build", "-t", skc.image, ".")
	return aedeploy.bootstrap(x).cwd(entryPath).check(w)
}

func (skc *stagedKubernetesContainer) push(w *work) error {
	gcloud, err := w.tools.gcloud(skc.pod.cloudProject().Name)
	if err != nil {
		return errors.Annotate(err).Reason("could not get gcloud tool").Err()
	}

	if err := gcloud.exec("docker", "--", "push", skc.image).check(w); err != nil {
		return errors.Annotate(err).Reason("failed to push Docker image %(image)q").
			D("image", skc.image).Err()
	}
	return nil
}

func getContainerEngineKubernetesContext(w *work, cluster *layoutDeploymentGKECluster) (
	string, error) {
	// Generate our Kubernetes context name. This is derived from the Google
	// Container Engine cluster parameters.
	kubeCtx := fmt.Sprintf("gke_%s_%s_%s", cluster.cloudProject.Name, cluster.Zone, cluster.Name)

	kubectl, err := w.tools.kubectl(kubeCtx)
	if err != nil {
		return "", errors.Annotate(err).Err()
	}

	// Check if the context is already installed in our Kubernetes configuration.
	switch has, err := kubectl.hasContext(w); {
	case err != nil:
		return "", errors.Annotate(err).Reason("failed to check for Kubernetes context").Err()

	case !has:
		gcloud, err := w.tools.gcloud(cluster.cloudProject.Name)
		if err != nil {
			return "", errors.Annotate(err).Err()
		}

		// The context isn't cached, we will fetch it via:
		// $ gcloud container clusters get-credentials
		x := gcloud.exec(
			"container", "clusters",
			"get-credentials", cluster.Name,
			"--zone", cluster.Zone)
		if err := x.check(w); err != nil {
			return "", errors.Annotate(err).Reason("failed to get cluster credentials").Err()
		}
		switch has, err = kubectl.hasContext(w); {
		case err != nil:
			return "", errors.Annotate(err).Reason("failed to confirm Kubernetes context").Err()
		case !has:
			return "", errors.Reason("context %(context)q missing after fetching credentials").D("context", kubeCtx).Err()
		}
	}
	return kubeCtx, nil
}
