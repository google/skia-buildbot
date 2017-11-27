// blamer searches through git history and does text searches on the full patch
// text and the commit message. If any matches are found then the commit
// message is displayed.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	num     = flag.Int("num", 10, "The last N commits to search through.")
	match   = flag.String("match", "SkEdgeClipper", "The case-sensitive text to search for.")
	verbose = flag.Bool("verbose", true, "Show the commit message for each match, otherwise just display the git hash.")
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s <flags>\n\n", os.Args[0])
		fmt.Printf(`%s searches through git history and does text searches on the full patch
text and the commit message. If any matches are found then the commit
message is displayed. Must be run within the git repo.

`, os.Args[0])
		flag.PrintDefaults()
	}
}
func main() {
	common.Init()
	ctx := context.Background()
	match_found := false
	for i := 0; i < *num; i++ {
		res, err := exec.RunSimple(ctx, fmt.Sprintf("git show HEAD~%d..HEAD~%d", i+1, i))
		if err != nil {
			sklog.Fatalf("Failed to get the git info: %s", err)
		}
		// Each response begins with "commit 3f61d....ffb2\n", so use
		// that to slice out the git hash.
		parts := strings.SplitN(res, "\n", 2)
		firstLine := strings.Split(parts[0], " ")
		githash := firstLine[1]
		if strings.Index(res, *match) >= 0 {
			match_found = true
			if *verbose {
				msg, err := exec.RunSimple(ctx, fmt.Sprintf("git show --no-patch %s", githash))
				if err != nil {
					sklog.Fatalf("Failed to get commit details: %s", err)
				}
				fmt.Printf("%s\n\n", msg)
			} else {
				fmt.Printf("%s\n", githash)
			}
		}
	}
	if !match_found {
		fmt.Println("No matches found.")
	}
}
