package main

// This script will clean up files in GCS that are older
// than the specified date.
import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	os_exec "os/exec"
	"regexp"
	"sync"
	"time"

	"go.skia.org/infra/go/exec"
)

var (
	bucket    = flag.String("bucket", "", "[REQUIRED] The GCP bucket to clean up.")
	prefix    = flag.String("prefix", "", "The prefix (directory) to clean in")
	olderThan = flag.String("older_than", "", "[REQUIRED] Date in YYYY-MM-DD of the oldest file to keep.")
	dryRun    = flag.Bool("dry_run", false, "Print out those files that would be deleted instead of actually deleting them.")

	deleteThreads = flag.Int("delete_threads", 16, "How many files to simultaneously delete")
)

const YMD_FORMAT = "2006-01-02"
const NO_MORE_FILES = "NO_MORE_FILES"

var fileLine = regexp.MustCompile(`^\s+\d+\s+(?P<date>\S+)\s+(?P<file>\S+)`)

func main() {
	flag.Parse()
	if *bucket == "" || *olderThan == "" {
		fmt.Println("--bucket and --older_than are required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	oldestDate, err := time.ParseInLocation(YMD_FORMAT, *olderThan, time.UTC)
	if err != nil {
		fmt.Println("Date must be in YYYY-MM-DD format")
		os.Exit(1)
	}

	search := fmt.Sprintf("%s/%s", *bucket, *prefix)
	fmt.Printf("Searching for files in %s\n", search)

	files := make(chan string, 10000)
	wg := sync.WaitGroup{}
	wg.Add(*deleteThreads)
	for i := 0; i < *deleteThreads; i++ {
		go deleteHelper(files, &wg)
	}

	cmd := os_exec.Command("gsutil", "ls", "-r", "-l", search)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if match := fileLine.FindStringSubmatch(line); match != nil {
			// match[1] is the date, formatted like 2016-12-08T05:00:29Z
			// match[2] is the file name
			d, err := time.ParseInLocation(time.RFC3339, match[1], time.UTC)
			if err != nil {
				fmt.Printf("Invalid date in line %s, %s\n", line, err)
				continue
			}
			if d.Before(oldestDate) {
				files <- match[2]
			}
		}
	}
	if err = cmd.Wait(); err != nil {
		fmt.Printf("Listing failed, going to finish deleting files: %s\n", err)
	}
	fmt.Printf("Enumerated all files, waiting to delete %d more files\n", len(files))
	for i := 0; i < *deleteThreads; i++ {
		files <- NO_MORE_FILES
	}
	wg.Wait()
	fmt.Println("done")
}

func deleteHelper(files chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		toDelete := <-files
		if toDelete == NO_MORE_FILES {
			return
		}
		if *dryRun {
			fmt.Printf("dry deleted %s\n", toDelete)
		} else {
			err := exec.Run(context.Background(), &exec.Command{
				Name: "gsutil",
				Args: []string{"rm", toDelete},
			})
			if err != nil {
				fmt.Printf("Could not delete %s: %s\n", toDelete, err)
			} else {
				fmt.Printf("Deleted %s\n", toDelete)
			}
		}
	}
}
