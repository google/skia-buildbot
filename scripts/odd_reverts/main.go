// Run in a git repo it will print out all the hashes of git commits
// that contain an odd number of occurences of the word "Revert".
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

var (
	since = flag.String("since", "6months", "How far back to search in git history.")
	re    = regexp.MustCompile("Revert")
)

func main() {
	defer common.LogPanic()
	common.Init()

	output := bytes.Buffer{}
	ctx := context.Background()
	err := exec.Run(ctx, &exec.Command{
		Name:           "git",
		Args:           []string{"log", "--format=oneline", fmt.Sprintf("--since=%s", *since)},
		CombinedOutput: &output,
	})
	if err != nil {
		sklog.Fatal(err)
	}
	lines := strings.Split(output.String(), "\n")
	for _, line := range lines {
		numReverts := len(re.FindAllString(line, -1))
		if numReverts%2 == 1 {
			sklog.Info(line)
			fmt.Println(strings.Split(line, " ")[0])
		}
	}
}
