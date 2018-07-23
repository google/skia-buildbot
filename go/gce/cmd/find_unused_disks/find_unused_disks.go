package main

/*
   Find (and optionally delete) unused disks.
*/

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	workdir = flag.String("workdir", ".", "Working directory.")
	zone    = flag.String("zone", gce.ZONE_DEFAULT, "Which GCE zone to use.")
)

func log(str string, args ...interface{}) {
	if _, err := fmt.Fprintf(os.Stdout, str, args...); err != nil {
		sklog.Fatal(err)
	}
}

func logErr(str string, args ...interface{}) {
	if _, err := fmt.Fprintf(os.Stderr, str, args...); err != nil {
		sklog.Fatal(err)
	}
}

func main() {
	common.Init()

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the GCloud object.
	g, err := gce.NewGCloud(gce.PROJECT_ID_SERVER, *zone, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of unused disks.
	disks, err := g.ListDisks()
	if err != nil {
		sklog.Fatal(err)
	}
	unused := make([]*gce.Disk, 0, len(disks))
	for _, d := range disks {
		if len(d.InUseBy) == 0 {
			unused = append(unused, d)
		}
	}

	// Print out the unused disks and give the user the option to delete
	// them.
	if len(unused) > 0 {
		log("Found %d unused disks in zone %s:\n", len(unused), *zone)
		for i, d := range unused {
			log("  %d.\t%s\n", i+1, d.Name)
		}
		delete := make([]string, 0, len(unused))
		log("\nDo you want to delete them?\n")
		for {
			log("'n' to exit without deleting\n")
			log("'y' to delete them all\n")
			log("expression like '1-3,9,10-15' to delete specific disks\n? ")
			r := bufio.NewReader(os.Stdin)
			got, err := r.ReadString('\n')
			if err != nil {
				sklog.Fatal(err)
			}
			got = strings.TrimSpace(got)
			if got == "n" {
				break
			} else if got == "y" {
				for _, d := range unused {
					delete = append(delete, d.Name)
				}
				break
			} else {
				nums, err := util.ParseIntSet(got)
				if err != nil {
					logErr("Invalid choice: %s", err)
					continue
				}
				for _, n := range nums {
					idx := n - 1
					if idx < 0 || idx >= len(unused) {
						logErr("Invalid disk number: %d\n", n)
						continue
					} else {
						delete = append(delete, unused[n-1].Name)
					}
				}
				break
			}
		}
		if len(delete) > 0 {
			for _, name := range delete {
				log("Deleting disk %s...\n", name)
				if err := g.DeleteDisk(name, false); err != nil {
					sklog.Fatal(err)
				}
			}
			log("Successfully deleted %d disks.", len(delete))
		}
	} else {
		log("No unused disks found in zone %s\n", *zone)
	}
}
