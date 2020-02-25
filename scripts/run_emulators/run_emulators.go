package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
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
			cmd:  fmt.Sprintf("cockroach start-single-node --insecure --listen-addr=localhost:%%s --store=%s", os.Getenv("TMPDIR")),
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

func killEmulators() error {
	out, err := exec.RunCwd(context.Background(), ".", "ps", "aux")
	if err != nil {
		return err
	}
	lines := strings.Split(out, "\n")
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
				if _, err := exec.RunCwd(context.Background(), ".", "kill", id); err != nil {
					return err
				}
				delete(procs, desc)
			}
		}
	}
	return nil
}

func runEmulator(e emulator) {
	cmd := exec.ParseCommand(fmt.Sprintf(e.cmd, e.port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := exec.Run(context.Background(), &cmd); err != nil {
		sklog.Fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [start|stop]\n", filepath.Base(os.Args[0]))
	os.Exit(1)
}

func main() {
	common.Init()

	if len(os.Args) != 2 {
		usage()
	}
	start := false
	if os.Args[1] == "start" {
		start = true
	} else if os.Args[1] != "stop" {
		usage()
	}
	if err := killEmulators(); err != nil {
		sklog.Fatal(err)
	}
	if start {
		for _, e := range emulators {
			go runEmulator(e)
		}
		time.Sleep(5 * time.Second)
		fmt.Println("Emulators started. Set environment variables as follows:")
		for _, e := range emulators {
			fmt.Println(fmt.Sprintf("export %s=localhost:%s", e.env, e.port))
		}
		select {}
	} else {
		fmt.Println("Emulators stopped. Unset environment variables as follows:")
		for _, e := range emulators {
			fmt.Println(fmt.Sprintf("export %s=", e.env))
		}
	}
}
