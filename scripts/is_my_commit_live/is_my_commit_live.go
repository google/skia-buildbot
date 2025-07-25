package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/k8s"
	"go.skia.org/infra/go/kube/clusterconfig"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	ctx := context.Background()

	// Parse and validate flags.
	var (
		commit  = flag.String("commit", "", "Commit hash or reference to test")
		repoURL = flag.String("repo", common.REPO_SKIA_INFRA, "Git repo URL to test")

		// --image and --container are mutually exclusive.
		image     = flag.String("image", "", "Fully-qualified Docker image ID")
		container = flag.String("container", "", "String or regular expression indicating which live containers to check")

		// These are only valid with --container.
		cluster           = flag.String("cluster", "", "Optional. Kubernetes cluster of the container to test")
		namespace         = flag.String("namespace", "", "Optional. Kubernetes namespace of the container to test")
		kubeConfig        = flag.String("kube_config", "", "Path to Kubernetes config file. If not provided, attempt to find in HOME")
		clusterConfigFile = flag.String("cluster_config", "", "Path to cluster config file. If not provided, attempt to find in the current repo.")
	)

	flag.Parse()
	if *commit == "" {
		sklog.Fatal("--commit is required")
	}
	if (*image == "" && *container == "") || (*image != "" && *container != "") {
		sklog.Fatal("Exactly one of --image or --container is required")
	}
	if *container == "" {
		if *cluster != "" {
			sklog.Fatal("--cluster is only valid with --container")
		}
		if *namespace != "" {
			sklog.Fatal("--namespace is only valid with --container")
		}
	}

	// Set up clients.
	ts, err := google.DefaultTokenSource(ctx, gerrit.AuthScope)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().With2xxAnd3xx().WithTokenSource(ts).Client()
	repo := gitiles.NewRepo(*repoURL, httpClient)
	dockerClient, err := docker.NewClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	// Find (if necessary) and check the image(s).
	if *image != "" {
		isIncluded, err := isCommitInImage(ctx, dockerClient, repo, *commit, *image)
		if err != nil {
			sklog.Fatal(err)
		}
		fmt.Printf("%s: %v\n", *image, isIncluded)
	} else {
		containerRegex, err := regexp.Compile(*container)
		if err != nil {
			sklog.Fatal(err)
		}
		clusterRegex, err := regexp.Compile(*cluster)
		if err != nil {
			sklog.Fatal(err)
		}
		namespaceRegex, err := regexp.Compile(*namespace)
		if err != nil {
			sklog.Fatal(err)
		}
		if *kubeConfig == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				sklog.Fatalf("Failed to find home directory. Maybe supply --kube_config? %s", err)
			}
			*kubeConfig = filepath.Join(homeDir, ".kube", "config")
		}
		if _, err := os.Stat(*kubeConfig); os.IsNotExist(err) {
			sklog.Fatalf("Failed to find kubeconfig at %s; try a different value for --kube_config? %s", *kubeConfig, err)
		}
		if *clusterConfigFile == "" {
			repoRoot, err := repo_root.GetLocal()
			if err != nil {
				sklog.Fatalf("Failed to find repo root. Maybe supply --cluster_config")
			}
			*clusterConfigFile = filepath.Join(repoRoot, "kube", "clusters", "config.json")
		}
		if _, err := os.Stat(*clusterConfigFile); os.IsNotExist(err) {
			sklog.Fatalf("Failed to find cluster config at %s; try a different value for --cluster_config? %s", *clusterConfigFile, err)
		}

		imageToContainers, err := findMatchingContainerImages(ctx, *kubeConfig, *clusterConfigFile, clusterRegex, namespaceRegex, containerRegex)
		if err != nil {
			sklog.Fatal(err)
		}
		if len(imageToContainers) == 0 {
			sklog.Fatal("Found no images matching `%s`", *container)
		}
		for image := range imageToContainers {
			isIncluded, err := isCommitInImage(ctx, dockerClient, repo, *commit, image)
			if err != nil {
				sklog.Fatal(err)
			}
			fmt.Printf("%s: %v\n", image, isIncluded)
			for _, container := range imageToContainers[image] {
				fmt.Printf("  %s\n", container)
			}
		}
	}
}

// findMatchingContainerImages returns a mapping of image ID to slices of
// container names which are running each image.
func findMatchingContainerImages(ctx context.Context, kubeConfigFile, clusterConfigFile string, clusterRegex, namespaceRegex, containerRegex *regexp.Regexp) (map[string][]string, error) {
	clusterConfig, err := clusterconfig.New(clusterConfigFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	imageToContainers := map[string][]string{}
	for cluster := range clusterConfig.Clusters {
		if cluster == "skia-corp" || cluster == "skia-switchboard" {
			// TODO(borenet): These clusters no longer exist; remove from config?
			continue
		}
		if !clusterRegex.MatchString(cluster) {
			continue
		}
		client, err := k8s.NewLocalClient(ctx, kubeConfigFile, clusterConfigFile, cluster)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		namespaces, err := client.ListNamespaces(ctx, v1.ListOptions{})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for _, ns := range namespaces {
			if !namespaceRegex.MatchString(ns.Name) {
				continue
			}
			pods, err := client.ListPods(ctx, ns.Name, v1.ListOptions{})
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			for _, pod := range pods {
				for _, container := range pod.Spec.Containers {
					if containerRegex.MatchString(container.Name) {
						imageToContainers[container.Image] = append(imageToContainers[container.Image], container.Name)
					}
				}
			}
		}
	}
	return imageToContainers, nil
}

// isCommitInImage finds the commit hashes associated with the given image via
// tags, then checks to see whether the given commit is an ancestor of any of
// those commits.
func isCommitInImage(ctx context.Context, dockerClient docker.Client, repo *gitiles.Repo, commit, image string) (bool, error) {
	registry, repository, tagOrDigest, err := docker.SplitImage(image)
	if err != nil {
		return false, skerr.Wrap(err)
	}

	digest, err := docker.GetDigestIfNeeded(ctx, dockerClient, registry, repository, tagOrDigest)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	commitHashes, err := stages.GetCommitHashesForImage(ctx, dockerClient, registry, repository, digest)
	if err != nil {
		return false, skerr.Wrap(err)
	}

	for _, hash := range commitHashes {
		if hash == commit {
			return true, nil
		}
	}
	for _, hash := range commitHashes {
		commits, err := repo.Log(ctx, fmt.Sprintf("%s..%s", commit, hash))
		if err != nil {
			return false, skerr.Wrap(err)
		}
		if len(commits) > 0 {
			return true, nil
		}
	}
	return false, nil
}
