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

func main() {
	common.Init()
	defer common.LogPanic()

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
		fmt.Fprintf(os.Stdout, "Found %d unused disks in zone %s:", len(unused), *zone)
		for i, d := range unused {
			fmt.Fprintf(os.Stdout, "  %d.\t%s", i+1, d.Name)
		}
		delete := make([]string, 0, len(unused))
		fmt.Fprintf(os.Stdout, "Do you want to delete them?")
		for {
			fmt.Fprintf(os.Stdout, "'n' to exit without deleting\n")
			fmt.Fprintf(os.Stdout, "'y' to delete them all\n")
			fmt.Fprintf(os.Stdout, "expression like '1-3,9,10-15' to delete specific disks\n? ")
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
					fmt.Fprintf(os.Stderr, "Invalid choice: %s", err)
					continue
				}
				for _, n := range nums {
					idx := n - 1
					if idx < 0 || idx >= len(unused) {
						fmt.Fprintf(os.Stderr, "Invalid disk number: %d\n", n)
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
				fmt.Fprintf(os.Stderr, "Deleting disk %s...", name)
				if err := g.DeleteDisk(name, false); err != nil {
					sklog.Fatal(err)
				}
			}
			fmt.Fprintf(os.Stderr, "Successfully deleted %d disks.", len(delete))
		}
	} else {
		fmt.Fprintf(os.Stdout, "No unused disks found in zone %s\n", *zone)
	}
}
