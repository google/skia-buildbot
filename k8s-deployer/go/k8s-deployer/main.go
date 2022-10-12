package main

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	livenessMetric       = "k8s_deployer"
	gitstoreSubscriberID = "k8s-deployer"
)

func main() {
	autoDeleteCrashingStatefulSetPods := flag.Bool("auto_delete_crashing_statefulset_pods", false, "If set, delete out-of-date and crashing StatefulSet pods after applying changes.")
	configRepo := flag.String("config_repo", "https://skia.googlesource.com/k8s-config.git", "Repo containing Kubernetes configurations.")
	configSubdir := flag.String("config_subdir", "", "Subdirectory within the config repo to apply to this cluster.")
	configFiles := common.NewMultiStringFlag("config_file", nil, "Individual config files to apply. Supports regex. Incompatible with --prune.")
	interval := flag.Duration("interval", 10*time.Minute, "How often to re-apply configurations to the cluster")
	port := flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	promPort := flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	prune := flag.Bool("prune", false, "Whether to run 'kubectl apply' with '--prune'")
	kubectl := flag.String("kubectl", "kubectl", "Path to the kubectl executable.")
	k8sServer := flag.String("k8s_server", "", "Address of the Kubernetes server.")

	common.InitWithMust("k8s_deployer", common.PrometheusOpt(promPort))
	defer sklog.Flush()

	if *configRepo == "" {
		sklog.Fatal("config_repo is required.")
	}
	if *configSubdir == "" {
		// Note: this wouldn't be required if we had separate config repos per
		// cluster.
		sklog.Fatal("config_subdir is required.")
	}
	var configFileRegexes []*regexp.Regexp
	if len(*configFiles) > 0 {
		if *prune {
			sklog.Fatal("--config_file is incompatible with --prune.")
		}
		for _, expr := range *configFiles {
			re, err := regexp.Compile(expr)
			if err != nil {
				sklog.Fatal(err)
			}
			configFileRegexes = append(configFileRegexes, re)
		}
	}

	ctx := context.Background()

	// OAuth2.0 TokenSource.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}
	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Gitiles repo.
	repo := gitiles.NewRepo(*configRepo, httpClient)

	// Kubernetes API client.
	config, err := rest.InClusterConfig()
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster config: %s", err)
	}
	sklog.Infof("Auth username: %s", config.Username)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster clientset: %s", err)
	}

	// Apply configurations in a loop.  Note that we could respond directly to
	// commits in the repo via GitStore and PubSub, but that would require
	// access to BigTable, and with a relatively small interval we won't notice
	// too much of a delay.
	liveness := metrics2.NewLiveness(livenessMetric)
	go util.RepeatCtx(ctx, *interval, func(ctx context.Context) {
		applyErr := applyConfigs(ctx, repo, *kubectl, *k8sServer, *configSubdir, configFileRegexes, *prune, clientset)
		if applyErr != nil {
			sklog.Errorf("Failed to apply configs to cluster: %s", applyErr)
		}

		// Delete any outdated and crash-looping StatefulSet pods.  These do not
		// get updated via "kubectl apply" and must be deleted explicitly in
		// order to be updated.
		var deleteErr error
		if *autoDeleteCrashingStatefulSetPods {
			deleteErr = deleteCrashingStatefulSetPods(ctx, clientset)
			if deleteErr != nil {
				sklog.Errorf("Failed to delete crashing StatefulSet pods: %s", deleteErr)
			}
		}
		if applyErr == nil && deleteErr == nil {
			liveness.Reset()
		}
	})

	// Run health check server.
	httputils.RunHealthCheckServer(*port)
}

func applyConfigs(ctx context.Context, repo *gitiles.Repo, kubectl, k8sServer, configSubdir string, configFileRegexes []*regexp.Regexp, prune bool, clientset *kubernetes.Clientset) error {
	// Download the configs from Gitiles instead of maintaining a local Git
	// checkout, to avoid dealing with Git, persistent checkouts, etc.

	// Obtain the current set of configurations for the cluster.
	head, err := repo.Details(ctx, "main")
	if err != nil {
		return skerr.Wrapf(err, "failed to get most recent commit")
	}
	files, err := repo.ListFilesRecursiveAtRef(ctx, configSubdir, head.Hash)
	if err != nil {
		return skerr.Wrapf(err, "failed to list configs")
	}

	// Read the config contents in the given directory.
	eg := util.NewNamedErrGroup()
	contents := make(map[string][]byte, len(files))
	contentsMtx := sync.Mutex{}
	sklog.Infof("Downloading config files at %s", head.Hash)
	for _, file := range files {
		file := file // https://golang.org/doc/faq#closures_and_goroutines

		// Ensure that the file matches any provided regular expressions.
		if len(configFileRegexes) > 0 {
			match := false
			for _, re := range configFileRegexes {
				if re.MatchString(file) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Download the file contents.
		eg.Go(file, func() error {
			fullPath := path.Join(configSubdir, file)
			sklog.Infof("  %s", fullPath)
			fileContents, err := repo.ReadFileAtRef(ctx, fullPath, head.Hash)
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve contents of %s", fullPath)
			}
			contentsMtx.Lock()
			defer contentsMtx.Unlock()
			contents[file] = fileContents
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return skerr.Wrapf(err, "failed to download configs")
	}

	// Write the config contents to a temporary dir.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return skerr.Wrapf(err, "failed to create temp dir")
	}
	defer util.RemoveAll(tmp)

	for path, fileContents := range contents {
		fullPath := filepath.Join(tmp, path)
		dir := filepath.Dir(fullPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return skerr.Wrapf(err, "failed to create %s", dir)
			}
		}
		if err := ioutil.WriteFile(fullPath, fileContents, os.ModePerm); err != nil {
			return skerr.Wrapf(err, "failed to create %s", fullPath)
		}
	}

	// Apply the configs to the cluster.
	// Note: this is a very naive approach.  We rely on the Kubernetes server to
	// determine when changes actually need to be made.  We could instead use
	// the REST API to load the full set of configuration which is actually
	// running on the server, diff that against the checked-in config files,
	// and then explicitly make only the changes we want to make.  That would be
	// a much more complicated and error-prone approach, but it would allow us
	// to partially apply configurations in the case of a failure and to alert
	// on specific components which fail to apply for whatever reason.
	cmd := []string{kubectl, "apply"}
	if k8sServer != "" {
		cmd = append(cmd, "--server", k8sServer)
	}
	if prune {
		cmd = append(cmd, "--prune", "--all")
	}
	cmd = append(cmd, "-f", ".")
	output, err := exec.RunCwd(ctx, tmp, cmd...)
	if err != nil {
		return skerr.Wrapf(err, "failed to apply configs: %s", output)
	}
	sklog.Info("Output from kubectl")
	for _, line := range strings.Split(output, "\n") {
		sklog.Info(line)
	}
	return deleteCrashingStatefulSetPods(ctx, clientset)
}

// deleteCrashingStatefulSetPods finds all pods which are crash-looping and are
// owned by a StatefulSet which is outdated and deletes them.  This should
// prevent needing to manually run "kubectl delete pod" when a fix has landed
// for a StatefulSet but the new pod hasn't started because the old pod is
// crash-looping.
func deleteCrashingStatefulSetPods(ctx context.Context, clientset *kubernetes.Clientset) error {
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve namespaces")
	}
	for _, namespace := range namespaces.Items {
		pods, err := clientset.CoreV1().Pods(namespace.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return skerr.Wrapf(err, "failed to retrieve pods in namespace %q", namespace.Name)
		}
		for _, pod := range pods.Items {
			// Find the StatefulSet that owns this pod, if any.
			statefulSetName := ""
			for _, ownerRef := range pod.ObjectMeta.OwnerReferences {
				if ownerRef.Kind == "StatefulSet" {
					statefulSetName = ownerRef.Name
					break
				}
			}
			if statefulSetName == "" {
				continue
			}

			// Is any container in the pod crashing?
			isCrashing := false
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
					isCrashing = true
				}
			}
			if !isCrashing {
				continue
			}

			// Does the StatefulSet have a pending update?
			statefulSet, err := clientset.AppsV1().StatefulSets(namespace.Name).Get(ctx, statefulSetName, metav1.GetOptions{})
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve statefulset %q in namespace %q", statefulSetName, namespace.Name)
			}
			if statefulSet.Status.CurrentRevision != statefulSet.Status.UpdateRevision {
				// Delete the pod.
				sklog.Infof("Deleting crash-looping and outdated pod %q in namespace %q", pod.Name, namespace.Name)
				if err := clientset.CoreV1().Pods(namespace.Name).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
					return skerr.Wrapf(err, "failed to delete pod %q in namespace %q", pod.Name, namespace.Name)
				}
			}
		}
	}
	return nil
}
