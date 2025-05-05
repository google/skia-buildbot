package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func validatePairwiseRequest2(req *pb.SchedulePairwiseRequest) error {
	switch {
	case req.Configuration == "":
		return skerr.Fmt("configuration is undefined")
	case req.Benchmark == "":
		return skerr.Fmt("benchmark is undefined")
	case req.Story == "":
		return skerr.Fmt("story is undefined")
	case req.StartCommit == nil || req.EndCommit == nil:
		return skerr.Fmt("invalid start and end commits")
	}
	return nil
}

func TestValidatePairwiseRequest_MissingConfiguration_ReturnError(t *testing.T) {
	req := &pb.SchedulePairwiseRequest{
		Configuration: "",
		Benchmark:     "speedometer3",
		Story:         "s3",
		StartCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("1"),
		},
		EndCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("2"),
		},
	}

	err := validatePairwiseRequest(req)
	assert.ErrorContains(t, err, "configuration is undefined")
}

func TestValidatePairwiseRequest_MissingBenchmark_ReturnError(t *testing.T) {
	req := &pb.SchedulePairwiseRequest{
		Configuration: "mac-m1_mini_2020_perf",
		Benchmark:     "",
		Story:         "s3",
		StartCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("1"),
		},
		EndCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("2"),
		},
	}

	err := validatePairwiseRequest(req)
	assert.ErrorContains(t, err, "benchmark is undefined")
}

func TestValidatePairwiseRequest_MissingStory_ReturnError(t *testing.T) {
	req := &pb.SchedulePairwiseRequest{
		Configuration: "mac-m1_mini_2020_perf",
		Benchmark:     "speedometer3",
		Story:         "",
		StartCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("1"),
		},
		EndCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("2"),
		},
	}

	err := validatePairwiseRequest(req)
	assert.ErrorContains(t, err, "story is undefined")
}

func TestValidatePairwiseRequest_StartAndEndCommits_ReturnError(t *testing.T) {
	req := &pb.SchedulePairwiseRequest{
		Configuration: "mac-m1_mini_2020_perf",
		Benchmark:     "speedometer3",
		Story:         "s3",
		EndCommit: &pb.CombinedCommit{
			Main: common.NewChromiumCommit("2"),
		},
	}

	err := validatePairwiseRequest(req)
	assert.ErrorContains(t, err, "invalid start and end commits")

	req.StartCommit = &pb.CombinedCommit{
		Main: common.NewChromiumCommit("1"),
	}
	req.EndCommit = nil

	err = validatePairwiseRequest(req)
	assert.ErrorContains(t, err, "invalid start and end commits")
}

func TestValidateQueryPairwiseRequest_MissingJobId_ReturnError(t *testing.T) {
	req := &pb.QueryPairwiseRequest{
		JobId: "",
	}
	err := validateQueryPairwiseRequest(req)
	assert.ErrorContains(t, err, "Job ID is undefined")
}

func TestValidateQueryPairwiseRequest_ValidRequest_ReturnNil(t *testing.T) {
	req := &pb.QueryPairwiseRequest{
		JobId: "189ee4a7-fe14-4472-81eb-d201b17ddd9b",
	}
	err := validateQueryPairwiseRequest(req)
	assert.NoError(t, err)
}

func TestValidateQueryPairwiseRequest_JobIdNotValidUUID_ReturnsError(t *testing.T) {
	invalidJobID := "this-is-not-a-uuid"
	req := &pb.QueryPairwiseRequest{
		JobId: invalidJobID,
	}
	err := validateQueryPairwiseRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID length")
}
