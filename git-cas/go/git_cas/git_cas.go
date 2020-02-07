package git_cas

// TODO(borenet): Consider not shelling out to git and instead using something
// like https://github.com/src-d/go-git.

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

const (
	REF_TMPL = "refs/intermediates/%s"
	// Tree hash for an empty dir.
	EMPTY_HASH = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
)

func ref(treeHash string) string {
	return fmt.Sprintf(REF_TMPL, treeHash)
}

func workTree(dir string) string {
	return fmt.Sprintf("--work-tree=%s", dir)
}

func Download(ctx context.Context, repo *git.Repo, destDir, treeHash string) error {
	if treeHash == EMPTY_HASH {
		return nil
	}
	if _, err := repo.Git(ctx, "fetch", "origin", ref(treeHash)); err != nil {
		return err
	}
	if _, err := repo.Git(ctx, workTree(destDir), "checkout", "FETCH_HEAD", "--", "."); err != nil {
		return err
	}
	return nil
}

func Upload(ctx context.Context, repo *git.Repo, srcDir string) (string, error) {
	return UploadItems(ctx, repo, srcDir, []string{"."})
}

// UploadItems uploads the specified items.
func UploadItems(ctx context.Context, repo *git.Repo, srcDir string, items []string) (string, error) {
	g := func(args ...string) (string, error) {
		return repo.Git(ctx, append([]string{workTree(srcDir)}, args...)...)
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
	remote, err := g("ls-remote", "origin", ref(tree))
	if err != nil {
		return "", err
	}
	if strings.Contains(remote, tree) {
		fmt.Println(fmt.Sprintf("Already exists: %s", ref(tree)))
	} else {
		if _, err := g("push", "origin", fmt.Sprintf("%s:%s", branch, ref(tree))); err != nil {
			return "", err
		}
	}
	return tree, nil
}
