package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
)

var (
	procsToKill = []*regexp.Regexp{
		regexp.MustCompile("[g]cloud\\.py"),
		regexp.MustCompile("[c]loud_datastore_emulator"),
		regexp.MustCompile("[C]loudDatastore.jar"),
		regexp.MustCompile("[c]btemulator"),
		regexp.MustCompile("[c]loud-pubsub-emulator"),
		regexp.MustCompile("[c]loud-firestore-emulator"),
		regexp.MustCompile("[c]ockroach"),
	}

	// The ports below should be kept in sync with the run_emulators script.
	emulators = []emulator{
		{
			cmd:  "gcloud beta emulators datastore start --no-store-on-disk --host-port=localhost:%s --project=test-project",
			env:  "DATASTORE_EMULATOR_HOST",
			port: "8891",
		},
		{
			cmd:  "gcloud beta emulators bigtable start --host-port=localhost:%s --project=test-project",
			env:  "BIGTABLE_EMULATOR_HOST",
			port: "8892",
		},
		{
			cmd:  "gcloud beta emulators pubsub start --host-port=localhost:%s --project=test-project",
			env:  "PUBSUB_EMULATOR_HOST",
			port: "8893",
		},
		{
			cmd:  "gcloud beta emulators firestore start --host-port=localhost:%s",
			env:  "FIRESTORE_EMULATOR_HOST",
			port: "8894",
		},
		{
			cmd:  fmt.Sprintf("cockroach start-single-node --insecure --listen-addr=localhost:%%s --store=%s", os.TempDir()),
			env:  "COCKROACHDB_EMULATOR_HOST",
			port: "8895",
		},
	}
)

type emulator struct {
	cmd  string
	env  string
	port string
}

func killEmulators(cmds []*exec.Cmd) error {
	// Start by directly killing any passed-in processes.
	for _, cmd := range cmds {
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
	}
	// Now, find any processes not started by this process (eg. by a previous
	// invocation) and kill them.
	out, err := exec.Command("ps", "aux").CombinedOutput()
	if err != nil {
		return err
	}
	lines := strings.Split(string(out), "\n")
	procs := make(map[string]string, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		procs[line] = fields[1]
	}
	for _, re := range procsToKill {
		for desc, id := range procs {
			if re.MatchString(desc) {
				if err := exec.Command("kill", id).Run(); err != nil {
					return err
				}
				delete(procs, desc)
			}
		}
	}
	fmt.Println("Emulators stopped. Unset environment variables as follows:")
	for _, e := range emulators {
		fmt.Println(fmt.Sprintf("export %s=", e.env))
	}
	return nil
}

func startEmulator(e emulator) (*exec.Cmd, error) {
	split := strings.Split(fmt.Sprintf(e.cmd, e.port), " ")
	cmd := exec.Command(split[0], split[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Allow the subprocess to live longer than this process.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [start|stop]\n", filepath.Base(os.Args[0]))
	os.Exit(1)
}

func main() {
	common.Init()

	start := false
	wait := false
	if len(os.Args) == 1 {
		start = true
		wait = true
	} else if len(os.Args) == 2 {
		if os.Args[1] == "start" {
			start = true
		} else if os.Args[1] != "stop" {
			usage()
		}
	} else {
		usage()
	}
	if err := killEmulators(nil); err != nil {
		log.Fatal(err)
	}
	if start {
		cmds := make([]*exec.Cmd, 0, len(emulators))
		for _, e := range emulators {
			cmd, err := startEmulator(e)
			if err != nil {
				// TODO(borenet): Should we kill any emulators we started?
				log.Fatal(err)
			}
			cmds = append(cmds, cmd)
		}
		time.Sleep(5 * time.Second)
		fmt.Println("Emulators started. Set environment variables as follows:")
		for _, e := range emulators {
			fmt.Println(fmt.Sprintf("export %s=localhost:%s", e.env, e.port))
		}
		if wait {
			cleanup.AtExit(func() {
				if err := killEmulators(cmds); err != nil {
					log.Fatal(err)
				}
			})
			fmt.Println("Waiting for ")
			select {}
		}
	}
}
