// sheriff_emails is an application that emails the next sheriff every week.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

const (
	// Turn into flags?
	CHECK_DIRTY_COMMITTED_CONFIGS_TIME = 2 * time.Minute

	METRIC_NAME = "alerts_watcher"
)

var (
	// Flags.
	k8sYamlRepo = flag.String("k8_yaml_repo", "https://skia.googlesource.com/skia-public-config", "The repository where K8s yaml files are stored (eg: https://skia.googlesource.com/skia-public-config)")
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir     = flag.String("workdir", ".", "Directory to use for scratch work.")
	promPort    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
)

func checkForDirtyCommittedConfigs(ctx context.Context) {
	// Just check out what you need to here in skia-public-config or skia-corp-config specify via a flag.
	g, err := git.NewCheckout(ctx, *k8sYamlRepo, *workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	if err := g.Update(ctx); err != nil {
		sklog.Fatal(err)
	}

	files, err := ioutil.ReadDir(g.Dir())
	if err != nil {
		sklog.Fatal(err)
	}
	re := regexp.MustCompile(`image: .*-dirty`)
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".yaml" {
			// Only interested in yaml configs.
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(g.Dir(), f.Name()))
		if err != nil {
			sklog.Fatal(err)
		}
		if re.Match(b) {
			// Do your stuff here!
			fmt.Println(f.Name())
		}

		//fmt.Println(f.Name())
	}
}

func main() {
	common.InitWithMust(METRIC_NAME, common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	for range time.Tick(time.Duration(CHECK_DIRTY_COMMITTED_CONFIGS_TIME)) {
		checkForDirtyCommittedConfigs(ctx)
	}
}
