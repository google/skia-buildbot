package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/git-cas/go/git_cas"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
)

func readIsolate(ctx context.Context, root, isolate string) ([]string, error) {
	script := `
import ast
import os
import sys

def load_isolate(root, isolate):
  base = os.path.dirname(os.path.relpath(isolate, root))
  with open(isolate) as f:
    content = ast.literal_eval(f.read())
  files = set()
  for f in content.get('variables', {}).get('files', []):
    files.add(os.path.normpath(os.path.join(base, f)))
  for inc in content.get('includes', []):
    for f in load_isolate(root, os.path.join(base, inc)):
      files.add(f)
  return sorted(files)

print('\n'.join(load_isolate(sys.argv[1], sys.argv[2])))`
	out, err := exec.RunCwd(ctx, root, "python", "-c", script, root, isolate)
	if err != nil {
		return nil, err
	}
	items := strings.Split(strings.TrimSpace(out), "\n")
	rv := make([]string, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item, "..") {
			rv = append(rv, item)
		}
	}
	return rv, nil
}

func main() {
	// Global flags.
	var gitDir string
	var target string

	// Subcommands.
	cmdUp := flag.NewFlagSet("upload", flag.ExitOnError)
	cmdDown := flag.NewFlagSet("download", flag.ExitOnError)
	var treeHash string
	cmdDown.StringVar(&treeHash, "tree-hash", "", "Tree hash to download.")
	cmdIsolate := flag.NewFlagSet("isolate", flag.ExitOnError)
	var isolateFile string
	cmdIsolate.StringVar(&isolateFile, "isolate-file", "", "Isolate file indicating what to upload.")
	for _, fs := range []*flag.FlagSet{cmdUp, cmdDown, cmdIsolate} {
		fs.StringVar(&gitDir, "git-dir", "", "Path to the git repo to use.")
		fs.StringVar(&target, "target", "", "Path to the target directory.")
	}

	// Find the first arg which doesn't start with "-" and use that as the
	// sub-command.
	var cmd string
	args := make([]string, 0, len(os.Args))
	for _, arg := range os.Args[1:] {
		if cmd == "" && !strings.HasPrefix(arg, "-") {
			cmd = arg
		} else {
			args = append(args, arg)
		}
	}
	var err error
	switch cmd {
	case "upload":
		err = cmdUp.Parse(args)
	case "download":
		err = cmdDown.Parse(args)
	case "isolate":
		err = cmdIsolate.Parse(args)
	default:
		log.Fatal(`Usage:
  upload: Upload a directory.
  download: Download a directory.
  isolate: Upload files and directories specified by an isolate file.
`)
	}
	if err != nil {
		log.Fatal(err)
	}

	if gitDir == "" {
		log.Fatal("--git-dir is required.")
	}
	gitDir, err = filepath.Abs(gitDir)
	if err != nil {
		log.Fatal(err)
	}
	if target == "" {
		log.Fatal("--target is required.")
	}
	target, err = filepath.Abs(target)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	repo := &git.Repo{git.GitDir(gitDir)}
	if cmdUp.Parsed() {
		treeHash, err = git_cas.Upload(ctx, repo, target)
	} else if cmdDown.Parsed() {
		if treeHash == "" {
			log.Fatal("--tree-hash is required.")
		}
		err = git_cas.Download(ctx, repo, target, treeHash)
	} else if cmdIsolate.Parsed() {
		if isolateFile == "" {
			log.Fatal("--isolate-file is required.")
		}
		items, err := readIsolate(ctx, target, isolateFile)
		if err != nil {
			log.Fatal(err)
		}
		treeHash, err = git_cas.UploadItems(ctx, repo, target, items)
	} else {
		err = errors.New("No command to run!")
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(treeHash)
}
