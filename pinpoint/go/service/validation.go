package service

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/read_values"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

// updateFieldsForCatapult converts specific catapult Pinpoint arguments
// to their skia Pinpoint counterparts
func updateFieldsForCatapult(req *pb.ScheduleBisectRequest) *pb.ScheduleBisectRequest {
	switch {
	case req.Statistic == "avg":
		req.AggregationMethod = "mean"
	case req.Statistic != "":
		req.AggregationMethod = req.Statistic
	}
	return req
}

// updateCulpritFinderFieldsForCatapult converts specific catapult Pinpoint arguments
// to their skia Pinpoint counterparts
func updateCulpritFinderFieldsForCatapult(req *pb.ScheduleCulpritFinderRequest) *pb.ScheduleCulpritFinderRequest {
	switch {
	case req.Statistic == "avg":
		req.AggregationMethod = "mean"
	case req.Statistic != "":
		req.AggregationMethod = req.Statistic
	case req.AggregationMethod == "avg":
		req.AggregationMethod = "mean"
	}
	return req
}

func validateBisectRequest(req *pb.ScheduleBisectRequest) error {
	switch {
	case req.StartGitHash == "" || req.EndGitHash == "":
		return skerr.Fmt("git hash is empty")
	case !read_values.IsSupportedAggregation(req.AggregationMethod):
		return skerr.Fmt("aggregation method (%s) is not available", req.AggregationMethod)
	default:
		return nil
	}
}

func validateCulpritFinderRequest(req *pb.ScheduleCulpritFinderRequest) error {
	switch {
	case req.StartGitHash == "" || req.EndGitHash == "":
		return skerr.Fmt("git hash is empty")
	case req.Benchmark == "":
		return skerr.Fmt("benchmark is empty")
	case req.Story == "":
		return skerr.Fmt("story is empty")
	case req.Chart == "":
		return skerr.Fmt("chart is empty")
	case req.Configuration == "":
		return skerr.Fmt("configuration (aka the device name) is empty")
	case req.Configuration == "android-pixel-fold-perf" || req.Configuration == "mac-m1-pro-perf":
		return skerr.Fmt("bot (%s) is currently unsupported due to low resources", req.Configuration)
	case !read_values.IsSupportedAggregation(req.AggregationMethod):
		return skerr.Fmt("aggregation method (%s) is not available", req.AggregationMethod)
	}
	return nil
}

// validatePairwiseRequest returns an error if required params are missing.
func validatePairwiseRequest(req *pb.SchedulePairwiseRequest) error {
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
