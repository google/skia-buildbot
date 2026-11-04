// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestGeminiGenerate(t *testing.T) {
	mockResponse := `
	{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "This is a mock review response."
						}
					]
				}
			}
		]
	}`

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "generateContent")
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockResponse))
		}),
	)
	defer ts.Close()

	ctx := context.Background()
	sdkClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:     genai.BackendVertexAI,
		Project:     "test-project",
		Location:    "test-location",
		Credentials: &auth.Credentials{},
		HTTPClient:  ts.Client(),
		HTTPOptions: genai.HTTPOptions{
			BaseURL: ts.URL + "/",
		},
	})
	require.NoError(t, err)

	client := &GeminiClient{
		Client:   sdkClient,
		Project:  "test-project",
		Location: "test-location",
		Model:    "test-model",
	}

	response, err := client.generate(ctx, "Test prompt")

	require.NoError(t, err)
	assert.Equal(t, "This is a mock review response.", response)
}

func TestGeminiGenerate_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal Server Error"))
		}),
	)
	defer ts.Close()

	ctx := context.Background()
	sdkClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:     genai.BackendVertexAI,
		Project:     "test-project",
		Location:    "test-location",
		Credentials: &auth.Credentials{},
		HTTPClient:  ts.Client(),
		HTTPOptions: genai.HTTPOptions{
			BaseURL: ts.URL + "/",
		},
	})
	require.NoError(t, err)

	client := &GeminiClient{
		Client:   sdkClient,
		Project:  "test-project",
		Location: "test-location",
		Model:    "test-model",
	}

	_, err = client.generate(ctx, "Test prompt")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate content")
}
