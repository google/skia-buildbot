// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

type GeminiClient struct {
	Client   *genai.Client
	Project  string
	Location string
	Model    string
}

func (c *GeminiClient) generate(
	ctx context.Context,
	prompt string,
) (string, error) {
	client := c.Client
	if client == nil {
		var err error
		client, err = genai.NewClient(ctx, &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  c.Project,
			Location: c.Location,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create genai client: %w", err)
		}
	}

	config := &genai.GenerateContentConfig{
		Temperature:     ptr(float32(0.0)),
		TopP:            ptr(float32(0.0)),
		MaxOutputTokens: 65535,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: false,
			ThinkingBudget:  ptr(int32(0)),
		},
		SafetySettings: []*genai.SafetySetting{
			{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockThresholdOff},
			{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockThresholdOff},
			{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockThresholdOff},
			{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockThresholdOff},
		},
	}

	resp, err := client.Models.GenerateContent(ctx, c.Model, genai.Text(prompt), config)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return resp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("no output generated")
}

func ptr[T any](v T) *T {
	return &v
}
