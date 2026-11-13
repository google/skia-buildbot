// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"text/tabwriter"
	"time"

	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/skerr"
)

const (
	perfserverImage       = "gcr.io/skia-public/perfserver"
	backendserverImage    = "gcr.io/skia-public/backendserver"
	groupingWorkflowImage = "gcr.io/skia-public/grouping_workflow"
	gatePass              = "PASS"
	gateBlock             = "BLOCK"
)

func latestStageFile(stagesFile string) (*stages.StageFile, error) {
	sf, err := stages.DecodeFile(stagesFile)
	if err != nil {
		return nil, skerr.Wrapf(err, "decoding stages file %s", stagesFile)
	}
	return sf, nil
}

// latestPerfserverHash reads the git hash of the latest perfserver stage from
// stages.json, which is the build that smoke tests validate before promotion.
func latestPerfserverHash(stagesFile string) (string, error) {
	sf, err := latestStageFile(stagesFile)
	if err != nil {
		return "", err
	}
	return latestPerfserverHashFromStageFile(sf, stagesFile)
}

func latestPerfserverHashFromStageFile(sf *stages.StageFile, source string) (string, error) {
	img := sf.Images[perfserverImage]
	if img == nil {
		return "", skerr.Fmt("image %q not found in %s", perfserverImage, source)
	}
	latest := img.Stages["latest"]
	if latest == nil || latest.GitHash == "" {
		return "", skerr.Fmt("stage \"latest\" for %q has no git_hash in %s", perfserverImage, source)
	}
	return latest.GitHash, nil
}

func normalizeStagesURL(stagesURL string) (string, error) {
	u, err := url.Parse(stagesURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", skerr.Fmt("invalid stages_url %q", stagesURL)
	}
	if strings.HasSuffix(u.Hostname(), ".googlesource.com") && !strings.HasPrefix(u.Path, "/a/") {
		u.Path = "/a/" + strings.TrimPrefix(u.Path, "/")
	}
	q := u.Query()
	if q.Get("format") == "" {
		q.Set("format", "TEXT")
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// fetchLatestPerfserverHash downloads stages.json from a Gitiles URL
// (?format=TEXT, base64-encoded) and returns the latest perfserver git hash.
func fetchLatestPerfserverHash(ctx context.Context, client *http.Client, stagesURL string) (string, error) {
	reqURL, err := normalizeStagesURL(stagesURL)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", skerr.Wrapf(err, "fetching %s", reqURL)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", skerr.Wrapf(err, "reading %s", reqURL)
	}
	if resp.StatusCode != http.StatusOK {
		return "", skerr.Fmt("GET %s returned %d", reqURL, resp.StatusCode)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	if err != nil {
		return "", skerr.Wrapf(err, "base64-decoding stages file from %s", stagesURL)
	}
	sf, err := stages.Decode(raw)
	if err != nil {
		return "", skerr.Wrapf(err, "decoding stages file from %s", stagesURL)
	}
	return latestPerfserverHashFromStageFile(sf, stagesURL)
}

type gateDecision struct {
	Instance          string
	Decision          string
	AppVersion        string
	ConsecutivePasses int
	Age               time.Duration
	HasVerdict        bool
	Reasons           []string
}

// evaluateGate checks every required instance and records all failed conditions
// per instance so operators see every blocker in one run rather than fixing
// them one at a time.
func evaluateGate(latest string, instances []string, verdicts map[string]*Verdict, minPasses int, maxAge time.Duration, now time.Time) ([]gateDecision, bool) {
	decisions := make([]gateDecision, 0, len(instances))
	overallPass := len(instances) > 0
	for _, inst := range instances {
		v := verdicts[inst]
		if v == nil {
			overallPass = false
			decisions = append(decisions, gateDecision{
				Instance: inst,
				Decision: gateBlock,
				Reasons:  []string{"missing verdict"},
			})
			continue
		}
		age := now.Sub(v.FinishedAt)
		var reasons []string
		if v.Status != statusPass {
			reasons = append(reasons, fmt.Sprintf("status is %q, want %q", v.Status, statusPass))
		}
		if v.AppVersion != latest {
			reasons = append(reasons, fmt.Sprintf("app_version %s != latest %s", shortHash(v.AppVersion), shortHash(latest)))
		}
		if v.ConsecutivePasses < minPasses {
			reasons = append(reasons, fmt.Sprintf("consecutive_passes %d < %d", v.ConsecutivePasses, minPasses))
		}
		if age > maxAge {
			reasons = append(reasons, fmt.Sprintf("verdict age %s > max_age %s", age.Round(time.Second), maxAge))
		}
		decision := gatePass
		if len(reasons) > 0 {
			decision = gateBlock
			overallPass = false
		}
		decisions = append(decisions, gateDecision{
			Instance:          inst,
			Decision:          decision,
			AppVersion:        v.AppVersion,
			ConsecutivePasses: v.ConsecutivePasses,
			Age:               age,
			HasVerdict:        true,
			Reasons:           reasons,
		})
	}
	return decisions, overallPass
}

func shortHash(h string) string {
	if len(h) > 10 {
		return h[:10]
	}
	if h == "" {
		return "-"
	}
	return h
}

func printGateTable(w io.Writer, latest string, decisions []gateDecision, overallPass bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INSTANCE\tDECISION\tAPP_VERSION\tSTREAK\tAGE\tREASONS")
	for _, d := range decisions {
		if !d.HasVerdict {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t-\t-\t-\t%s\n", d.Instance, d.Decision, strings.Join(d.Reasons, "; "))
			continue
		}
		reasons := "ok"
		if len(d.Reasons) > 0 {
			reasons = strings.Join(d.Reasons, "; ")
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			d.Instance,
			d.Decision,
			shortHash(d.AppVersion),
			d.ConsecutivePasses,
			d.Age.Round(time.Second),
			reasons,
		)
	}
	_ = tw.Flush()
	overall := gateBlock
	if overallPass {
		overall = gatePass
	}
	_, _ = fmt.Fprintf(w, "OVERALL: %s (latest=%s)\n", overall, shortHash(latest))
}

func formatGateReport(sf *stages.StageFile, latest string, decisions []gateDecision, overallPass bool) string {
	var buf bytes.Buffer
	printGateTable(&buf, latest, decisions, overallPass)

	if perfImg := sf.Images[perfserverImage]; perfImg != nil {
		prodStage := perfImg.Stages["prod"]
		latestStage := perfImg.Stages["latest"]
		repoURL := perfImg.GitRepo
		if repoURL == "" {
			repoURL = sf.DefaultGitRepo
		}
		repoURL = strings.TrimSuffix(repoURL, ".git")
		if repoURL == "" {
			repoURL = "https://skia.googlesource.com/buildbot"
		}
		if prodStage != nil && latestStage != nil && prodStage.GitHash != "" && latestStage.GitHash != "" {
			_, _ = fmt.Fprintf(&buf, "\nCommit range: %s/+log/%s..%s\n", repoURL, prodStage.GitHash, latestStage.GitHash)
		}
	}

	companionNotes := []struct {
		image string
		note  string
	}{
		{
			image: backendserverImage,
			note:  "Same digest as autopush; not functionally checked by smoke tests.",
		},
		{
			image: groupingWorkflowImage,
			note:  "latest started and is polling in skia-infra-public-dev; no workflows executed before prod.",
		},
	}
	var companionLines []string
	for _, c := range companionNotes {
		img := sf.Images[c.image]
		if img == nil {
			continue
		}
		prodStage := img.Stages["prod"]
		latestStage := img.Stages["latest"]
		if prodStage == nil || latestStage == nil {
			continue
		}
		companionLines = append(companionLines, fmt.Sprintf("- %s: %s -> %s (%s): %s",
			path.Base(c.image),
			shortHash(prodStage.GitHash),
			shortHash(latestStage.GitHash),
			latestStage.Digest,
			c.note,
		))
	}
	if len(companionLines) > 0 {
		_, _ = fmt.Fprintln(&buf, "\nCompanion images:")
		for _, line := range companionLines {
			_, _ = fmt.Fprintln(&buf, line)
		}
	}

	_, _ = fmt.Fprintln(&buf, "\nRollback: run the \"Rollback Perfserver Prod\" flow in Louhi.")
	return buf.String()
}
