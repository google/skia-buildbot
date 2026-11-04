// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
)

const successCode = 0
const blockerCode = 1
const errorCode = 2

func printError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

func printAuthHelp(project string) {
	fmt.Fprintf(
		os.Stderr,
		"Make sure you are authenticated with Google Cloud by running "+
			"following command:\n\n",
	)
	fmt.Fprintf(
		os.Stderr,
		"  gcloud auth application-default set-quota-project %s\n",
		project,
	)
	fmt.Fprintf(
		os.Stderr,
		"  gcloud auth application-default login\n\n",
	)
}

func printUntrackedFiles(ctx context.Context) {
	untracked, err := untrackedFiles(ctx)
	if err == nil && len(untracked) > 0 {
		fmt.Println("\nWarning: The following untracked files were ignored:")
		for _, file := range untracked {
			fmt.Printf("  - %s\n", file)
		}
	}
}

func printReview(review string) {
	fmt.Printf("\n\nAI Code Review:\n\n%s\n", review)
}

func reviewCode(ctx context.Context, cfg *Config) int {
	patch, err := fetchPatch(ctx, cfg.BaseCommit, cfg.ContextLines)
	if err != nil {
		printError(err)
		return errorCode
	}

	reviewPrompt := createReviewPrompt(patch)

	if cfg.Verbose {
		fmt.Printf(
			"=== REVIEW PROMPT START ===\n\n%s\n\n=== REVIEW PROMPT END ===\n\n",
			reviewPrompt,
		)
	}

	client := GeminiClient{Project: cfg.GCPProject, Location: cfg.Location, Model: cfg.Model}
	review, err := client.generate(ctx, reviewPrompt)
	if err != nil {
		printError(err)
		printAuthHelp(cfg.GCPProject)
		return errorCode
	}

	var returnCode int
	lgtm, lgtmErr := parseLgtm(review)
	if lgtmErr != nil {
		returnCode = errorCode
		printReview(review)
		fmt.Printf("\n⚠️ Could not parse LGTM status:\n%v\n\n", lgtmErr)
	} else if !lgtm {
		returnCode = blockerCode
		printReview(review)
		fmt.Printf("\nAutoreview: 🔴 Blocker\n\n")
	} else {
		returnCode = successCode
		if cfg.ShowLGTM {
			printReview(review)
			fmt.Printf("\nAutoreview: ✅ LGTM\n\n")
		}
	}

	if cfg.ShowWarnings {
		printUntrackedFiles(ctx)
	}

	return returnCode
}

func main() {
	cfg, err := Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(errorCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	os.Exit(reviewCode(ctx, cfg))
}
