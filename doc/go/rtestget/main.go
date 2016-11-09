package main

import (
	"fmt"
	"os"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
)

func main() {
	client := httputils.NewTimeoutClient()
	g, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", client)
	info, err := g.GetIssueProperties(4546)
	if err != nil {
		fmt.Printf("Failed to get: %s", err)
		os.Exit(1)
	}
	fmt.Printf("%#v\n", *info)
	for k, v := range info.Revisions {
		fmt.Printf("%q = %#v\n", k, v)
	}
}
