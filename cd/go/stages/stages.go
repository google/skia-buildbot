package stages

import (
	"bytes"
	"context"
	filesystem "io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
	"go.skia.org/infra/k8s-checker/go/k8s_config"
	v1 "k8s.io/api/core/v1"
)

const (
	// StageFilePath is the path to the json file.
	StageFilePath = "pipeline/stages.json"

	// GitTagPrefix is a prefix applied to all Docker image tags which
	// correspond to Git commit hashes.
	GitTagPrefix = "git-"

	// PodAnnotationKey is the key of the annotation which is attached to
	// PodTemplateSpec to map container names to stages.
	PodAnnotationKey = "skia.org/stage"
)

var (
	// TODO(borenet): Can we avoid hard-coding these directories?
	configDirs = []string{"skia-corp", "skia-public", "skia-infra-corp", "skia-infra-public", "skia-infra-public-dev", "templates"}
)

// CommitResolver is a function used to resolve a short Git commit hash or ref
// to a full commit hash.
type CommitResolver func(ctx context.Context, repoURL, reference string) (string, error)

// GitilesCommitResolver returns a CommitResolver which uses gitiles and the
// given http.Client to resolve commits.
func GitilesCommitResolver(httpClient *http.Client) CommitResolver {
	return func(ctx context.Context, repoURL, reference string) (string, error) {
		return gitiles.NewRepo(repoURL, httpClient).ResolveRef(ctx, reference)
	}
}

// StageManager provides utilities for working with release stage files.
type StageManager struct {
	fs             vfs.FS
	docker         docker.Client
	commitResolver CommitResolver
}

// NewStageManager returns a StageManager instance. The http.Client is used for
// interacting with git repositories and should have the necessary
// authentication settings (eg. OAuth2.0 token source and scopes) attached.
func NewStageManager(ctx context.Context, fs vfs.FS, dockerClient docker.Client, commitResolver CommitResolver) *StageManager {
	return &StageManager{
		fs:             fs,
		docker:         dockerClient,
		commitResolver: commitResolver,
	}
}

// AddImage adds the given image to the stage file. The gitRepo is optional and
// overrides the default git repo.
func (sm *StageManager) AddImage(ctx context.Context, image, gitRepo string) error {
	return sm.updateImages(ctx, func(sf *StageFile) error {
		if _, ok := sf.Images[image]; ok {
			return skerr.Fmt("image %q already exists", image)
		}
		if sf.Images == nil {
			sf.Images = map[string]*Image{}
		}
		sf.Images[image] = &Image{
			GitRepo: gitRepo,
		}
		return nil
	})
}

// RemoveImage removes the given image from the stage file.
func (sm *StageManager) RemoveImage(ctx context.Context, image string) error {
	return sm.updateImages(ctx, func(sf *StageFile) error {
		if _, ok := sf.Images[image]; !ok {
			return skerr.Fmt("image %q does not exist", image)
		}
		delete(sf.Images, image)
		return nil
	})
}

// SetStage sets the given stage of the given image to match the given
// reference, which may be a Docker image digest, tag, or Git commit hash.
func (sm *StageManager) SetStage(ctx context.Context, image, stage, reference string) error {
	registry, repository, _, err := docker.SplitImage(image)
	if err != nil {
		return skerr.Wrap(err)
	}
	return sm.updateImages(ctx, func(sf *StageFile) error {
		img, ok := sf.Images[image]
		if !ok {
			return skerr.Fmt("unknown image %q", image)
		}

		// If the reference looks like a Git commit hash, query the git repository
		// to validate it and to retrieve the full hash.
		var gitHash string
		maybeGitHash := strings.TrimPrefix(reference, GitTagPrefix)
		if git.IsCommitHash(maybeGitHash) {
			repoURL := img.GitRepo
			if repoURL == "" {
				repoURL = sf.DefaultGitRepo
			}
			fullHash, err := sm.commitResolver(ctx, repoURL, maybeGitHash)
			// The ref may look like a git commit hash and not be, and could
			// still be a valid Docker image tag, so we don't return the error
			// here.
			if err == nil {
				gitHash = fullHash
				reference = GitTagPrefix + fullHash
			}
		}

		// Retrieve the digest of the image.
		manifest, err := sm.docker.GetManifest(ctx, registry, repository, reference)
		if err != nil {
			return skerr.Wrap(err)
		}
		digest := manifest.Digest

		// Find a "git-abc123" tag for the image, derive the commit hash.
		if gitHash == "" {
			instances, err := sm.docker.ListInstances(ctx, registry, repository)
			if err != nil {
				return skerr.Wrap(err)
			}
			instance, ok := instances[digest]
			if !ok {
				return skerr.Fmt("failed to find instance %q of %s", digest, image)
			}
			for _, tag := range instance.Tags {
				if strings.HasPrefix(tag, GitTagPrefix) {
					maybeHash := strings.TrimPrefix(tag, GitTagPrefix)
					if git.IsFullCommitHash(maybeHash) {
						gitHash = maybeHash
						break
					}
				}
			}
		}
		if gitHash == "" {
			return skerr.Fmt("failed to find \"git-\" tag on instance %q of %s", digest, image)
		}

		// Update the stage file.
		if img.Stages == nil {
			img.Stages = map[string]*Stage{}
		}
		img.Stages[stage] = &Stage{
			GitHash: gitHash,
			Digest:  digest,
		}
		return nil
	})
}

// PromoteStage sets a given stage to the same image version as another.
func (sm *StageManager) PromoteStage(ctx context.Context, image, stageToMatch, stageToUpdate string) error {
	return sm.updateImages(ctx, func(sf *StageFile) error {
		img, ok := sf.Images[image]
		if !ok {
			return skerr.Fmt("image %q does not exist in %s", image, StageFilePath)
		}
		matchStage, ok := img.Stages[stageToMatch]
		if !ok {
			return skerr.Fmt("stage %q does not exist for image %s in %s", stageToMatch, image, StageFilePath)
		}
		img.Stages[stageToUpdate] = &Stage{
			GitHash: matchStage.GitHash,
			Digest:  matchStage.Digest,
		}
		return nil
	})
}

// RemoveStage removes the given stage from the stage file.
func (sm *StageManager) RemoveStage(ctx context.Context, image, stage string) error {
	return sm.updateImages(ctx, func(sf *StageFile) error {
		img, ok := sf.Images[image]
		if !ok {
			return skerr.Fmt("image %q does not exist in %s", image, StageFilePath)
		}
		if _, ok := img.Stages[stage]; !ok {
			return skerr.Fmt("stage %q does not exist for image %s in %s", stage, image, StageFilePath)
		}
		delete(img.Stages, stage)
		return nil
	})
}

// Apply updates all config files to conform to the stage file.
func (sm *StageManager) Apply(ctx context.Context) error {
	return sm.updateImages(ctx, func(sf *StageFile) error {
		return nil
	})
}

// ReadStageFile reads and returns the stage file.
func (sm *StageManager) ReadStageFile(ctx context.Context) (*StageFile, error) {
	var rv *StageFile
	if err := sm.updateImages(ctx, func(sf *StageFile) error {
		rv = sf
		return nil
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// updateImages generates updates to the stage file and configs in the config
// repo. The passed-in function may manipulate the StageFile as desired, and
// updateImages will handle updates to the other config files.
func (sm *StageManager) updateImages(ctx context.Context, fn func(*StageFile) error) error {
	oldContents, err := vfs.ReadFile(ctx, sm.fs, StageFilePath)
	if err != nil {
		return skerr.Wrap(err)
	}
	sf, err := Decode([]byte(oldContents))
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := fn(sf); err != nil {
		return skerr.Wrap(err)
	}
	newContents, err := sf.Encode()
	if err != nil {
		return skerr.Wrap(err)
	}

	var changesMtx sync.Mutex
	changes := map[string][]byte{}
	if !bytes.Equal(oldContents, newContents) {
		changes[StageFilePath] = newContents
	}

	eg := util.NewNamedErrGroup()
	for _, configDir := range configDirs {
		err := vfs.Walk(ctx, sm.fs, configDir, func(fp string, info filesystem.FileInfo, err error) error {
			if err != nil {
				return skerr.Wrap(err)
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(fp, ".yaml") && !strings.HasSuffix(fp, ".yml") {
				return nil
			}
			eg.Go(fp, func() error {
				// Download the file contents.
				fullPath := path.Join(configDir, fp)
				oldContents, err := vfs.ReadFile(ctx, sm.fs, fp)
				if err != nil {
					return skerr.Wrapf(err, "failed to retrieve contents of %s", fullPath)
				}
				// Parse the k8s configs from the file.
				k8sConfig, byteRanges, err := k8s_config.ParseK8sConfigFile(oldContents)
				if err != nil {
					return skerr.Wrap(err)
				}
				// Update the images of the containers.
				newContents, err := updateK8sConfig(sf, k8sConfig, byteRanges, oldContents)
				if err != nil {
					return skerr.Wrap(err)
				}
				if !bytes.Equal(oldContents, newContents) {
					changesMtx.Lock()
					changes[fp] = newContents
					changesMtx.Unlock()
				}
				return nil
			})
			return nil
		})
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	if err := eg.Wait(); err != nil {
		return skerr.Wrapf(err, "failed to update configs")
	}
	for path, newContents := range changes {
		if err := vfs.WriteFile(ctx, sm.fs, path, newContents); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

func updateK8sConfig(sf *StageFile, k8sConfig *k8s_config.K8sConfigFile, byteRanges map[interface{}]*k8s_config.ByteRange, oldContents []byte) ([]byte, error) {
	newContents := make([]byte, len(oldContents))
	copy(newContents, oldContents)

	update := func(obj interface{}, name string, podSpec *v1.PodTemplateSpec) error {
		byteRange, ok := byteRanges[obj]
		if !ok {
			return skerr.Fmt("no byte range found!")
		}
		oldSlice := newContents[byteRange.Start:byteRange.End]
		newSlice, err := updatePodSpec(sf, podSpec, oldSlice)
		if err != nil {
			return skerr.Wrapf(err, "updating containers of %s", name)
		}
		contentsPre := newContents[:byteRange.Start]
		contentsPost := newContents[byteRange.End:]
		newContents = append(contentsPre, newSlice...)
		newContents = append(newContents, contentsPost...)
		return nil
	}

	for _, cronJob := range k8sConfig.CronJob {
		if err := update(cronJob, cronJob.Name, &cronJob.Spec.JobTemplate.Spec.Template); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	for _, daemonSet := range k8sConfig.DaemonSet {
		if err := update(daemonSet, daemonSet.Name, &daemonSet.Spec.Template); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	for _, deployment := range k8sConfig.Deployment {
		if err := update(deployment, deployment.Name, &deployment.Spec.Template); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	for _, statefulSet := range k8sConfig.StatefulSet {
		if err := update(statefulSet, statefulSet.Name, &statefulSet.Spec.Template); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return newContents, nil
}

func updatePodSpec(sf *StageFile, podSpec *v1.PodTemplateSpec, contents []byte) ([]byte, error) {
	stagesMap, err := GetStagesFromAnnotations(podSpec.Annotations)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if len(stagesMap) == 0 {
		return contents, nil
	}
	for _, container := range podSpec.Spec.Containers {
		stage, ok := stagesMap[container.Name]
		if !ok {
			continue
		}
		imagePath := strings.Split(container.Image, "@")[0]
		img, ok := sf.Images[imagePath]
		if !ok {
			continue
		}
		stg, ok := img.Stages[stage]
		if !ok {
			continue
		}
		if bytes.Count(contents, []byte(container.Image)) > 1 {
			return nil, skerr.Fmt("found more than one instance of %s", container.Image)
		}
		contents = bytes.Replace(contents, []byte(container.Image), []byte(imagePath+"@"+stg.Digest), 1)
	}
	return contents, nil
}

func GetStagesFromAnnotations(annotations map[string]string) (map[string]string, error) {
	value, ok := annotations[PodAnnotationKey]
	if !ok {
		return map[string]string{}, nil
	}
	pairs := strings.Split(value, ",")
	rv := map[string]string{}
	for _, pair := range pairs {
		splitPair := strings.Split(pair, ":")
		if len(splitPair) != 2 {
			return nil, skerr.Fmt("invalid value for %q annotation; expected \"image:stage[, image:stage]\"", PodAnnotationKey)
		}
		rv[strings.TrimSpace(splitPair[0])] = strings.TrimSpace(splitPair[1])
	}
	return rv, nil
}
