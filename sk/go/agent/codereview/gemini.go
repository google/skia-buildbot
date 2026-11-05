// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/invopop/jsonschema"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/genai"
)

const (
	// maxOutputTokens is set to the maximum output token limit supported by
	// Gemini Flash models (65,535). Because MaxOutputTokens is shared between
	// internal chain-of-thought (thoughtsTokenCount) and the structured JSON
	// response (candidatesTokenCount), setting this to the maximum prevents
	// reviews with many comments and suggested fixes from being truncated.
	maxOutputTokens = 65535

	// thinkingBudget is the fallback thinking token budget used when
	// --thinking-level is empty (e.g. for Gemini 2.5 models that configure
	// reasoning via ThinkingBudget instead of thinkingLevel). Capping thinking
	// at 24,576 tokens reserves over 40,000 tokens of maxOutputTokens for the
	// structured JSON response.
	thinkingBudget = 24576

	reviewPrompt = `You are a code reviewer for the Skia project.
Attached are the Skia coding style guide (style.md), SKILL.md, and the full contents of the files
modified in this Gerrit CL (cl_files.txt).

In cl_files.txt, every line is prefixed with a change marker and a 1-based line number:
- Lines starting with "+ <line>:" were added or modified in this CL.
- Lines starting with "  <line>:" are unchanged context lines.

Find all Skia style guide violations on the added/modified ('+') lines across ALL files in
cl_files.txt and return a JSON array of comment objects:
1. Scan every file in cl_files.txt from top to bottom and report all style violations on '+' lines.
   Do NOT report violations on unchanged context lines (' ').
2. Keep each comment focused on a SINGLE distinct style rule and symbol/construct:
   - Do NOT combine multiple distinct style rules into one comment, even if they occur on the same
     line or within the same enum, struct, class, function, or #include block. A single line or
     declaration may violate multiple rules at once; emit a separate comment object for each
     distinct rule violated.
   - Do NOT group multiple unrelated functions, types, or variables scattered across a file into a
     single summary comment. Emit a separate comment for each distinct function, type, or variable.
   - Only group multiple locations into a single comment when they are consecutive lines sharing the
     exact same formatting issue (e.g., tab indentation, or field/enumerator names within the same
     struct/enum) or a single symbol's declaration and its usages.
3. Suggested fixes ("replacements"):
   - Whenever a violation can be cleanly fixed by replacing text in the original file(s), populate
     "replacements" with one or more {path, start_line, old_text, new_text} entries.
   - "old_text" must be the exact raw text from cl_files.txt (do NOT include the "+ <line>: " or
     "  <line>: " prefix).
   - For pure insertions or deletions, include an adjacent line from the file in both "old_text"
     and "new_text" as an anchor.
   - Leave "replacements" empty if the violation cannot be expressed as a clean text replacement
     against the original file (for example, when a file has the wrong extension or directory).
`
)

// modelReplacement is the JSON schema returned by Gemini for a single text
// substitution within a suggested fix.
type modelReplacement struct {
	Path      string `json:"path,omitempty" jsonschema_description:"Optional file path if different from the comment's path."`
	StartLine int    `json:"start_line" jsonschema_description:"Approximate 1-based start line in the original file where old_text begins."`
	OldText   string `json:"old_text" jsonschema_description:"Exact text from the original file to replace (without the '+ <line>: ' prefix)."`
	NewText   string `json:"new_text" jsonschema_description:"Replacement text to substitute for old_text."`
}

// modelComment is the JSON schema returned by Gemini for a single code review
// comment.
type modelComment struct {
	Path         string             `json:"path" jsonschema_description:"File path where the review comment should be attached."`
	StartLine    int                `json:"start_line" jsonschema_description:"1-based start line of the style violation."`
	EndLine      int                `json:"end_line" jsonschema_description:"1-based end line of the style violation (inclusive)."`
	Message      string             `json:"message" jsonschema_description:"Concise explanation of the single style violation and how to fix it."`
	Replacements []modelReplacement `json:"replacements,omitempty" jsonschema_description:"Optional text replacements in the original file that resolve this violation."`
}

// runGeminiReview uploads the style guide, skill instructions, and annotated CL
// files via the Gemini Files API, sends the review prompt in a structured-output
// chat session, and returns the parsed modelComments.
func runGeminiReview(ctx context.Context, geminiClient *genai.Client, styleGuide, skillMd, annotatedFiles string, opts reviewOptions) ([]*modelComment, error) {
	stylePart, cleanupStyle, err := uploadFilePart(ctx, geminiClient, styleGuide, "text/markdown", "style.md")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer cleanupStyle()

	skillPart, cleanupSkill, err := uploadFilePart(ctx, geminiClient, skillMd, "text/markdown", "SKILL.md")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer cleanupSkill()

	filesPart, cleanupFiles, err := uploadFilePart(ctx, geminiClient, annotatedFiles, "text/plain", "cl_files.txt")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer cleanupFiles()

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		MaxOutputTokens:  maxOutputTokens,
	}
	if opts.thinkingLevel != "" {
		config.HTTPOptions = &genai.HTTPOptions{
			ExtraBody: map[string]any{
				"generationConfig": map[string]any{
					"thinkingConfig": map[string]any{
						"includeThoughts": false,
						"thinkingLevel":   opts.thinkingLevel,
					},
				},
			},
		}
	} else {
		config.ThinkingConfig = &genai.ThinkingConfig{
			IncludeThoughts: false,
			ThinkingBudget:  genai.Ptr(int32(thinkingBudget)),
		}
	}
	chat, err := geminiClient.Chats.Create(ctx, opts.model, config, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create Gemini chat")
	}
	var lastResp *genai.GenerateContentResponse
	if opts.dumpChat != "" {
		defer func() {
			dump := struct {
				History  []*genai.Content               `json:"history"`
				Response *genai.GenerateContentResponse `json:"response,omitempty"`
			}{
				History:  chat.History(false),
				Response: lastResp,
			}
			b, err := json.MarshalIndent(dump, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to marshal chat history: %s\n", err)
				return
			}
			if err := os.WriteFile(opts.dumpChat, b, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to write chat history to %s: %s\n", opts.dumpChat, err)
				return
			}
			fmt.Printf("Wrote %d chat history turn(s) to %s\n", len(dump.History), opts.dumpChat)
		}()
	}

	rawComments, resp, err := sendChatMessage[modelComment](
		ctx,
		chat,
		config,
		"ReviewCL",
		stylePart,
		skillPart,
		filesPart,
		*genai.NewPartFromText(reviewPrompt),
	)
	lastResp = resp
	if err != nil {
		return nil, skerr.Wrapf(err, "review request failed")
	}
	return rawComments, nil
}

// doBackoff retries fn with exponential backoff, logging a warning before each
// retry attempt.
func doBackoff(opName string, fn func() error) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.RandomizationFactor = 0.5
	b.Multiplier = 1.5
	b.MaxInterval = 60 * time.Second
	b.MaxElapsedTime = 15 * time.Minute
	return backoff.RetryNotify(fn, b, func(err error, d time.Duration) {
		fmt.Fprintf(os.Stderr, "Warning: %s failed; retrying in %s: %s\n", opName, d, extractErrorMessage(err))
	})
}

// extractErrorMessage formats a genai.APIError with its HTTP code and status,
// or falls back to err.Error().
func extractErrorMessage(err error) string {
	if apiError, ok := err.(genai.APIError); ok {
		return fmt.Sprintf("Error %d %s; Message: %s", apiError.Code, apiError.Status, apiError.Message)
	}
	return err.Error()
}

// sendMessageWithBackoff sends a message on chat with exponential backoff,
// aborting immediately on permanent context-length errors.
func sendMessageWithBackoff(ctx context.Context, chat *genai.Chat, opName string, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
	var resp *genai.GenerateContentResponse
	err := doBackoff(opName, func() error {
		var err error
		resp, err = chat.SendMessage(ctx, parts...)
		if err != nil && strings.Contains(err.Error(), "token count exceeds the maximum") {
			return backoff.Permanent(err)
		}
		return err
	})
	return resp, skerr.Wrap(err)
}

// uploadFilePart uploads content to the Gemini Files API with retry and returns
// a genai.Part referencing the uploaded file along with a cleanup function that
// deletes the remote file when the review finishes.
func uploadFilePart(ctx context.Context, client *genai.Client, content, mimeType, displayName string) (genai.Part, func(), error) {
	var f *genai.File
	err := doBackoff("UploadFile("+displayName+")", func() error {
		var err error
		f, err = client.Files.Upload(ctx, strings.NewReader(content), &genai.UploadFileConfig{
			MIMEType:    mimeType,
			DisplayName: displayName,
		})
		return err
	})
	if err != nil {
		return genai.Part{}, nil, skerr.Wrapf(err, "failed to upload %s", displayName)
	}
	cleanupFn := func() {
		if _, err := client.Files.Delete(ctx, f.Name, nil); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete uploaded file %s: %s\n", f.Name, err)
		}
	}
	return *genai.NewPartFromFile(*f), cleanupFn, nil
}

// sendChatMessage reflects the JSON schema for []*T onto config, sends parts on
// chat, and unmarshals the structured JSON response into a slice of *T.
func sendChatMessage[T any](ctx context.Context, chat *genai.Chat, config *genai.GenerateContentConfig, opName string, parts ...genai.Part) ([]*T, *genai.GenerateContentResponse, error) {
	var result []*T
	config.ResponseJsonSchema = jsonschema.Reflect(&result)
	resp, err := sendMessageWithBackoff(ctx, chat, opName, parts...)
	if err != nil {
		return nil, resp, skerr.Wrapf(err, "failed to send chat message")
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, resp, skerr.Fmt("Gemini returned no content")
	}
	rawJSON := resp.Candidates[0].Content.Parts[0].Text
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, resp, skerr.Wrapf(err, "failed to decode response JSON (raw: %s)", rawJSON)
	}
	return result, resp, nil
}
