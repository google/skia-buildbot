package common

import "fmt"

const (
	REPO_URL_TEMPLATE = "https://skia.googlesource.com/%s-config"
	REPO_BASE_DIR     = "/tmp"
	REPO_DIR_TEMPLATE = "/tmp/%s-config"
)

// ToFullRepoURL converts the project name into a git repo URL.
func ToFullRepoURL(s string) string {
	return fmt.Sprintf(REPO_URL_TEMPLATE, s)

}

// ToRepoDir converts the project name into a git repo directory name.
func ToRepoDir(s string) string {
	return fmt.Sprintf(REPO_DIR_TEMPLATE, s)
}
