package git_cas

/*
	Package git_cas provides a git-based content-addressed file storage
	system based on git. It uses a local repo as a cache, downloading files
	from the remote when necessary.

	TODO(borenet): Consider not shelling out to git and instead using
	something like https://github.com/src-d/go-git.
*/

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// Refs are named for the tree hash they point to. They only exist
	// because we can't directly fetch a particular tree hash from a remote.
	refTmpl = "refs/content/%s"
	// For now, use "origin" for the name of the remote. This may change,
	// for example if we decide to use the CAS repo to house a "real" repo
	// as well.
	remoteName = "origin"
	// Tree hash for an empty dir.
	emptyHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
)

// GitCAS implements a content-addressed storage system which uses git. It
// maintains a local repo as a cache.
type GitCAS struct {
	cache *git.Repo
}

// New returns a GitCAS instance, ensuring that a local copy of the given repo
// URL exists at the given path, syncing it if necessary. Note that, because the
// content-addessed refs are outside of refs/heads, InitLocalCache does not
// actually warm the cache. This is generally desirable, because the remote repo
// may contain a large number of objects which are not needed locally.
func New(ctx context.Context, path, repoUrl string) (*GitCAS, error) {
	repo := &git.Repo{GitDir: git.GitDir(path)}

	// Create the cache dir if it doesn't exist.
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, skerr.Wrap(err)
		}
	} else if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Ensure that we're tracking the correct remote repo.
	out, err := repo.Git(ctx, "remote", "get-url", remoteName)
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			// The directory exists but is not a git repo. Ensure
			// that it's empty and initialize the repo.
			contents, err := ioutil.ReadDir(path)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if len(contents) > 0 {
				return nil, skerr.Fmt("%s exists but is not empty!", path)
			}
			if _, err := repo.Git(ctx, "--bare", "init"); err != nil {
				return nil, skerr.Wrap(err)
			}
			if _, err := repo.Git(ctx, "remote", "add", remoteName, repoUrl); err != nil {
				return nil, skerr.Wrap(err)
			}
		} else {
			return nil, skerr.Wrap(err)
		}
	} else {
		// The directory exists and contains a git repo. Ensure that
		// it's the one we want.
		actual, err := git.NormalizeURL(strings.TrimSpace(out))
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		expect, err := git.NormalizeURL(strings.TrimSpace(repoUrl))
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if actual != expect {
			return nil, skerr.Fmt("Repo already exists at %s but is configured to fetch from %s!", path, actual)
		}
		out, err = repo.Git(ctx, "rev-parse", "--is-bare-repository")
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if strings.TrimSpace(out) != "true" {
			return nil, skerr.Fmt("Repo at %s is not bare!", path)
		}
	}
	return &GitCAS{
		cache: repo,
	}, nil
}

// git is a helper function for running git commands on the local cache.
func (s *GitCAS) git(ctx context.Context, args ...string) (string, error) {
	return s.cache.Git(ctx, args...)
}

// Prune the local cache, evict no-longer-referenced objects.
func (s *GitCAS) Prune(ctx context.Context) error {
	if _, err := s.git(ctx, "remote", "update", "--prune"); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := s.git(ctx, "gc"); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func ref(treeHash string) string {
	return fmt.Sprintf(refTmpl, treeHash)
}

func workTree(dir string) string {
	return fmt.Sprintf("--work-tree=%s", dir)
}

// Download the given treeHash to the given directory.
func (s *GitCAS) Download(ctx context.Context, destDir, treeHash string) error {
	st, err := os.Stat(destDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
			return skerr.Wrap(err)
		}
	} else if err != nil {
		return skerr.Wrap(err)
	} else if !st.IsDir() {
		return skerr.Fmt("Destination %q must be a directory.", destDir)
	}
	if treeHash == emptyHash {
		return nil
	}
	if _, err := s.git(ctx, "fetch", remoteName, ref(treeHash)); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := s.git(ctx, workTree(destDir), "checkout", "FETCH_HEAD", "--", "."); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Upload the given target. If it is a directory, upload its contents, not the
// directory itself. Empty directories will be skipped.
func (s *GitCAS) Upload(ctx context.Context, target string) (string, error) {
	st, err := os.Stat(target)
	if err != nil {
		return "", nil
	}
	var item string
	if st.IsDir() {
		item = "."
	} else {
		item = filepath.Base(target)
		target = filepath.Dir(target)
	}
	return s.UploadItems(ctx, target, []string{item})
}

// UploadItems uploads the specified items from the given directory. They must
// be relative paths inside srcDir. Empty directories will be skipped.
func (s *GitCAS) UploadItems(ctx context.Context, srcDir string, items []string) (string, error) {
	g := func(args ...string) (string, error) {
		return s.git(ctx, append([]string{workTree(srcDir)}, args...)...)
	}
	branch := fmt.Sprintf("tmp-%s", uuid.New().String())
	if _, err := g("checkout", "--orphan", branch); err != nil {
		return "", err
	}
	defer func() {
		_, err := g("branch", "-D", branch)
		if err != nil {
			sklog.Errorf("Failed to delete temporary branch %q: %s", branch, err)
		}
	}()
	// Clear out any cruft.
	if _, err := g("reset"); err != nil {
		return "", err
	}
	// Add the requested items.
	for _, item := range items {
		if strings.HasPrefix(item, "..") {
			return "", skerr.Fmt("Items must be contained inside the target directory, not %s", item)
		}
		if _, err := g("add", "-f", item); err != nil {
			return "", err
		}
	}
	// Commit.
	if _, err := g("commit", "--allow-empty", "--no-verify", "-m", "blah blah"); err != nil {
		return "", err
	}
	// Obtain the tree hash of the commit.
	tree, err := g("rev-parse", fmt.Sprintf("%s^{tree}", branch))
	if err != nil {
		return "", err
	}
	tree = strings.TrimSpace(tree)
	// If the remote ref doesn't exist, git prints no output but exits with
	// zero code.
	remote, err := g("ls-remote", remoteName, ref(tree))
	if err != nil {
		return "", err
	}
	if strings.Contains(remote, tree) {
		fmt.Println(fmt.Sprintf("Already exists: %s", ref(tree)))
	} else {
		if _, err := g("push", remoteName, fmt.Sprintf("%s:%s", branch, ref(tree))); err != nil {
			return "", err
		}
	}
	return tree, nil
}
