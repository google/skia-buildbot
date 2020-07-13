package config

import "time"

const (
	// REFRESH is the duration between git refreshes, and cleanup checks for closed issues.
	REFRESH = 15 * time.Minute

	// REPO_SUBDIR is the sub-directory in the git repo where we look for Markdown documents to serve.
	REPO_SUBDIR = "site"
)

var (
	// ALLOWED_DOMAINS is the list of domains that are allowed to have their CLs previewed.
	ALLOWED_DOMAINS = []string{"google.com", "chromium.org", "skia.org"}
)
