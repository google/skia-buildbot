package main

// This script will clear out Docker images in gcr.io that are older than the
// specified date, leaving at least min_images remaining. The min_images
// supersedes the age.

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
)

var (
	project   = flag.String("project", "", "[REQUIRED] The GCP project to clean up images.")
	olderThan = flag.String("older_than", "", "[REQUIRED] Date in YYYY-MM-DD of newest image to get rid of.")
	minImages = flag.Int("min_images", 10, "Minimum number of images to keep around, ignoring age.")
	dryRun    = flag.Bool("dry_run", false, "Print out those images that would be deleted instead of actually deleting them.")
)

const YMD_FORMAT = "2006-01-02"

func main() {
	flag.Parse()
	if *project == "" || *olderThan == "" {
		fmt.Println("--project and --older_than are required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	oldestDate, err := time.ParseInLocation(YMD_FORMAT, *olderThan, time.UTC)
	if err != nil {
		fmt.Println("Date must be in YYYY-MM-DD format")
		os.Exit(1)
	}

	fmt.Println("Fetching images in project")
	output := bytes.Buffer{}
	err = exec.Run(context.Background(), &exec.Command{
		Name:   "gcloud",
		Args:   []string{"--project", *project, "container", "images", "list"},
		Stdout: &output,
	})
	if err != nil {
		fmt.Printf("Could not retrieve images: %s\n", err)
		os.Exit(1)
	}

	images := strings.Split(strings.TrimSpace(output.String()), "\n")
	fmt.Printf("Found %d images\n", len(images))
	fmt.Println("Deleting old images")

	for _, image := range images {
		if !strings.HasPrefix(image, "gcr.io") {
			// Skip the header and any footer
			continue
		}
		olderThan := oldestDate

		output := bytes.Buffer{}
		err = exec.Run(context.Background(), &exec.Command{
			Name: "gcloud",
			Args: []string{"--project", *project, "container", "images", "list-tags",
				image, "--sort-by=~TIMESTAMP", fmt.Sprintf("--limit=%d", *minImages),
				"--format=csv(timestamp.year,timestamp.month,timestamp.day)"},
			Stdout: &output,
		})
		if err != nil {
			fmt.Printf("Could not retrieve image tags for %s: %s\n", image, err)
			os.Exit(1)
		}
		// output now looks like
		//year,month,day
		// 2019,1,3
		// 2019,1,2
		// 2018,12,28

		// trim off the header
		newestDates := strings.Split(strings.TrimSpace(output.String()), "\n")[1:]
		if len(newestDates) < *minImages {
			fmt.Printf("%s has fewer than %d tags (%d), skipping\n", image, *minImages, len(newestDates))
			continue
		}
		// Look at when nth newest image, if it is before the min timeline, we use
		// 1 day before that nth newest image, just to be safe.
		nthNewest := newestDates[len(newestDates)-1]
		ymd := strings.Split(nthNewest, ",")
		if altDate := time.Date(safeAtoI(ymd[0]), time.Month(safeAtoI(ymd[1])), safeAtoI(ymd[2]), 0, 0, 0, 0, time.UTC); altDate.Equal(olderThan) || altDate.Before(olderThan) {
			olderThan = altDate.AddDate(0, 0, -1)
		}

		output = bytes.Buffer{}
		err = exec.Run(context.Background(), &exec.Command{
			Name: "gcloud",
			Args: []string{"--project", *project, "container", "images", "list-tags",
				image, "--sort-by=TIMESTAMP", "--limit=999999",
				"--filter=timestamp.datetime < " + olderThan.Format(YMD_FORMAT),
				"--format=get(digest)"},
			Stdout: &output,
		})
		if err != nil {
			fmt.Printf("Could not retrieve image tags for %s: %s\n", image, err)
			os.Exit(1)
		}
		toDelete := strings.Split(strings.TrimSpace(output.String()), "\n")

		fmt.Printf("=== Will delete %d containers from %s\n", len(toDelete), image)

		for _, digest := range toDelete {
			i := fmt.Sprintf("%s@%s", image, digest)
			if *dryRun {
				fmt.Println("dry run delete ", i)
			} else {
				err = exec.Run(context.Background(), &exec.Command{
					Name: "gcloud",
					Args: []string{"--project", *project, "container", "images", "delete",
						"--quiet", "--force-delete-tags",
						i},
					LogStderr: true,
					LogStdout: true,
				})
				if err != nil {
					fmt.Printf("error while deleting %s, continuing anyway: %s\n", i, err)
				} else {
					fmt.Println("deleted ", i)
				}
			}
		}
	}
}

func safeAtoI(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatal(err)
	}
	return i
}
