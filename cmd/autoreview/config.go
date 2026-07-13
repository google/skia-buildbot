package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	project      = "skia-infra-corp"
	location     = "global"
	model        = "gemini-3.5-flash"
	baseCommit   = "HEAD~1"
	contextLines = 10
)

type Config struct {
	GCPProject   string
	Location     string
	Model        string
	BaseCommit   string
	ContextLines int
	Timeout      time.Duration
	Verbose      bool
	ShowWarnings bool
	ShowLGTM     bool
}

func Parse() (*Config, error) {
	fs := flag.NewFlagSet("autoreview", flag.ExitOnError)

	verbose := fs.Bool(
		"verbose",
		false,
		"Print extra information e.g. full prompt",
	)
	showWarnings := fs.Bool(
		"show-warnings",
		true,
		"Display warnings e.g. untracked git files",
	)
	showLGTM := fs.Bool(
		"show-lgtm",
		true,
		"Display the review results if LGTM",
	)
	baseCommit := fs.String(
		"base-commit",
		baseCommit,
		"Base commit to compare current change against",
	)
	contextLines := fs.Int(
		"context-lines",
		contextLines,
		"Generate diffs with <n> lines of context",
	)
	model := fs.String(
		"model",
		model,
		"Gemini model name to use for making review",
	)
	gcpProject := fs.String(
		"project",
		project,
		"GCP project to use for Gemini API billing",
	)
	locationFlag := fs.String(
		"location",
		location,
		"Vertex AI location/region",
	)
	timeout := fs.Duration(
		"timeout",
		time.Minute,
		"Context timeout for the tool run",
	)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(
			os.Stderr,
			"AI-powered code review tool. It analyzes the latest commit using Google's\n"+
				"Gemini API and provides a summary and an LGTM or Action Required status.\n\n"+
				"Important considerations:\n"+
				"- This is an AI tool. The result is just an AI opinion. Taking action based\n"+
				"  on that opinion is up to the user.\n"+
				"- To make the tool work, you must authenticate and grant permissions via the\n"+
				"  gcloud cli.\n"+
				"- Git untracked files are not supported. You should commit them or make\n"+
				"  them trackable first.\n\n",
		)
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	return &Config{
		GCPProject:   *gcpProject,
		Location:     *locationFlag,
		Model:        *model,
		BaseCommit:   *baseCommit,
		ContextLines: *contextLines,
		Timeout:      *timeout,
		Verbose:      *verbose,
		ShowWarnings: *showWarnings,
		ShowLGTM:     *showLGTM,
	}, nil
}
