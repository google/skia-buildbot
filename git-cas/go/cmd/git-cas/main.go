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
	"go.skia.org/infra/git-cas/go/isolate"
	"go.skia.org/infra/go/git"
)

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
	// subcommand.
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
		err = errors.New(`Usage:
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
	repo := &git.Repo{GitDir: git.GitDir(gitDir)}
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
		items, err := isolate.Read(ctx, target, isolateFile)
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
