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
