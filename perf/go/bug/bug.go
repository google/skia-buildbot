// bug is a package for handling bug reporting URLs.
package bug

import (
	"go.skia.org/infra/go/sklog"
	perfgit "go.skia.org/infra/perf/go/git"
	"gopkg.in/olivere/elastic.v5/uritemplates"
)

// Expand the uriTemplate given a link to the regressing cluster, the commit, and the user's message about the regression.
func Expand(uriTemplate string, clusterLink string, c perfgit.Commit, message string) string {
	expansion := map[string]string{
		"cluster_url": clusterLink,
		"commit_url":  c.URL,
		"message":     message,
	}
	url, err := uritemplates.Expand(uriTemplate, expansion)
	if err != nil {
		sklog.Errorf("Failed to create bug reporting URL: %s", err)
	}
	return url
}

// ExampleExpand expands the given uriTemplate with example data.
func ExampleExpand(uriTemplate string) string {
	c := perfgit.Commit{
		URL: "https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664",
	}
	clusterLink := "https://perf.skia.org/t/?begin=1498332791&end=1498528391&subset=flagged"
	message := "Looks like a regression."
	return Expand(uriTemplate, clusterLink, c, message)
}
