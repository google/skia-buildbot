// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package formatter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestNewCommitHashRangeFormatter_Gitiles(t *testing.T) {
	mg := &mocks.Git{}
	ctx := context.Background()

	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(101)).Return("1234567890abcdef", nil)
	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(105)).Return("abcdef1234567890", nil)

	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL:      "https://chromium.googlesource.com/chromium/src",
			Provider: config.GitProviderGitiles,
		},
	}

	formatter := NewCommitHashRangeFormatter(mg)
	res := formatter(ctx, 101, 105)

	assert.Equal(t, "[\\(12345678..abcdef12\\]](https://chromium.googlesource.com/chromium/src/+log/1234567890abcdef..abcdef1234567890)", res)
}

func TestNewCommitHashRangeFormatter_OneCommit(t *testing.T) {
	mg := &mocks.Git{}
	ctx := context.Background()

	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(104)).Return("1234567890abcdef", nil)
	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(105)).Return("abcdef1234567890", nil)

	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL:      "https://chromium.googlesource.com/chromium/src",
			Provider: config.GitProviderGitiles,
		},
	}

	formatter := NewCommitHashRangeFormatter(mg)
	res := formatter(ctx, 104, 105)

	assert.Equal(t, "[abcdef12](https://chromium.googlesource.com/chromium/src/+log/1234567890abcdef..abcdef1234567890)", res)
}

func TestNewCommitHashRangeFormatter_NonGitilesFallback(t *testing.T) {
	mg := &mocks.Git{}
	ctx := context.Background()

	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(101)).Return("1234567890abcdef", nil)
	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(105)).Return("abcdef1234567890", nil)

	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL:      "https://github.com/foo/bar",
			Provider: config.GitProviderCLI,
		},
	}

	formatter := NewCommitHashRangeFormatter(mg)
	res := formatter(ctx, 101, 105)

	assert.Equal(t, "[\\(12345678..abcdef12\\]](https://github.com/foo/bar/compare/1234567890abcdef...abcdef1234567890)", res)
}

func TestNewCommitHashRangeFormatter_ShortHashPanicSafety(t *testing.T) {
	mg := &mocks.Git{}
	ctx := context.Background()

	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(201)).Return("abc", nil)
	mg.On("GitHashFromCommitNumber", ctx, types.CommitNumber(205)).Return("123", nil)

	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL:      "https://chromium.googlesource.com/chromium/src",
			Provider: config.GitProviderGitiles,
		},
	}

	formatter := NewCommitHashRangeFormatter(mg)
	res := formatter(ctx, 201, 205)

	assert.Equal(t, "[\\(abc..123\\]](https://chromium.googlesource.com/chromium/src/+log/abc..123)", res)
}
