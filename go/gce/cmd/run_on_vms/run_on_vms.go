package main

/*
   Run a command via SSH on all instances.
*/

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"regexp"
	"strings"
	"sync"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/workerpool"
	compute "google.golang.org/api/compute/v0.alpha"
)

var (
	command        = flag.String("cmd", "", "Command to run.")
	instanceRe     = flag.String("instance", ".*", "Instance name or regular expression to match instance names.")
	outfile        = flag.String("out_file", "", "File to write results, in JSON format. If provided, no output will be printed.")
	showSuccessful = flag.Bool("show_successful", false, "Show output of successful commands, in addition to failed commands. Only valid if --out_file is not specified.")
	workdir        = flag.String("workdir", ".", "Working directory to use.")
)

// result is a struct used for collecting results of a command run on many
// instances. Typically only one of Output or Error is used, since Error usually
// includes the output of the command.
type result struct {
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// RunOnInstances runs the given command on all instances matching the given
// regular expression. Returns the map of results, keyed by zone then instance
// name. Only returns an error on critical failures, like failure to create the
// API client or failure to retrieve the list of instances.
func RunOnInstances(re *regexp.Regexp, cmd []string) (map[string]map[string]*result, error) {
	results := map[string]map[string]*result{}
	pool := workerpool.New(50)
	for _, zone := range gce.VALID_ZONES {
		g, err := gce.NewGCloud(zone, *workdir)
		if err != nil {
			return nil, err
		}
		s := g.Service()
		call := s.Instances.List(gce.PROJECT_ID, zone)
		instances := []string{}
		if err := call.Pages(context.Background(), func(list *compute.InstanceList) error {
			for _, i := range list.Items {
				if re.MatchString(i.Name) {
					instances = append(instances, i.Name)
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
		results[zone] = map[string]*result{}
		mtx := sync.Mutex{}
		for _, i := range instances {
			// Alias these vars to prevent them changing from under us.
			instance := i
			z := zone
			pool.Go(func() {
				// TODO(borenet): We can't determine the OS using the API. We will
				// fail to SSH into Windows instances.
				vm := &gce.Instance{
					Name: instance,
					Os:   gce.OS_LINUX,
					User: "default",
				}
				out, err := g.Ssh(vm, cmd...)
				res := &result{}
				if err != nil {
					res.Error = err.Error()
				} else {
					res.Output = out
				}
				mtx.Lock()
				defer mtx.Unlock()
				results[z][instance] = res
			})
		}
	}
	pool.Wait()
	return results, nil
}

func main() {
	common.Init()
	if *command == "" {
		sklog.Fatal("--cmd is required.")
	}
	cmd := strings.Split(*command, " ")
	re := regexp.MustCompile(*instanceRe)

	if *outfile == "" {
		sklog.Fatal("--out_file is required.")
	}

	results, err := RunOnInstances(re, cmd)
	if err != nil {
		sklog.Fatal(err)
	}
	if *outfile != "" {
		if err := util.WithWriteFile(*outfile, func(w io.Writer) error {
			e := json.NewEncoder(w)
			e.SetIndent("", "  ")
			return e.Encode(results)
		}); err != nil {
			sklog.Fatal(err)
		}
	} else {
		for zone, byZone := range results {
			if len(byZone) > 0 {
				sklog.Infof("Zone: %s", zone)
				for instance, result := range byZone {
					if result.Error != "" {
						sklog.Infof("%s (FAILED):\t%s", instance, result.Error)
					} else if *showSuccessful {
						sklog.Infof("%s:\t%s", instance, result.Output)
					}
				}
			}
		}
	}
}
