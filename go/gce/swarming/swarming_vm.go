package main

/*
   Program for automating creation and setup of Swarming bot VMs.

   Bot numbers should be assigned as follows:
     1-99 (skia-e-gce-0..): Temporary or experimental bots.
     100-499 (skia-e-gce-[1234]..): Linux
       100-199: linux-small
       200-299: linux-medium
       300-399: linux-large
       400-499: linux-skylake
         405-408: linux-amd
     500-699 (skia-e-gce-[56]..): Windows
       500-599: win-medium
       600-699: win-large
     700-999: unassigned
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	kindExternal = "external"
	kindInternal = "internal"
	kindDev      = "dev"
)

var (
	instanceKinds = []string{
		kindExternal,
		kindInternal,
		kindDev,
	}

	validInstanceTypes = []string{
		instance_types.INSTANCE_TYPE_CT,
		instance_types.INSTANCE_TYPE_LINUX_MICRO,
		instance_types.INSTANCE_TYPE_LINUX_SMALL,
		instance_types.INSTANCE_TYPE_LINUX_MEDIUM,
		instance_types.INSTANCE_TYPE_LINUX_LARGE,
		instance_types.INSTANCE_TYPE_LINUX_AMD,
		instance_types.INSTANCE_TYPE_LINUX_SKYLAKE,
		instance_types.INSTANCE_TYPE_WIN_MEDIUM,
		instance_types.INSTANCE_TYPE_WIN_LARGE,
	}

	winInstanceTypes = []string{
		instance_types.INSTANCE_TYPE_WIN_MEDIUM,
		instance_types.INSTANCE_TYPE_WIN_LARGE,
	}

	ansibleDirectory     = filepath.Join("skolo", "ansible")
	linuxAnsiblePlaybook = filepath.Join("switchboard", "linux.yml")
	winAnsiblePlaybook   = filepath.Join("switchboard", "win.yml")
)

type validMachineRange struct {
	min                  int
	max                  int
	kind                 string
	expectedInstanceType string // If empty, any instance type is accepted.
}

func (r validMachineRange) matchesNumberAndKind(n int, kind string) bool {
	return n >= r.min && n <= r.max && kind == r.kind
}

var validMachineRanges = []validMachineRange{
	// skia-e-gce-* and skia-ct-gce-* machines.
	{0, 99, kindExternal, ""}, // Any instance type is accepted, including CT machines.
	{100, 199, kindExternal, instance_types.INSTANCE_TYPE_LINUX_SMALL},
	{200, 299, kindExternal, instance_types.INSTANCE_TYPE_LINUX_MEDIUM},
	{300, 399, kindExternal, instance_types.INSTANCE_TYPE_LINUX_LARGE},
	{400, 404, kindExternal, instance_types.INSTANCE_TYPE_LINUX_SKYLAKE},
	{405, 408, kindExternal, instance_types.INSTANCE_TYPE_LINUX_AMD},
	{409, 499, kindExternal, instance_types.INSTANCE_TYPE_LINUX_SKYLAKE},
	{500, 599, kindExternal, instance_types.INSTANCE_TYPE_WIN_MEDIUM},
	{600, 699, kindExternal, instance_types.INSTANCE_TYPE_WIN_LARGE},

	// skia-i-gce-* machines.
	{100, 199, kindInternal, instance_types.INSTANCE_TYPE_LINUX_SMALL},
	{200, 299, kindInternal, instance_types.INSTANCE_TYPE_LINUX_LARGE},

	// skia-d-gce-* machines.
	{100, 100, kindDev, instance_types.INSTANCE_TYPE_LINUX_SMALL}, // Exactly one machine.
	{101, 599, kindDev, instance_types.INSTANCE_TYPE_LINUX_MICRO},
}

type vmsToCreate struct {
	vms                  []*gce.Instance
	zone                 string
	project              string
	configuredViaAnsible bool
}

func makeVMsToCreate(ctx context.Context, kind, instanceType string, forceInstanceType bool, instanceNums []int) (vmsToCreate, error) {
	if !util.In(kind, instanceKinds) {
		return vmsToCreate{}, skerr.Fmt("Unknown kind: %s", kind)
	}

	retval := vmsToCreate{
		zone:    gce.ZONE_DEFAULT,
		project: gce.PROJECT_ID_SWARMING,
		// CT instances are the only ones that we do not configure via Ansible.
		configuredViaAnsible: instanceType != instance_types.INSTANCE_TYPE_CT,
	}

	var getInstance func(int) *gce.Instance
	switch instanceType {
	case instance_types.INSTANCE_TYPE_CT:
		getInstance = func(num int) *gce.Instance { return instance_types.SkiaCT(num) }
	case instance_types.INSTANCE_TYPE_LINUX_MICRO:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxMicro(num) }
	case instance_types.INSTANCE_TYPE_LINUX_SMALL:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxSmall(num) }
	case instance_types.INSTANCE_TYPE_LINUX_MEDIUM:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxMedium(num) }
	case instance_types.INSTANCE_TYPE_LINUX_LARGE:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxLarge(num) }
	case instance_types.INSTANCE_TYPE_LINUX_AMD:
		retval.zone = gce.ZONE_AMD
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxAmd(num) }
	case instance_types.INSTANCE_TYPE_LINUX_SKYLAKE:
		retval.zone = gce.ZONE_SKYLAKE
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxSkylake(num) }
	case instance_types.INSTANCE_TYPE_WIN_MEDIUM:
		getInstance = func(num int) *gce.Instance {
			instance, err := instance_types.WinMedium(ctx, num)
			if err != nil {
				sklog.Fatal(err)
			}
			return instance
		}
	case instance_types.INSTANCE_TYPE_WIN_LARGE:
		getInstance = func(num int) *gce.Instance {
			instance, err := instance_types.WinLarge(ctx, num)
			if err != nil {
				sklog.Fatal(err)
			}
			return instance
		}
	default:
		// Should never happen.
		panic(fmt.Sprintf("Unknown instanceType: %q. This is a bug.", instanceType))
	}

	if kind == kindInternal {
		retval.project = gce.PROJECT_ID_INTERNAL_SWARMING
		getInstanceInner := getInstance
		getInstance = func(num int) *gce.Instance {
			return instance_types.Internal(getInstanceInner(num))
		}
	} else if kind == kindDev {
		getInstanceInner := getInstance
		getInstance = func(num int) *gce.Instance {
			return instance_types.Dev(getInstanceInner(num))
		}
	}

	// Validate that the given instance type and machine numbers correspond with the per-type range
	// assignment.
	if !forceInstanceType {
		for _, instanceNum := range instanceNums {
			found := false
			for _, r := range validMachineRanges {
				if r.matchesNumberAndKind(instanceNum, kind) {
					found = true
					// Empty string means any instance type is valid.
					if r.expectedInstanceType != "" && instanceType != r.expectedInstanceType {
						return vmsToCreate{}, skerr.Fmt("Machine number %d is expected to be of instance type %s. To force a different type, re-run with --force-type.", instanceNum, r.expectedInstanceType)
					}
					break
				}
			}
			if !found {
				return vmsToCreate{}, skerr.Fmt("Machine number %d is not in any known machine range, and thus its expected instance type cannot be determined. To proceed anyway, re-run with --force-type.", instanceNum)
			}
		}
	}

	// Create the Instance objects.
	for _, num := range instanceNums {
		retval.vms = append(retval.vms, getInstance(num))
	}

	return retval, nil
}

func main() {
	var (
		// Flags.
		instances         = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
		create            = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
		delete            = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
		deleteDataDisk    = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
		dev               = flag.Bool("dev", false, "Whether or not the bots connect to chromium-swarm-dev.")
		dumpJson          = flag.Bool("dump-json", false, "Dump out JSON for each of the instances to create/delete and exit without changing anything.")
		ignoreExists      = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
		instanceType      = flag.String("type", "", fmt.Sprintf("Type of instance; one of: %v", validInstanceTypes))
		forceInstanceType = flag.Bool("force-type", false, "Skip validation of instance types by machine number.")
		internal          = flag.Bool("internal", false, "Whether or not the bots are internal.")
	)

	common.Init()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}
	if !util.In(*instanceType, validInstanceTypes) {
		sklog.Fatalf("--type must be one of %v", validInstanceTypes)
	}
	instanceNums, err := util.ParseIntSet(*instances)
	if err != nil {
		sklog.Fatal(err)
	}
	if len(instanceNums) == 0 {
		sklog.Fatal("Please specify at least one instance number via --instances.")
	}
	if *dev && *internal {
		sklog.Fatalf("At most one of --dev or --internal must be set.")
	}

	kind := kindExternal
	if *internal {
		kind = kindInternal
	}
	if *dev {
		kind = kindDev
	}

	// Get the list of VMs to create.
	ctx := context.Background()
	vmsToCreate, err := makeVMsToCreate(ctx, kind, *instanceType, *forceInstanceType, instanceNums)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the GCloud object.
	g, err := gce.NewLocalGCloud(vmsToCreate.project, vmsToCreate.zone)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := g.CheckSsh(); err != nil {
		sklog.Fatal(err)
	}

	// If requested, dump JSON for the given instances and exit.
	if *dumpJson {
		verb := "create"
		if *delete {
			verb = "delete"
		}
		data := map[string][]*gce.Instance{
			verb: vmsToCreate.vms,
		}
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infof("\n%s", string(b))
		return
	}

	// Perform the requested operation.
	verb := "Creating"
	if *delete {
		verb = "Deleting"
	}
	sklog.Infof("%s instances: %v", verb, instanceNums)
	group := util.NewNamedErrGroup()
	for _, vm := range vmsToCreate.vms {
		vm := vm // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(vm.Name, func() error {
			if *create {
				if err := g.CreateAndSetup(ctx, vm, *ignoreExists); err != nil {
					return err
				}
			} else {
				return g.Delete(vm, *ignoreExists, *deleteDataDisk)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		sklog.Fatal(err)
	}

	// Print out ansible-playbook command if necessary.
	//
	// TODO(lovisolo): Run Ansible playbook unless --skip-ansible-playbook is provided.
	if *create && vmsToCreate.configuredViaAnsible {
		playbook := linuxAnsiblePlaybook
		if util.In(*instanceType, winInstanceTypes) {
			playbook = winAnsiblePlaybook
		}
		var machines []string
		for _, n := range instanceNums {
			machines = append(machines, fmt.Sprintf("skia-e-gce-%03d", n))
		}
		commaSeparatedMachines := strings.Join(machines, ",")
		command := fmt.Sprintf("$ ansible-playbook %s --limit %s", playbook, commaSeparatedMachines)
		if util.In(*instanceType, winInstanceTypes) {
			// For some reason, sometimes passwordless auth does not work on Windows machines.
			command += " --ask-pass"
		}

		sklog.Infof("To finish setting up these machines, cd into %s, then run the following commands:", ansibleDirectory)
		sklog.Infof("$ make update_ssh_gce_config")
		sklog.Infof(command)
		sklog.Infof("Then update hosts.yml to inclue the new machines.")
	}
}
