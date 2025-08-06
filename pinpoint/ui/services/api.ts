//Contains functions for interacting with the Pinpoint backend API.

export interface Job {
  job_id: string;
  job_name: string;
  job_type: string;
  benchmark: string;
  created_date: string;
  job_status: string;
  bot_name: string;
  user: string;
}

export interface ListJobsOptions {
  searchTerm?: string;
  benchmark?: string;
  botName?: string;
  user?: string;
  limit?: number;
  offset?: number;
}

export interface CASDigest {
  hash: string;
  size_bytes: number;
}

export interface CASReference {
  cas_instance: string;
  digest: CASDigest;
}

// This interface represents a single commit's details.
// It mirrors the pinpointpb.Commit struct.
export interface Commit {
  git_hash: string;
  repository: string;
  url?: string;
  author?: string;
  created?: string; // time.Time Golang string. General scheme is YYYY-MM-DDTHH:MM:SS.SSSSSSSSSZ
  subject?: string;
  message?: string;
  commit_branch?: string;
  commit_position?: number;
  review_url?: string;
  change_id?: string;
}

// This interface represents a commit with potential dependencies or patches.
// It mirrors the pinpointpb.CombinedCommit struct.
export interface CombinedCommit {
  main: Commit | null;
  modified_deps?: Commit[];
  patch?: any | null; // This corresponds to *cabe.v1.GerritChange
}

export interface Build {
  // Fields from embedded workflows.BuildParams
  WorkflowID: string;
  Commit: CombinedCommit | null; // This corresponds to *common.CombinedCommit
  Device: string;
  Target: string;
  Patch: any[]; // This corresponds to []*buildbucketpb.GerritChange
  Project: string;
  // Fields from workflows.Build
  ID: number;
  Status: number; // This corresponds to buildbucketpb.Status
  CAS: CASReference | null;
}

export interface TestRun {
  TaskID: string;
  Status: string; // This corresponds to run_benchmark.State
  CAS: CASReference | null;
  Architecture: string;
  OSName: string;
  Values: { [key: string]: number[] };
  Units: { [key: string]: string };
}

export interface CommitRunData {
  Build: Build | null;
  Runs: TestRun[];
}

export interface CommitRuns {
  left: CommitRunData;
  right: CommitRunData;
}
export interface AdditionalRequestParameters {
  start_commit_githash?: string;
  end_commit_githash?: string;
  story?: string;
  story_tags?: string;
  initial_attempt_count?: string;
  aggregation_method?: string;
  target?: string;
  project?: string;
  bug_id?: string;
  chart?: string;
  duration?: string;
  commit_runs?: CommitRuns;
}

export interface WilcoxonResult {
  p_value: number;
  confidence_interval_lower: number;
  confidence_interval_higher: number;
  control_median: number;
  treatment_median: number;
  significant: boolean;
}

/**
 * JobSchema mirrors the jobstore.JobSchema struct from the Go backend.
 * The frontend will receive this entire object and can decide what to use.
 */
export interface JobSchema {
  JobID: string;
  JobName: string;
  JobStatus: string;
  JobType: string;
  SubmittedBy: string;
  Benchmark: string;
  BotName: string;
  AdditionalRequestParameters: AdditionalRequestParameters;
  MetricSummary: { [chart: string]: WilcoxonResult };
  ErrorMessage: string;
  CreatedDate: string; // This will be a time.Time string from Go.
}

/**
 * BuildStatus provides a mapping from the integer status received from the
 * backend (buildbucketpb.Status) to a human-readable string.
 */
export const BuildStatus: { [key: number]: string } = {
  0: 'STATUS_UNSPECIFIED',
  1: 'SCHEDULED',
  2: 'STARTED',
  4: 'ENDED_MASK',
  12: 'SUCCESS',
  20: 'FAILURE',
  36: 'INFRA_FAILURE',
  68: 'CANCELED',
};

export interface SchedulePairwiseRequest {
  start_commit?: CombinedCommit;
  end_commit?: CombinedCommit;
  // This key 'bot_name' maps to the 'configuration' field in the proto.
  bot_name?: string;
  benchmark?: string;
  story?: string;
  job_name?: string;
}

export interface PairwiseExecution {
  job_id: string;
}

export interface CancelJobRequest {
  job_id: string;
  reason: string;
}

export interface CancelJobResponse {
  job_id: string;
  state: string;
}

/**
 * Schedules a new Pairwise job.
 * @param request - The parameters for the pairwise job.
 * @returns A promise that resolves to the execution details of the new job.
 */
export async function schedulePairwise(
  request: SchedulePairwiseRequest
): Promise<PairwiseExecution> {
  const response = await fetch('/pinpoint/v1/pairwise', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });
  if (!response.ok) {
    throw new Error(`Failed to schedule pairwise job: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Cancels a running job.
 * @param request - The job ID and reason for cancellation.
 * @returns A promise that resolves to the cancellation response.
 */
export async function cancelJob(request: CancelJobRequest): Promise<CancelJobResponse> {
  const params = new URLSearchParams();
  params.set('job_id', request.job_id);
  params.set('reason', request.reason);
  const response = await fetch(`/pinpoint/v1/cancel?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to cancel job: ${response.statusText}`);
  }
  return response.json();
}

/**

 * Fetches a list of jobs from the backend.
 * @param options - The query parameters for the request.
 * @returns A promise that resolves to an array of jobs.
 */
export async function listJobs(options: ListJobsOptions): Promise<Job[]> {
  const params = new URLSearchParams();
  if (options.searchTerm) params.set('search_term', options.searchTerm);
  if (options.benchmark) params.set('benchmark', options.benchmark);
  if (options.botName) params.set('bot_name', options.botName);
  if (options.user) params.set('user', options.user);
  if (options.limit) params.set('limit', options.limit.toString());
  if (options.offset) params.set('offset', options.offset.toString());

  const response = await fetch(`/json/jobs/list?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list jobs: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches the details for a single job.
 * @param jobId - The ID of the job to fetch.
 * @returns A promise that resolves to the job details.
 */
export async function getJob(jobId: string): Promise<JobSchema> {
  const response = await fetch(`/json/job/${jobId}`);
  if (!response.ok) {
    throw new Error(`Failed to get job ${jobId}: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches a list of benchmarks to run jobs against.
 * @returns A promise that resolves to an array of benchmarks.
 */
export async function listBenchmarks(): Promise<string[]> {
  const response = await fetch(`/benchmarks`);
  if (!response.ok) {
    throw new Error(`Failed to list benchmarks: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches a list of bots to run jobs on based on a chosen benchmark.
 * If given an empty benchmark, the function will return all bots.
 * @returns A promise that resolves to an array of strings.
 */
export async function listBots(benchmark: string): Promise<string[]> {
  const params = new URLSearchParams();
  params.set('benchmark', benchmark);

  const response = await fetch(`/bots?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list bots: ${response.statusText}`);
  }
  return response.json();
}

/**
 * Fetches a list of stories to run jobs on based on a chosen benchmark.
 * @returns A promise that resolves to an array of strings.
 */
export async function listStories(benchmark: string): Promise<string[]> {
  const params = new URLSearchParams();
  if (benchmark === '') {
    throw new Error(`Failed to list stories: No benchmark provided`);
  }

  params.set('benchmark', benchmark);
  const response = await fetch(`/stories?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Failed to list stories: ${response.statusText}`);
  }
  return response.json();
}
