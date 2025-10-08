package main

import (
	"context"
	"flag"
	"fmt"
	"os/user"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tracing"
	jobstore "go.skia.org/infra/pinpoint/go/sql/jobs_store"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/catapult"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	"go.skia.org/infra/temporal/go/metrics"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	tempotel "go.temporal.io/sdk/contrib/opentelemetry"
	tempinter "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const appName = "pinpoint-worker"

var (
	hostPort          = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	promPort          = flag.String("promPort", ":8000", "Prometheus port that it listens on.")
	namespace         = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue         = flag.String("taskQueue", "", "Task queue name registered to worker services.")
	local             = flag.Bool("local", false, "Test run on local dev machine (skip GCP tracing).")
	databaseWriteback = flag.Bool("databaseWriteback", false, "Write back pairwise job information into Spanner Database")
	pairwiseDBConnStr = flag.String("pairwise_db_conn_str", "postgresql://root@localhost:5432/natnael-test-database?sslmode=disable",
		"The connection string for the Pairwise backend database.")
)

func main() {
	flag.Parse()

	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
	)

	if *taskQueue == "" {
		if u, err := user.Current(); err != nil {
			sklog.Fatalf("Unable to get the current user: %s", err)
		} else {
			*taskQueue = fmt.Sprintf("localhost.%s", u.Username)
		}
	}

	if !*local {
		if err := tracing.InitializeOtel(); err != nil {
			sklog.Fatalf("Failed to init tracing: %s", err)
		}
	}

	interceptor, err := tempotel.NewTracingInterceptor(tempotel.TracerOptions{})
	if err != nil {
		sklog.Fatalf("Failed to init interceptor: %s", err)
	}

	// The client and worker are heavyweight objects that should be created once per process.
	c, err := client.Dial(client.Options{
		MetricsHandler: metrics.NewMetricsHandler(map[string]string{}, nil),
		HostPort:       *hostPort,
		Namespace:      *namespace,
		Interceptors: []tempinter.ClientInterceptor{
			interceptor,
		},
	})
	if err != nil {
		sklog.Fatalf("Unable to create client: %s", err)
	}
	defer c.Close()

	w := worker.New(c, *taskQueue, worker.Options{})

	// Only register writeback activity if flag is set to true
	// TODO(b/439651172) Consider setting to always true when database is fully
	// integrated into Pairwise Workflow
	if *databaseWriteback {
		// Initialize the database connection pool for Pairwise activities.
		ctx := context.Background()
		cfg, err := pgxpool.ParseConfig(*pairwiseDBConnStr)
		if err != nil {
			sklog.Fatalf("Failed to parse database config: %s", err)
		}
		pool, err := pgxpool.ConnectConfig(ctx, cfg)
		if err != nil {
			sklog.Fatalf("Failed to connect to database: %s", err)
		}
		js := jobstore.NewJobStore(pool)
		jsa := internal.NewJobStoreActivities(js)
		if err != nil {
			sklog.Fatalf("Unable to create job store: %s", err)
		}
		w.RegisterActivityWithOptions(jsa.AddInitialJob, activity.RegisterOptions{Name: internal.AddInitialJob})
		w.RegisterActivityWithOptions(jsa.UpdateJobStatus, activity.RegisterOptions{Name: internal.UpdateJobStatus})
		w.RegisterActivityWithOptions(jsa.SetErrors, activity.RegisterOptions{Name: internal.SetErrors})
		w.RegisterActivityWithOptions(jsa.AddResults, activity.RegisterOptions{Name: internal.AddResults})
		w.RegisterActivityWithOptions(jsa.AddCommitRuns, activity.RegisterOptions{Name: internal.AddCommitRuns})
	}
	bca := &internal.BuildActivity{}
	w.RegisterActivity(bca)
	w.RegisterWorkflowWithOptions(internal.BuildWorkflow, workflow.RegisterOptions{Name: workflows.BuildChrome})

	rba := &internal.RunBenchmarkActivity{}
	w.RegisterActivity(rba)
	w.RegisterWorkflowWithOptions(internal.RunBenchmarkWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmark})
	w.RegisterWorkflowWithOptions(internal.RunBenchmarkPairwiseWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmarkPairwise})

	w.RegisterActivity(internal.CollectValuesActivity)
	w.RegisterActivity(internal.CollectAllValuesActivity)
	w.RegisterWorkflowWithOptions(internal.SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	w.RegisterActivity(internal.CompareActivity)
	w.RegisterActivity(internal.FindMidCommitActivity)
	w.RegisterActivity(internal.CheckCombinedCommitEqualActivity)
	w.RegisterActivity(internal.ReportStatusActivity)
	w.RegisterWorkflowWithOptions(internal.BisectWorkflow, workflow.RegisterOptions{Name: workflows.Bisect})

	w.RegisterActivity(internal.FindAvailableBotsActivity)
	w.RegisterActivity(internal.ComparePairwiseActivity)
	w.RegisterWorkflowWithOptions(internal.PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	w.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})

	w.RegisterActivity(internal.PostBugCommentActivity)
	w.RegisterWorkflowWithOptions(internal.PostBugCommentWorkflow, workflow.RegisterOptions{Name: workflows.BugUpdate})

	// CBB workflows and activities registration.
	w.RegisterActivity(internal.ReadGitFileActivity)
	w.RegisterActivity(internal.GetChromeReleasesInfoActivity)
	w.RegisterActivity(internal.CollectBrowserVersionsActivity)
	w.RegisterActivity(internal.DownloadSafariTPActivity)
	w.RegisterActivity(internal.UploadCbbResultsActivity)
	w.RegisterWorkflowWithOptions(internal.CbbRunnerWorkflow, workflow.RegisterOptions{Name: workflows.CbbRunner})
	w.RegisterWorkflowWithOptions(internal.CbbNewReleaseDetectorWorkflow, workflow.RegisterOptions{Name: workflows.CbbNewReleaseDetector})
	w.RegisterWorkflowWithOptions(internal.CbbGetBrowserVersionsWorkflow, workflow.RegisterOptions{Name: workflows.CbbGetBrowserVersions})

	// TODO(b/322203189) - Remove Catapult workflows and activities once the backwards
	// UI compatibility is no longer needed and thus the catapult package is deprecated.
	w.RegisterActivity(catapult.FetchTaskActivity)
	w.RegisterActivity(catapult.FetchCommitActivity)
	w.RegisterActivity(catapult.WriteBisectToCatapultActivity)
	w.RegisterWorkflowWithOptions(catapult.CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})
	w.RegisterWorkflowWithOptions(catapult.ConvertToCatapultResponseWorkflow, workflow.RegisterOptions{Name: workflows.ConvertToCatapultResponseWorkflow})
	w.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: workflows.CulpritFinderWorkflow})

	// Activities and workflows for experiments
	w.RegisterActivity(internal.FetchAllSwarmingTasksActivity)
	w.RegisterActivity(internal.GetAllSampleValuesActivity)
	w.RegisterActivity(internal.UploadResultsActivity)
	w.RegisterWorkflowWithOptions(internal.CollectAndUploadWorkflow, workflow.RegisterOptions{Name: workflows.CollectAndUpload})
	w.RegisterWorkflowWithOptions(internal.RunTestAndExportWorkflow, workflow.RegisterOptions{Name: workflows.TestAndExport})

	err = w.Run(worker.InterruptCh())
	if err != nil {
		sklog.Fatalf("Unable to start worker: %s", err)
	}
}
