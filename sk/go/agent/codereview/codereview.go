// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package codereview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
)

const (
	flagGerritHost     = "gerrit-host"
	flagStyleGuidePath = "style-guide-path"
	flagSkillPath      = "skill-path"
	flagModel          = "model"
	flagThinkingLevel  = "thinking-level"
	flagDryRun         = "dry-run"
	flagDumpChat       = "dump-chat"
)

// allowedExtensions lists the source file extensions eligible for style review.
var allowedExtensions = map[string]bool{
	".c":    true,
	".cc":   true,
	".cpp":  true,
	".cxx":  true,
	".gn":   true,
	".gni":  true,
	".go":   true,
	".h":    true,
	".hpp":  true,
	".hxx":  true,
	".inc":  true,
	".inl":  true,
	".m":    true,
	".mm":   true,
	".py":   true,
	".sksl": true,
}

// reviewOptions holds the CLI configuration for a single CL review run.
type reviewOptions struct {
	styleGuidePath string
	skillPath      string
	model          string
	thinkingLevel  string
	dryRun         bool
	dumpChat       string
}

// Command returns a cli.Command instance which represents the "code-review" command.
func Command() *cli.Command {
	return &cli.Command{
		Name:      "code-review",
		Usage:     "Perform an AI style review on a Gerrit CL",
		ArgsUsage: "<issue ID>|<issue URL>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagGerritHost,
				Value: gerrit.GerritSkiaURL,
				Usage: "Gerrit host URL (used when an issue ID is provided instead of a URL).",
			},
			&cli.StringFlag{
				Name:  flagStyleGuidePath,
				Value: "site/docs/dev/contrib/style.md",
				Usage: "Path to the style guide within the repo.",
			},
			&cli.StringFlag{
				Name:  flagSkillPath,
				Value: "agents/skills/skia-style-check/SKILL.md",
				Usage: "Path to the relevant SKILL.md file.",
			},
			&cli.StringFlag{
				Name:  flagModel,
				Value: "gemini-flash-latest",
				Usage: "Gemini model to use.",
			},
			&cli.StringFlag{
				Name:  flagThinkingLevel,
				Value: "MEDIUM",
				Usage: "Gemini thinking level (MINIMAL, LOW, MEDIUM, HIGH).",
			},
			&cli.BoolFlag{
				Name:  flagDryRun,
				Value: false,
				Usage: "Do not post comments to Gerrit.",
			},
			&cli.StringFlag{
				Name:  flagDumpChat,
				Value: "chat_history.json",
				Usage: "If non-empty, path to write the full Gemini chat sequence as JSON.",
			},
		},
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
			if len(args) != 1 {
				return skerr.Fmt("expected exactly one argument: <issue ID>|<issue URL>")
			}
			gerritURL, issueID, err := parseIssueArg(args[0], c.String(flagGerritHost))
			if err != nil {
				return skerr.Wrap(err)
			}

			apiKey := os.Getenv("GEMINI_API_KEY")
			if apiKey == "" {
				return skerr.Fmt("GEMINI_API_KEY environment variable is required")
			}

			ctx := c.Context
			ts, err := google.DefaultTokenSource(ctx, gerrit.AuthScope)
			if err != nil {
				return skerr.Wrapf(err, "failed to create token source")
			}
			httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()

			g, err := gerrit.NewGerrit(gerritURL, httpClient)
			if err != nil {
				return skerr.Wrapf(err, "failed to create Gerrit client")
			}

			geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey:  apiKey,
				Backend: genai.BackendGeminiAPI,
			})
			if err != nil {
				return skerr.Wrapf(err, "failed to create Gemini client")
			}

			ci, err := g.GetIssueProperties(ctx, issueID)
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve issue %d", issueID)
			}

			opts := reviewOptions{
				styleGuidePath: c.String(flagStyleGuidePath),
				skillPath:      c.String(flagSkillPath),
				model:          c.String(flagModel),
				thinkingLevel:  c.String(flagThinkingLevel),
				dryRun:         c.Bool(flagDryRun),
				dumpChat:       c.String(flagDumpChat),
			}
			return reviewCL(ctx, g, geminiClient, ci, opts)
		},
	}
}

// parseIssueArg accepts either a numeric Gerrit issue ID (using
// defaultGerritHost) or a full Gerrit change URL and returns the host URL and
// issue ID.
func parseIssueArg(arg, defaultGerritHost string) (string, int64, error) {
	if issueID, err := strconv.ParseInt(arg, 10, 64); err == nil && issueID > 0 {
		return defaultGerritHost, issueID, nil
	}
	return gerrit.ParseChangeURL(arg)
}

// reviewCL fetches the CL diff, modified file contents, and style guide from
// Gerrit, runs the Gemini code review, and either writes a local Markdown
// summary (--dry-run) or posts inline comments on the CL.
func reviewCL(ctx context.Context, g gerrit.GerritInterface, geminiClient *genai.Client, ci *gerrit.ChangeInfo, opts reviewOptions) error {
	fmt.Printf("Reviewing issue %d (%s)...\n", ci.Issue, g.Url(ci.Issue))
	const rev = "current"
	patch, err := g.GetPatch(ctx, ci.Issue, rev, "")
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve patch for issue %d (revision %s)", ci.Issue, rev)
	}

	// Build a single combined document containing all reviewable files in the
	// CL with '+' markers on modified lines.
	changedByFile, fileOrder := parseChangedLinesFromPatch(patch)
	files := make(map[string]*sourceFile)
	var allFilesBuilder strings.Builder
	for _, path := range fileOrder {
		ext := strings.ToLower(filepath.Ext(path))
		if !allowedExtensions[ext] {
			fmt.Printf("Skipping file %s (extension %q not in allowlist)\n", path, ext)
			continue
		}
		if len(changedByFile[path]) == 0 {
			continue
		}
		content, err := g.GetContent(ctx, ci.Issue, rev, path)
		if err != nil {
			return skerr.Wrapf(err, "failed to retrieve content for %s in issue %d", path, ci.Issue)
		}
		sf := newSourceFile(path, content, changedByFile[path])
		files[path] = sf
		fmt.Fprintf(&allFilesBuilder, "=== File: %s ===\n", path)
		allFilesBuilder.WriteString(sf.renderAnnotated())
		allFilesBuilder.WriteString("\n")
	}

	if len(files) == 0 {
		fmt.Printf("No reviewable files with added/modified lines in issue %d.\n", ci.Issue)
		return nil
	}

	// Fetch the style guide and skill instructions at the CL's revision so the
	// review uses the rules matching the commit being reviewed.
	styleGuide, err := g.GetContent(ctx, ci.Issue, rev, opts.styleGuidePath)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve style guide (%s) for issue %d (revision %s)", opts.styleGuidePath, ci.Issue, rev)
	}
	skillMd, err := g.GetContent(ctx, ci.Issue, rev, opts.skillPath)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve SKILL.md (%s) for issue %d (revision %s)", opts.skillPath, ci.Issue, rev)
	}

	rawComments, err := runGeminiReview(ctx, geminiClient, styleGuide, skillMd, allFilesBuilder.String(), opts)
	if err != nil {
		return skerr.Wrap(err)
	}

	comments := resolveComments(rawComments, files)
	withFix := 0
	for _, c := range comments {
		if len(c.SuggestedFix) > 0 {
			withFix++
		}
	}
	fmt.Printf("Review returned %d style violation(s) (%d with suggested fixes).\n", len(comments), withFix)

	if len(comments) > 0 {
		if opts.dryRun {
			if err := writeReviewMarkdown(ctx, g, ci, rev, comments); err != nil {
				return skerr.Wrap(err)
			}
		} else {
			// Group comments by path to match the Gerrit ReviewInput.comments map format.
			commentsByPath := make(map[string][]*gerrit.CommentInfo)
			for _, c := range comments {
				gerritComment := &gerrit.CommentInfo{
					Side:       "REVISION",
					Range:      c.Range,
					Message:    "Style: " + c.Message,
					Unresolved: true,
				}
				if len(c.SuggestedFix) > 0 {
					gerritComment.FixSuggestions = []*gerrit.FixSuggestionInfo{
						{
							Description:  c.Message,
							Replacements: c.SuggestedFix,
						},
					}
				}
				commentsByPath[c.Path] = append(commentsByPath[c.Path], gerritComment)
			}
			if err := g.AddInlineComments(ctx, ci, "", commentsByPath); err != nil {
				return skerr.Wrapf(err, "failed to post inline comments on issue %d", ci.Issue)
			}
			fmt.Printf("Posted %d inline comment(s) on %s\n", len(comments), g.Url(ci.Issue))
		}
	}
	return nil
}

// writeReviewMarkdown writes the resolved review comments and their corresponding
// source snippets to a local file.
func writeReviewMarkdown(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo, rev string, comments []*reviewComment) error {
	var md strings.Builder
	latestPatchset := ci.Patchsets[len(ci.Patchsets)-1]
	fmt.Fprintf(&md, "# Review for Issue %d (Patchset %d)\n\n", ci.Issue, latestPatchset.Number)
	fmt.Fprintf(&md, "URL: %s\n\n", g.Url(ci.Issue))

	fileLines := make(map[string][]string)
	for _, c := range comments {
		lines, ok := fileLines[c.Path]
		if !ok {
			content, err := g.GetContent(ctx, ci.Issue, rev, c.Path)
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve content for %s in issue %d", c.Path, ci.Issue)
			}
			lines = strings.Split(content, "\n")
			fileLines[c.Path] = lines
		}

		start := c.Range.StartLine
		if start < 1 {
			start = 1
		}
		end := c.Range.EndLine
		if end < start {
			end = start
		}
		if end > len(lines) {
			end = len(lines)
		}
		var snippet string
		if start <= len(lines) {
			snippet = strings.Join(lines[start-1:end], "\n")
		}

		if c.Range.StartLine == c.Range.EndLine {
			fmt.Fprintf(&md, "## `%s` (line %d)\n\n", c.Path, c.Range.StartLine)
		} else {
			fmt.Fprintf(&md, "## `%s` (lines %d-%d)\n\n", c.Path, c.Range.StartLine, c.Range.EndLine)
		}
		fmt.Fprintf(&md, "```\n%s\n```\n\n%s\n\n", snippet, c.Message)
	}

	filename := fmt.Sprintf("%d_%d.md", ci.Issue, latestPatchset.Number)
	if err := os.WriteFile(filename, []byte(md.String()), 0644); err != nil {
		return skerr.Wrapf(err, "failed to write %s", filename)
	}
	fmt.Printf("Wrote %d inline comment(s) to %s\n", len(comments), filename)
	return nil
}
