package buildbucket

import (
	"context"
	"strconv"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/metrics2"
	metrics_cleanup "go.skia.org/infra/go/metrics2/cleanup"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	buildAgeMetric  = "buildbucket_build_age"
	finishLagMetric = "buildbucket_finish_update_lag"
	pollFrequency   = 30 * time.Second
)

// Start ingesting Buildbucket metrics.
func Start(ctx context.Context, tsDB db.JobReader, bb2 buildbucket.BuildBucketInterface, buildbucketProject, buildbucketBucket string) {
	lv := metrics2.NewLiveness("last_successful_buildbucket_metrics_update")
	metrics_cleanup.DoMetricsWithCleanup(ctx, pollFrequency, lv, func(ctx context.Context, now time.Time) ([]metrics2.Int64Metric, error) {
		return doMetrics(ctx, tsDB, bb2, buildbucketProject, buildbucketBucket, now)
	})
}

// doMetrics performs a single loop of Buildbucket metrics.
func doMetrics(ctx context.Context, tsDb db.JobReader, bb2 buildbucket.BuildBucketInterface, buildbucketProject, buildbucketBucket string, now time.Time) ([]metrics2.Int64Metric, error) {
	// Retrieve all active builds.
	scheduled, err := bb2.Search(ctx, &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{
			Project: buildbucketProject,
			Bucket:  buildbucketBucket,
		},
		Status: buildbucketpb.Status_SCHEDULED,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	started, err := bb2.Search(ctx, &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{
			Project: buildbucketProject,
			Bucket:  buildbucketBucket,
		},
		Status: buildbucketpb.Status_STARTED,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	builds := append(scheduled, started...)

	// Create metrics for the active builds.
	newMetrics := make([]metrics2.Int64Metric, 0, len(builds))
	for _, build := range builds {
		// Age and status of the build.
		var jobId string
		if build.Infra != nil && build.Infra.Backend != nil && build.Infra.Backend.Task != nil && build.Infra.Backend.Task.Id != nil {
			jobId = build.Infra.Backend.Task.Id.Id
		}
		buildAge := int64(now.Sub(build.CreateTime.AsTime()).Seconds())
		m := metrics2.GetInt64Metric(buildAgeMetric, map[string]string{
			"build_id":    strconv.FormatInt(build.Id, 10),
			"build_state": build.Status.String(),
			"job_id":      jobId,
		})
		m.Update(buildAge)
		newMetrics = append(newMetrics, m)

		// If the build is started and the job is finished, add a metric which
		// tracks the lag time between the job finishing and the build being
		// marked as finished.
		if jobId != "" {
			job, err := tsDb.GetJobById(ctx, jobId)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if job == nil {
				sklog.Errorf("No such job %s for build %s", jobId, build.Id)
				continue
			}
			if job.Done() {
				lagTime := int64(now.Sub(job.Finished).Seconds())
				m := metrics2.GetInt64Metric(finishLagMetric, map[string]string{
					"build_id":    strconv.FormatInt(build.Id, 10),
					"build_state": build.Status.String(),
					"job_id":      jobId,
					"job_state":   string(job.Status),
				})
				m.Update(lagTime)
				newMetrics = append(newMetrics, m)
			}
		}
	}
	return newMetrics, nil
}
