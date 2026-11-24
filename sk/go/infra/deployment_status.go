// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package infra

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/cd/go/deployment_status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/term"
	"golang.org/x/oauth2/google"
)

const (
	flagAncestor        = "commit"
	flagCluster         = "cluster"
	flagNamespace       = "namespace"
	flagPod             = "pod"
	flagContainer       = "container"
	flagCommitFilter    = "image-commit"
	flagCommitTime      = "commit-time"
	flagCommitSubject   = "commit-subject"
	flagImage           = "image"
	flagIncludeExternal = "include-external"
	flagRepo            = "repo"
)

func deploymentStatusCommand() *cli.Command {
	return &cli.Command{
		Name:        "deployment-status",
		Description: "Display deployment status of containers and check whether a commit is live.",
		Usage:       "deployment-status [options]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    flagAncestor,
				Aliases: []string{"cl"},
				Usage:   "Commit hash/prefix or Gerrit CL number/URL to test whether deployed images include it.",
			},
			&cli.StringFlag{
				Name:  flagCluster,
				Usage: "Regex filter for Kubernetes cluster.",
			},
			&cli.StringFlag{
				Name:  flagNamespace,
				Usage: "Regex filter for Kubernetes namespace.",
			},
			&cli.StringFlag{
				Name:  flagPod,
				Usage: "Regex filter for Kubernetes pod.",
			},
			&cli.StringFlag{
				Name:  flagContainer,
				Usage: "Regex filter for container name.",
			},
			&cli.StringFlag{
				Name:  flagCommitFilter,
				Usage: "Regex filter for the image's build commit hash.",
			},
			&cli.StringFlag{
				Name:  flagCommitTime,
				Usage: "Regex filter for the image's build commit time.",
			},
			&cli.StringFlag{
				Name:  flagCommitSubject,
				Usage: "Regex filter for the image's build commit subject.",
			},
			&cli.StringFlag{
				Name:  flagImage,
				Usage: "Regex filter for the container image.",
			},
			&cli.BoolFlag{
				Name:  flagIncludeExternal,
				Usage: "Include external images not built by Skia CD.",
			},
			&cli.StringSliceFlag{
				Name:  flagRepo,
				Usage: "Git repository URLs to query.",
				Value: cli.NewStringSlice(
					common.REPO_SKIA_INFRA,
					common.REPO_SKIA,
					common.REPO_K8S_CONFIG,
				),
			},
		},
		Action: deploymentStatus,
	}
}

func deploymentStatus(c *cli.Context) error {
	ctx := c.Context
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
	if err != nil {
		return skerr.Wrap(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	dockerClient, err := docker.NewClient(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	repoURLs := c.StringSlice(flagRepo)
	repos := make([]deployment_status.GitRepo, 0, len(repoURLs))
	for _, repoURL := range repoURLs {
		gr, err := gitiles.NewRepoWithClient(repoURL, httpClient)
		if err != nil {
			return skerr.Wrap(err)
		}
		repos = append(repos, gr)
	}

	q := url.Values{}
	for param, val := range map[string]string{
		"ancestor":       c.String(flagAncestor),
		"cluster":        c.String(flagCluster),
		"namespace":      c.String(flagNamespace),
		"pod":            c.String(flagPod),
		"container":      c.String(flagContainer),
		"commit":         c.String(flagCommitFilter),
		"commit_time":    c.String(flagCommitTime),
		"commit_subject": c.String(flagCommitSubject),
		"image":          c.String(flagImage),
	} {
		if val != "" {
			q.Set(param, val)
		}
	}
	if c.Bool(flagIncludeExternal) {
		q.Set("ignore_external", "false")
	}

	checker, fetchErr := deployment_status.NewChecker(ctx, httpClient, dockerClient, repos)
	if checker == nil {
		return skerr.Wrap(fetchErr)
	}
	resp, err := checker.QueryEntries(ctx, q)
	if err != nil {
		return skerr.Wrap(err)
	}
	if resp.AncestorError != "" {
		return skerr.Fmt("invalid --%s: %s", flagAncestor, resp.AncestorError)
	}
	if len(resp.FilterErrors) > 0 {
		cols := make([]string, 0, len(resp.FilterErrors))
		for col := range resp.FilterErrors {
			cols = append(cols, col)
		}
		sort.Strings(cols)
		msgs := make([]string, 0, len(cols))
		for _, col := range cols {
			msgs = append(msgs, fmt.Sprintf("invalid filter for %s: %s", col, resp.FilterErrors[col]))
		}
		return skerr.Fmt("%s", strings.Join(msgs, "; "))
	}

	includeLive := c.String(flagAncestor) != ""
	header := []string{"Cluster", "Namespace", "Pod", "Container", "Commit", "CommitTime", "CommitSubject", "Image"}
	if includeLive {
		header = append([]string{"Live"}, header...)
	}
	rows := make([][]string, 0, len(resp.Entries)+1)
	rows = append(rows, header)
	for _, e := range resp.Entries {
		shortCommit := e.Commit
		if len(shortCommit) > 7 {
			shortCommit = shortCommit[:7]
		}
		row := []string{
			e.Cluster,
			e.Namespace,
			e.Pod,
			e.Container,
			shortCommit,
			e.CommitTime,
			e.CommitSubject,
			e.Image,
		}
		if includeLive {
			live := "unknown"
			if e.IsAncestor != nil {
				live = fmt.Sprintf("%t", *e.IsAncestor)
			}
			row = append([]string{live}, row...)
		}
		rows = append(rows, row)
	}
	fmt.Println(term.TableConfig{IncludeHeader: true}.MakeTable(rows))
	if fetchErr != nil {
		return skerr.Wrap(fetchErr)
	}
	return nil
}
