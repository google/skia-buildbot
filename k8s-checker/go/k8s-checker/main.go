// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/kube/clusterconfig"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/k8s-checker/go/k8s_config"
)

const (
	appLabel         = "app"
	dirtyImageSuffix = "-dirty"
	namespaceDefault = "default"

	// Metric names.
	appRunningMetric                = "app_running_metric"
	containerRunningMetric          = "container_running_metric"
	dirtyCommittedImageMetric       = "dirty_committed_image_metric"
	dirtyConfigMetric               = "dirty_config_metric"
	ephemeralDiskRequestMetric      = "ephemeral_disk_requested"
	evictedPodMetric                = "evicted_pod_metric"
	livenessMetric                  = "k8s_checker"
	podMaxReadyTimeMetric           = "pod_max_ready_time_s"
	podReadyMetric                  = "pod_ready"
	podRestartCountMetric           = "pod_restart_count"
	podRunningMetric                = "pod_running"
	runningAppHasConfigMetric       = "running_app_has_config_metric"
	runningContainerHasConfigMetric = "running_container_has_config_metric"
	staleImageMetric                = "stale_image_metric"
	totalDiskRequestMetric          = "total_disk_requested"
)

// The format of the image is expected to be:
// "gcr.io/${PROJECT}/${APPNAME}:${DATETIME}-${USER}-${HASH:0:7}-${REPO_STATE}" (from bash/docker_build.sh).
var imageRegex = regexp.MustCompile(`^.+:(.+)-.+-.+-.+$`)

// allowedAppsInNamespace maps a namespace to a list of allowed applications in that namespace.
type allowedAppsInNamespace map[string][]string

func main() {
	// Flags.
	dirtyConfigChecksPeriod := flag.Duration("dirty_config_checks_period", 2*time.Minute, "How often to check for dirty configs/images in K8s.")
	configFile := flag.String("config_file", "", "The location of the config.json file that describes all the clusters.")
	cluster := flag.String("cluster", "skia-public", "The k8s cluster name.")
	promPort := flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	ignoreNamespaces := common.NewMultiStringFlag("ignore_namespace", nil, "Namespaces to ignore.")
	namespaceAllowFilter := common.NewMultiStringFlag("namespace_allow_filter", nil, "app names to ignore in a namespace. A namespace name, colon, list of comma separated app names. Ex: gmp-system:rule-evaluator,gmp-system:collector")

	common.InitWithMust("k8s_checker", common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	allowedAppsByNamespace, err := parseNamespaceAllowFilterFlag(*namespaceAllowFilter)
	if err != nil {
		sklog.Fatal("Failed to parse flag --namespace_allow_filter %s: %s", *namespaceAllowFilter, err)
	}
	sklog.Infof("allowedAppsByNamespace: %v", allowedAppsByNamespace)

	clusterConfig, err := clusterconfig.New(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to load cluster config: %s", err)
	}

	if _, ok := clusterConfig.Clusters[*cluster]; !ok {
		sklog.Fatalf("Invalid cluster %q: %s", *cluster, err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster config: %s", err)
	}
	sklog.Infof("Auth username: %s", config.Username)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster clientset: %s", err)
	}

	// OAuth2.0 TokenSource.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}
	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	liveness := metrics2.NewLiveness(livenessMetric)
	oldMetrics := map[metrics2.Int64Metric]struct{}{}
	go util.RepeatCtx(ctx, *dirtyConfigChecksPeriod, func(ctx context.Context) {
		newMetrics, err := performChecks(ctx, *cluster, clusterConfig.Repo, clientset, *ignoreNamespaces, gitiles.NewRepo(clusterConfig.Repo, httpClient), oldMetrics, allowedAppsByNamespace)
		if err != nil {
			sklog.Errorf("Error when checking for dirty configs: %s", err)
		} else {
			liveness.Reset()
			oldMetrics = newMetrics
		}
	})

	select {}
}

func parseNamespaceAllowFilterFlag(namespaceAllowFilter []string) (allowedAppsInNamespace, error) {
	ret := allowedAppsInNamespace{}

	for _, filter := range namespaceAllowFilter {
		parts := strings.SplitN(filter, ":", 2)
		if len(parts) != 2 {
			return nil, skerr.Fmt("Missing colon in: %q", filter)
		}
		ns := fixupNamespace(parts[0])
		app := parts[1]
		ret[ns] = append(ret[ns], app)
	}

	return ret, nil
}

// fixupNamespace sets the namespace to the default, if necessary.
func fixupNamespace(namespace string) string {
	if namespace == "" {
		return namespaceDefault
	}
	return namespace
}

// getNamespaces finds all namespaces in the cluster.
func getNamespaces(ctx context.Context, cluster string, clientset *kubernetes.Clientset, ignoreNamespaces []string) ([]string, error) {
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, skerr.Wrapf(err, "listing namespaces")
	}
	rv := make([]string, 0, len(namespaces.Items))
	for _, namespace := range namespaces.Items {
		if !util.In(namespace.Name, ignoreNamespaces) {
			rv = append(rv, namespace.Name)
		}
	}
	return rv, nil
}

// getEvictedPods finds all pods in "Evicted" state and reports metrics.
// It puts all reported evictedMetrics into the specified metrics map.
func getEvictedPods(ctx context.Context, cluster, namespace string, clientset *kubernetes.Clientset, metrics map[metrics2.Int64Metric]struct{}) error {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Failed",
	})
	if err != nil {
		return skerr.Wrapf(err, "listing failed pods")
	}

	for _, p := range pods.Items {
		evictedMetricTags := map[string]string{
			"pod":       p.ObjectMeta.Name,
			"cluster":   cluster,
			"namespace": fixupNamespace(namespace),
			"reason":    p.Status.Reason,
			"message":   p.Status.Message,
		}
		evictedMetric := metrics2.GetInt64Metric(evictedPodMetric, evictedMetricTags)
		metrics[evictedMetric] = struct{}{}
		if strings.Contains(p.Status.Reason, "Evicted") {
			evictedMetric.Update(1)
		} else {
			evictedMetric.Update(0)
		}
	}

	return nil
}

// getPodMetrics reports metrics for all pods and places them into the specified
// metrics map.
func getPodMetrics(ctx context.Context, cluster, namespace string, clientset *kubernetes.Clientset, metrics map[metrics2.Int64Metric]struct{}) error {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return skerr.Wrapf(err, "listing all pods")
	}

	for _, p := range pods.Items {
		for _, c := range p.Status.ContainerStatuses {
			tags := map[string]string{
				"app":       p.Labels["app"],
				"pod":       p.ObjectMeta.Name,
				"container": c.Name,
				"cluster":   cluster,
				"namespace": fixupNamespace(namespace),
			}
			restarts := metrics2.GetInt64Metric(podRestartCountMetric, tags)
			restarts.Update(int64(c.RestartCount))
			metrics[restarts] = struct{}{}

			running := metrics2.GetInt64Metric(podRunningMetric, tags)
			isRunning := int64(0)
			if c.State.Running != nil {
				isRunning = 1
			}
			running.Update(isRunning)
			metrics[running] = struct{}{}

			ready := metrics2.GetInt64Metric(podReadyMetric, tags)
			isReady := int64(0)
			if c.Ready {
				isReady = 1
			}
			ready.Update(isReady)
			metrics[ready] = struct{}{}

			for _, containerSpec := range p.Spec.Containers {
				if containerSpec.Name == c.Name {
					if containerSpec.ReadinessProbe != nil {
						rp := containerSpec.ReadinessProbe
						maxReadyTime := rp.InitialDelaySeconds + (rp.FailureThreshold+rp.SuccessThreshold)*(rp.PeriodSeconds+rp.TimeoutSeconds)
						mrtMetric := metrics2.GetInt64Metric(podMaxReadyTimeMetric, tags)
						mrtMetric.Update(int64(maxReadyTime))
						metrics[mrtMetric] = struct{}{}
					}
				}
			}
		}
	}
	return nil
}

// getLiveAppContainersToImages returns a map of app names to their containers to the images running on them.
func getLiveAppContainersToImages(ctx context.Context, namespace string, clientset *kubernetes.Clientset) (map[string]map[string]string, error) {
	// Get JSON output of pods running in K8s.
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "listing running pods")
	}
	liveAppContainersToImages := map[string]map[string]string{}
	for _, p := range pods.Items {
		if app, ok := p.Labels["app"]; ok {
			liveAppContainersToImages[app] = map[string]string{}
			for _, container := range p.Spec.Containers {
				liveAppContainersToImages[app][container.Name] = container.Image
			}
		} else {
			sklog.Infof("No app label found for pod %+v", p)
		}
	}

	return liveAppContainersToImages, nil
}

// performChecks checks for:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
// * How old the image running in K8s is.
// * Apps and containers running in K8s but not checked into the git repo.
// * Apps and containers checked into the git repo but not running in K8s.
// * Checks for evicted pods.
//
// It takes in a map of oldMetrics, any metrics from that map that are not encountered during this
// invocation of the function are deleted. This is done to handle the case when metric tags
// change. Eg: liveImage in dirtyConfigMetricTags.
// It returns a map of newMetrics, which are all the metrics that were used during this
// invocation of the function.
func performChecks(ctx context.Context, cluster, repo string, clientset *kubernetes.Clientset, ignoreNamespaces []string, g *gitiles.Repo, oldMetrics map[metrics2.Int64Metric]struct{}, allowedAppsByNamespace allowedAppsInNamespace) (map[metrics2.Int64Metric]struct{}, error) {
	sklog.Info("---------- New round of checking k8s ----------")
	newMetrics := map[metrics2.Int64Metric]struct{}{}

	namespaces, err := getNamespaces(ctx, cluster, clientset, ignoreNamespaces)
	if err != nil {
		return nil, skerr.Wrapf(err, "retrieving namespaces")
	}

	liveAppContainerToImagesByNamespace := make(map[string]map[string]map[string]string, len(namespaces))
	for _, namespace := range namespaces {
		// Check for evicted pods.
		if err := getEvictedPods(ctx, cluster, namespace, clientset, newMetrics); err != nil {
			return nil, skerr.Wrapf(err, "checking for evicted pods from kubectl")
		}

		// Check for crashing pods.
		if err := getPodMetrics(ctx, cluster, namespace, clientset, newMetrics); err != nil {
			return nil, skerr.Wrapf(err, "checking for crashing pods from kubectl")
		}

		// Get mapping from live apps to their containers and images.
		liveAppContainerToImages, err := getLiveAppContainersToImages(ctx, namespace, clientset)
		if err != nil {
			return nil, skerr.Wrapf(err, "getting live pods from kubectl for cluster %s", cluster)
		}
		liveAppContainerToImagesByNamespace[namespace] = liveAppContainerToImages
	}
	// TODO(borenet): Remove this logging after debugging.
	b, err := json.MarshalIndent(liveAppContainerToImagesByNamespace, "", "  ")
	if err == nil {
		sklog.Infof("Running apps to containers by namespace: %s", string(b))
	}

	// Read files from the repo using gitiles.
	fileInfos, err := g.ListDirAtRef(ctx, cluster, git.MainBranch)
	if err != nil {
		return nil, skerr.Wrapf(err, "listing files from %s", repo)
	}

	checkedInAppsToContainers := map[string]util.StringSet{}
	for _, fi := range fileInfos {
		if fi.IsDir() {
			// Only interested in files.
			continue
		}
		f := fi.Name()
		if filepath.Ext(f) != ".yaml" {
			// Only interested in YAML configs.
			continue
		}
		yamlContents, err := g.ReadFileAtRef(ctx, filepath.Join(cluster, f), git.MainBranch)
		if err != nil {
			return nil, skerr.Wrapf(err, "reading file %s from %s in cluster %s", f, repo, cluster)
		}

		// There can be multiple YAML documents within a single YAML file.
		deployments, statefulSets, cronJobs, daemonSets, err := k8s_config.ParseK8sConfigFile(yamlContents)
		if err != nil {
			sklog.Errorf("Error when parsing %s: %s", filepath.Join(cluster, f), err)
			continue
		}
		for _, config := range cronJobs {
			namespace := fixupNamespace(config.Namespace)
			for _, c := range config.Spec.JobTemplate.Spec.Template.Spec.Containers {
				// Check if the image in the config is dirty.
				addMetricForDirtyCommittedImage(f, repo, cluster, namespace, c.Image, newMetrics)

				// Now add a metric for how many days old the committed image is.
				if err := addMetricForImageAge(c.Name, c.Name, namespace, f, repo, c.Image, newMetrics); err != nil {
					sklog.Errorf("Could not add image age metric for %s: %s", c.Name, err)
				}
			}
		}

		apps := []string{}
		namespaces := []string{}
		containers := [][]v1.Container{}
		volumeClaims := [][]v1.PersistentVolumeClaim{}
		for _, config := range deployments {
			apps = append(apps, config.Spec.Template.Labels[appLabel])
			namespaces = append(namespaces, fixupNamespace(config.Namespace))
			containers = append(containers, config.Spec.Template.Spec.Containers)
			volumeClaims = append(volumeClaims, []v1.PersistentVolumeClaim{})
		}
		for _, config := range statefulSets {
			apps = append(apps, config.Spec.Template.Labels[appLabel])
			namespaces = append(namespaces, fixupNamespace(config.Namespace))
			containers = append(containers, config.Spec.Template.Spec.Containers)
			volumeClaims = append(volumeClaims, config.Spec.VolumeClaimTemplates)
		}
		for _, config := range daemonSets {
			apps = append(apps, config.Spec.Template.Labels[appLabel])
			namespaces = append(namespaces, fixupNamespace(config.Namespace))
			containers = append(containers, config.Spec.Template.Spec.Containers)
			volumeClaims = append(volumeClaims, []v1.PersistentVolumeClaim{})
		}
		for idx, app := range apps {
			// Create a simple mapping of volume name to its size.
			volumeNameToSize := map[string]int64{}
			for _, vol := range volumeClaims[idx] {
				volumeNameToSize[vol.Name] = vol.Spec.Resources.Requests.Storage().Value()
			}

			checkedInAppsToContainers[app] = util.StringSet{}

			// Loop through the containers for this app.
			for _, c := range containers[idx] {
				namespace := namespaces[idx]
				liveAppContainerToImages := liveAppContainerToImagesByNamespace[namespace]

				container := c.Name
				committedImage := c.Image
				checkedInAppsToContainers[app][c.Name] = true

				// Check if the image in the config is dirty.
				addMetricForDirtyCommittedImage(f, repo, cluster, namespace, committedImage, newMetrics)

				// Check if the config specifies ephemeral disk requests.
				diskRequestMetricTags := map[string]string{
					"app":       app,
					"container": container,
					"yaml":      f,
					"repo":      repo,
					"cluster":   cluster,
					"namespace": fixupNamespace(namespace),
				}
				ephemeralDiskRequestMetric := metrics2.GetInt64Metric(ephemeralDiskRequestMetric, diskRequestMetricTags)
				newMetrics[ephemeralDiskRequestMetric] = struct{}{}
				ephemeralDiskRequest := c.Resources.Requests.StorageEphemeral().Value()
				ephemeralDiskRequestMetric.Update(ephemeralDiskRequest)

				// Find the total requested disk.
				totalDiskRequest := ephemeralDiskRequest + c.Resources.Requests.Storage().Value()
				for _, vol := range c.VolumeMounts {
					totalDiskRequest += volumeNameToSize[vol.Name]
				}
				totalDiskRequestMetric := metrics2.GetInt64Metric(totalDiskRequestMetric, diskRequestMetricTags)
				newMetrics[totalDiskRequestMetric] = struct{}{}
				totalDiskRequestMetric.Update(totalDiskRequest)

				// Create app_running metric.
				appRunningMetricTags := map[string]string{
					"app":       app,
					"yaml":      f,
					"repo":      repo,
					"cluster":   cluster,
					"namespace": fixupNamespace(namespace),
				}
				appRunningMetric := metrics2.GetInt64Metric(appRunningMetric, appRunningMetricTags)
				newMetrics[appRunningMetric] = struct{}{}

				// Check if the image running in k8s matches the checked in image.
				if liveContainersToImages, ok := liveAppContainerToImages[app]; ok {
					appRunningMetric.Update(1)

					// Create container_running metric.
					containerRunningMetricTags := map[string]string{
						"app":       app,
						"container": container,
						"yaml":      f,
						"repo":      repo,
						"cluster":   cluster,
						"namespace": fixupNamespace(namespace),
					}
					containerRunningMetric := metrics2.GetInt64Metric(containerRunningMetric, containerRunningMetricTags)
					newMetrics[containerRunningMetric] = struct{}{}

					if liveImage, ok := liveContainersToImages[container]; ok {
						containerRunningMetric.Update(1)

						dirtyConfigMetricTags := map[string]string{
							"app":            app,
							"container":      container,
							"yaml":           f,
							"repo":           repo,
							"cluster":        cluster,
							"namespace":      fixupNamespace(namespace),
							"committedImage": committedImage,
							"liveImage":      liveImage,
						}
						dirtyConfigMetric := metrics2.GetInt64Metric(dirtyConfigMetric, dirtyConfigMetricTags)
						newMetrics[dirtyConfigMetric] = struct{}{}
						if liveImage != committedImage {
							dirtyConfigMetric.Update(1)
							sklog.Infof("For app %s and container %s the running image differs from the image in config: %s != %s", app, container, liveImage, committedImage)
						} else {
							// The live image is the same as the committed image.
							dirtyConfigMetric.Update(0)

							// Now add a metric for how many days old the live/committed image is.
							if err := addMetricForImageAge(app, container, namespace, f, repo, liveImage, newMetrics); err != nil {
								sklog.Errorf("Could not add image age metric for %s: %s", container, err)
							}
						}
					} else {
						sklog.Infof("There is no running container %s for the config file %s", container, f)
						containerRunningMetric.Update(0)
					}
				} else {
					sklog.Infof("There is no running app %s for the config file %s", app, f)
					appRunningMetric.Update(0)
				}
			}
		}
	}
	// TODO(borenet): Remove this logging after debugging.
	b, err = json.MarshalIndent(checkedInAppsToContainers, "", "  ")
	if err == nil {
		sklog.Infof("Checked in apps to containers: %s", string(b))
	}

	// Find out which apps and containers are live but not found in git repo.
	for namespace, liveAppContainerToImages := range liveAppContainerToImagesByNamespace {
		for liveApp := range liveAppContainerToImages {
			ns := fixupNamespace(namespace)
			runningAppHasConfigMetricTags := map[string]string{
				"app":       liveApp,
				"repo":      repo,
				"cluster":   cluster,
				"namespace": ns,
			}
			runningAppHasConfigMetric := metrics2.GetInt64Metric(runningAppHasConfigMetric, runningAppHasConfigMetricTags)
			newMetrics[runningAppHasConfigMetric] = struct{}{}
			if checkedInApp, ok := checkedInAppsToContainers[liveApp]; ok {
				runningAppHasConfigMetric.Update(1)

				for liveContainer := range liveAppContainerToImages[liveApp] {
					runningContainerHasConfigMetricTags := map[string]string{
						"app":       liveApp,
						"container": liveContainer,
						"repo":      repo,
						"cluster":   cluster,
						"namespace": ns,
					}
					runningContainerHasConfigMetric := metrics2.GetInt64Metric(runningContainerHasConfigMetric, runningContainerHasConfigMetricTags)
					newMetrics[runningContainerHasConfigMetric] = struct{}{}
					if _, ok := checkedInApp[liveContainer]; ok {
						runningContainerHasConfigMetric.Update(1)
					} else {
						sklog.Infof("The running container %s of app %s is not checked into %s", liveContainer, liveApp, repo)
						runningContainerHasConfigMetric.Update(0)
					}
				}
			} else if util.In(liveApp, allowedAppsByNamespace[ns]) {
				sklog.Infof("The running app %s is allowed in namespace %s in repo %s", liveApp, ns, repo)
				runningAppHasConfigMetric.Update(1)
			} else {
				sklog.Infof("The running app %s is not checked into %s", liveApp, repo)
				runningAppHasConfigMetric.Update(0)
			}
		}
	}

	// Delete unused old metrics.
	for m := range oldMetrics {
		if _, ok := newMetrics[m]; !ok {
			if err := m.Delete(); err != nil {
				sklog.Errorf("Failed to delete metric: %s", err)
				// Add the metric to newMetrics so that we'll
				// have the chance to delete it again on the
				// next cycle.
				newMetrics[m] = struct{}{}
			}
		}
	}

	return newMetrics, nil
}

// addMetricForDirtyCommittedImage creates a metric for if the committed image is dirty, and adds
// it to the metrics map.
func addMetricForDirtyCommittedImage(yaml, repo, cluster, namespace, committedImage string, metrics map[metrics2.Int64Metric]struct{}) {
	dirtyCommittedMetricTags := map[string]string{
		"yaml":           yaml,
		"repo":           repo,
		"cluster":        cluster,
		"namespace":      fixupNamespace(namespace),
		"committedImage": committedImage,
	}
	dirtyCommittedMetric := metrics2.GetInt64Metric(dirtyCommittedImageMetric, dirtyCommittedMetricTags)
	metrics[dirtyCommittedMetric] = struct{}{}
	if strings.HasSuffix(committedImage, dirtyImageSuffix) {
		sklog.Infof("%s has a dirty committed image: %s", yaml, committedImage)
		dirtyCommittedMetric.Update(1)
	} else {
		dirtyCommittedMetric.Update(0)
	}
}

// addMetricForImageAge creates a metric for how old the specified image is, and adds it to the
// metrics map.
func addMetricForImageAge(app, container, namespace, yaml, repo, image string, metrics map[metrics2.Int64Metric]struct{}) error {
	m := imageRegex.FindStringSubmatch(image)
	if len(m) == 2 {
		t, err := time.Parse(time.RFC3339, strings.ReplaceAll(m[1], "_", ":"))
		if err != nil {
			return skerr.Wrapf(err, "parsing time %s from image %s", m[1], image)
		}
		staleImageMetricTags := map[string]string{
			"app":       app,
			"container": container,
			"yaml":      yaml,
			"repo":      repo,
			"liveImage": image,
			"namespace": fixupNamespace(namespace),
		}
		staleImageMetric := metrics2.GetInt64Metric(staleImageMetric, staleImageMetricTags)
		metrics[staleImageMetric] = struct{}{}
		numDaysOldImage := int64(time.Now().UTC().Sub(t).Hours() / 24)
		staleImageMetric.Update(numDaysOldImage)
	}
	return nil
}
