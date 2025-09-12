import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export enum TaskStatus {
  TASK_STATUS_PENDING = "TASK_STATUS_PENDING",
  TASK_STATUS_RUNNING = "TASK_STATUS_RUNNING",
  TASK_STATUS_SUCCESS = "TASK_STATUS_SUCCESS",
  TASK_STATUS_FAILURE = "TASK_STATUS_FAILURE",
  TASK_STATUS_MISHAP = "TASK_STATUS_MISHAP",
}

export enum JobStatus {
  JOB_STATUS_IN_PROGRESS = "JOB_STATUS_IN_PROGRESS",
  JOB_STATUS_SUCCESS = "JOB_STATUS_SUCCESS",
  JOB_STATUS_FAILURE = "JOB_STATUS_FAILURE",
  JOB_STATUS_MISHAP = "JOB_STATUS_MISHAP",
  JOB_STATUS_CANCELED = "JOB_STATUS_CANCELED",
  JOB_STATUS_REQUESTED = "JOB_STATUS_REQUESTED",
}

export interface TriggerJob {
  jobName: string;
  commitHash: string;
}

interface TriggerJobJSON {
  job_name?: string;
  commit_hash?: string;
}

const TriggerJobToJSON = (m: TriggerJob): TriggerJobJSON => {
  return {
    job_name: m.jobName,
    commit_hash: m.commitHash,
  };
};

export interface TriggerJobsRequest {
  jobs?: TriggerJob[];
}

interface TriggerJobsRequestJSON {
  jobs?: TriggerJobJSON[];
}

const TriggerJobsRequestToJSON = (m: TriggerJobsRequest): TriggerJobsRequestJSON => {
  return {
    jobs: m.jobs && m.jobs.map(TriggerJobToJSON),
  };
};

export interface TriggerJobsResponse {
  jobIds?: string[];
}

interface TriggerJobsResponseJSON {
  job_ids?: string[];
}

const JSONToTriggerJobsResponse = (m: TriggerJobsResponseJSON): TriggerJobsResponse => {
  return {
    jobIds: m.job_ids,
  };
};

export interface GetJobRequest {
  id: string;
}

interface GetJobRequestJSON {
  id?: string;
}

const GetJobRequestToJSON = (m: GetJobRequest): GetJobRequestJSON => {
  return {
    id: m.id,
  };
};

export interface GetJobResponse {
  job?: Job;
}

interface GetJobResponseJSON {
  job?: JobJSON;
}

const JSONToGetJobResponse = (m: GetJobResponseJSON): GetJobResponse => {
  return {
    job: m.job && JSONToJob(m.job),
  };
};

export interface CancelJobRequest {
  id: string;
}

interface CancelJobRequestJSON {
  id?: string;
}

const CancelJobRequestToJSON = (m: CancelJobRequest): CancelJobRequestJSON => {
  return {
    id: m.id,
  };
};

export interface CancelJobResponse {
  job?: Job;
}

interface CancelJobResponseJSON {
  job?: JobJSON;
}

const JSONToCancelJobResponse = (m: CancelJobResponseJSON): CancelJobResponse => {
  return {
    job: m.job && JSONToJob(m.job),
  };
};

export interface SearchJobsRequest {
  buildbucketBuildId: string;
  hasBuildbucketBuildId: boolean;
  isForce: boolean;
  hasIsForce: boolean;
  issue: string;
  hasIssue: boolean;
  name: string;
  hasName: boolean;
  patchset: string;
  hasPatchset: boolean;
  repo: string;
  hasRepo: boolean;
  revision: string;
  hasRevision: boolean;
  status: JobStatus;
  hasStatus: boolean;
  timeStart?: string;
  hasTimeStart: boolean;
  timeEnd?: string;
  hasTimeEnd: boolean;
}

interface SearchJobsRequestJSON {
  buildbucket_build_id?: string;
  has_buildbucket_build_id?: boolean;
  is_force?: boolean;
  has_is_force?: boolean;
  issue?: string;
  has_issue?: boolean;
  name?: string;
  has_name?: boolean;
  patchset?: string;
  has_patchset?: boolean;
  repo?: string;
  has_repo?: boolean;
  revision?: string;
  has_revision?: boolean;
  status?: string;
  has_status?: boolean;
  time_start?: string;
  has_time_start?: boolean;
  time_end?: string;
  has_time_end?: boolean;
}

const SearchJobsRequestToJSON = (m: SearchJobsRequest): SearchJobsRequestJSON => {
  return {
    buildbucket_build_id: m.buildbucketBuildId,
    has_buildbucket_build_id: m.hasBuildbucketBuildId,
    is_force: m.isForce,
    has_is_force: m.hasIsForce,
    issue: m.issue,
    has_issue: m.hasIssue,
    name: m.name,
    has_name: m.hasName,
    patchset: m.patchset,
    has_patchset: m.hasPatchset,
    repo: m.repo,
    has_repo: m.hasRepo,
    revision: m.revision,
    has_revision: m.hasRevision,
    status: m.status,
    has_status: m.hasStatus,
    time_start: m.timeStart,
    has_time_start: m.hasTimeStart,
    time_end: m.timeEnd,
    has_time_end: m.hasTimeEnd,
  };
};

export interface SearchJobsResponse {
  jobs?: Job[];
}

interface SearchJobsResponseJSON {
  jobs?: JobJSON[];
}

const JSONToSearchJobsResponse = (m: SearchJobsResponseJSON): SearchJobsResponse => {
  return {
    jobs: m.jobs && m.jobs.map(JSONToJob),
  };
};

export interface GetTaskRequest {
  id: string;
  includeStats: boolean;
}

interface GetTaskRequestJSON {
  id?: string;
  include_stats?: boolean;
}

const GetTaskRequestToJSON = (m: GetTaskRequest): GetTaskRequestJSON => {
  return {
    id: m.id,
    include_stats: m.includeStats,
  };
};

export interface GetTaskResponse {
  task?: Task;
}

interface GetTaskResponseJSON {
  task?: TaskJSON;
}

const JSONToGetTaskResponse = (m: GetTaskResponseJSON): GetTaskResponse => {
  return {
    task: m.task && JSONToTask(m.task),
  };
};

export interface SearchTasksRequest {
  attempt: number;
  hasAttempt: boolean;
  issue: string;
  hasIssue: boolean;
  name: string;
  hasName: boolean;
  patchset: string;
  hasPatchset: boolean;
  repo: string;
  hasRepo: boolean;
  revision: string;
  hasRevision: boolean;
  status: TaskStatus;
  hasStatus: boolean;
  timeStart?: string;
  hasTimeStart: boolean;
  timeEnd?: string;
  hasTimeEnd: boolean;
}

interface SearchTasksRequestJSON {
  attempt?: number;
  has_attempt?: boolean;
  issue?: string;
  has_issue?: boolean;
  name?: string;
  has_name?: boolean;
  patchset?: string;
  has_patchset?: boolean;
  repo?: string;
  has_repo?: boolean;
  revision?: string;
  has_revision?: boolean;
  status?: string;
  has_status?: boolean;
  time_start?: string;
  has_time_start?: boolean;
  time_end?: string;
  has_time_end?: boolean;
}

const SearchTasksRequestToJSON = (m: SearchTasksRequest): SearchTasksRequestJSON => {
  return {
    attempt: m.attempt,
    has_attempt: m.hasAttempt,
    issue: m.issue,
    has_issue: m.hasIssue,
    name: m.name,
    has_name: m.hasName,
    patchset: m.patchset,
    has_patchset: m.hasPatchset,
    repo: m.repo,
    has_repo: m.hasRepo,
    revision: m.revision,
    has_revision: m.hasRevision,
    status: m.status,
    has_status: m.hasStatus,
    time_start: m.timeStart,
    has_time_start: m.hasTimeStart,
    time_end: m.timeEnd,
    has_time_end: m.hasTimeEnd,
  };
};

export interface SearchTasksResponse {
  tasks?: Task[];
}

interface SearchTasksResponseJSON {
  tasks?: TaskJSON[];
}

const JSONToSearchTasksResponse = (m: SearchTasksResponseJSON): SearchTasksResponse => {
  return {
    tasks: m.tasks && m.tasks.map(JSONToTask),
  };
};

export interface GetSkipTaskRulesRequest {
}

interface GetSkipTaskRulesRequestJSON {
}

const GetSkipTaskRulesRequestToJSON = (m: GetSkipTaskRulesRequest): GetSkipTaskRulesRequestJSON => {
  return {
  };
};

export interface SkipTaskRule {
  addedBy: string;
  taskSpecPatterns?: string[];
  commits?: string[];
  description: string;
  name: string;
}

interface SkipTaskRuleJSON {
  added_by?: string;
  task_spec_patterns?: string[];
  commits?: string[];
  description?: string;
  name?: string;
}

const JSONToSkipTaskRule = (m: SkipTaskRuleJSON): SkipTaskRule => {
  return {
    addedBy: m.added_by || "",
    taskSpecPatterns: m.task_spec_patterns,
    commits: m.commits,
    description: m.description || "",
    name: m.name || "",
  };
};

export interface GetSkipTaskRulesResponse {
  rules?: SkipTaskRule[];
}

interface GetSkipTaskRulesResponseJSON {
  rules?: SkipTaskRuleJSON[];
}

const JSONToGetSkipTaskRulesResponse = (m: GetSkipTaskRulesResponseJSON): GetSkipTaskRulesResponse => {
  return {
    rules: m.rules && m.rules.map(JSONToSkipTaskRule),
  };
};

export interface AddSkipTaskRuleRequest {
  taskSpecPatterns?: string[];
  commits?: string[];
  description: string;
  name: string;
}

interface AddSkipTaskRuleRequestJSON {
  task_spec_patterns?: string[];
  commits?: string[];
  description?: string;
  name?: string;
}

const AddSkipTaskRuleRequestToJSON = (m: AddSkipTaskRuleRequest): AddSkipTaskRuleRequestJSON => {
  return {
    task_spec_patterns: m.taskSpecPatterns,
    commits: m.commits,
    description: m.description,
    name: m.name,
  };
};

export interface AddSkipTaskRuleResponse {
  rules?: SkipTaskRule[];
}

interface AddSkipTaskRuleResponseJSON {
  rules?: SkipTaskRuleJSON[];
}

const JSONToAddSkipTaskRuleResponse = (m: AddSkipTaskRuleResponseJSON): AddSkipTaskRuleResponse => {
  return {
    rules: m.rules && m.rules.map(JSONToSkipTaskRule),
  };
};

export interface DeleteSkipTaskRuleRequest {
  id: string;
}

interface DeleteSkipTaskRuleRequestJSON {
  id?: string;
}

const DeleteSkipTaskRuleRequestToJSON = (m: DeleteSkipTaskRuleRequest): DeleteSkipTaskRuleRequestJSON => {
  return {
    id: m.id,
  };
};

export interface DeleteSkipTaskRuleResponse {
  rules?: SkipTaskRule[];
}

interface DeleteSkipTaskRuleResponseJSON {
  rules?: SkipTaskRuleJSON[];
}

const JSONToDeleteSkipTaskRuleResponse = (m: DeleteSkipTaskRuleResponseJSON): DeleteSkipTaskRuleResponse => {
  return {
    rules: m.rules && m.rules.map(JSONToSkipTaskRule),
  };
};

export interface RepoState_Patch {
  issue: string;
  patchRepo: string;
  patchset: string;
  server: string;
}

interface RepoState_PatchJSON {
  issue?: string;
  patch_repo?: string;
  patchset?: string;
  server?: string;
}

const JSONToRepoState_Patch = (m: RepoState_PatchJSON): RepoState_Patch => {
  return {
    issue: m.issue || "",
    patchRepo: m.patch_repo || "",
    patchset: m.patchset || "",
    server: m.server || "",
  };
};

export interface RepoState {
  patch?: RepoState_Patch;
  repo: string;
  revision: string;
}

interface RepoStateJSON {
  patch?: RepoState_PatchJSON;
  repo?: string;
  revision?: string;
}

const JSONToRepoState = (m: RepoStateJSON): RepoState => {
  return {
    patch: m.patch && JSONToRepoState_Patch(m.patch),
    repo: m.repo || "",
    revision: m.revision || "",
  };
};

export interface TaskKey {
  repoState?: RepoState;
  name: string;
  forcedJobId: string;
}

interface TaskKeyJSON {
  repo_state?: RepoStateJSON;
  name?: string;
  forced_job_id?: string;
}

const JSONToTaskKey = (m: TaskKeyJSON): TaskKey => {
  return {
    repoState: m.repo_state && JSONToRepoState(m.repo_state),
    name: m.name || "",
    forcedJobId: m.forced_job_id || "",
  };
};

export interface Task_PropertiesEntry {
  [key: string]: string;
}

interface Task_PropertiesEntryJSON {
  [key: string]: string;
}

export interface Task {
  attempt: number;
  commits?: string[];
  createdAt?: string;
  dbModifiedAt?: string;
  finishedAt?: string;
  id: string;
  isolatedOutput: string;
  jobs?: string[];
  maxAttempts: number;
  parentTaskIds?: string[];
  properties?: Task_PropertiesEntry;
  retryOf: string;
  startedAt?: string;
  status: TaskStatus;
  swarmingBotId: string;
  swarmingTaskId: string;
  taskKey?: TaskKey;
  stats?: TaskStats;
  taskExecutor: string;
}

interface TaskJSON {
  attempt?: number;
  commits?: string[];
  created_at?: string;
  db_modified_at?: string;
  finished_at?: string;
  id?: string;
  isolated_output?: string;
  jobs?: string[];
  max_attempts?: number;
  parent_task_ids?: string[];
  properties?: Task_PropertiesEntryJSON;
  retry_of?: string;
  started_at?: string;
  status?: string;
  swarming_bot_id?: string;
  swarming_task_id?: string;
  task_key?: TaskKeyJSON;
  stats?: TaskStatsJSON;
  task_executor?: string;
}

const JSONToTask = (m: TaskJSON): Task => {
  return {
    attempt: m.attempt || 0,
    commits: m.commits,
    createdAt: m.created_at,
    dbModifiedAt: m.db_modified_at,
    finishedAt: m.finished_at,
    id: m.id || "",
    isolatedOutput: m.isolated_output || "",
    jobs: m.jobs,
    maxAttempts: m.max_attempts || 0,
    parentTaskIds: m.parent_task_ids,
    properties: m.properties,
    retryOf: m.retry_of || "",
    startedAt: m.started_at,
    status: (m.status || Object.keys(TaskStatus)[0]) as TaskStatus,
    swarmingBotId: m.swarming_bot_id || "",
    swarmingTaskId: m.swarming_task_id || "",
    taskKey: m.task_key && JSONToTaskKey(m.task_key),
    stats: m.stats && JSONToTaskStats(m.stats),
    taskExecutor: m.task_executor || "",
  };
};

export interface TaskDependencies {
  task: string;
  dependencies?: string[];
}

interface TaskDependenciesJSON {
  task?: string;
  dependencies?: string[];
}

const JSONToTaskDependencies = (m: TaskDependenciesJSON): TaskDependencies => {
  return {
    task: m.task || "",
    dependencies: m.dependencies,
  };
};

export interface TaskSummary {
  id: string;
  attempt: number;
  maxAttempts: number;
  status: TaskStatus;
  swarmingTaskId: string;
}

interface TaskSummaryJSON {
  id?: string;
  attempt?: number;
  max_attempts?: number;
  status?: string;
  swarming_task_id?: string;
}

const JSONToTaskSummary = (m: TaskSummaryJSON): TaskSummary => {
  return {
    id: m.id || "",
    attempt: m.attempt || 0,
    maxAttempts: m.max_attempts || 0,
    status: (m.status || Object.keys(TaskStatus)[0]) as TaskStatus,
    swarmingTaskId: m.swarming_task_id || "",
  };
};

export interface TaskSummaries {
  name: string;
  tasks?: TaskSummary[];
}

interface TaskSummariesJSON {
  name?: string;
  tasks?: TaskSummaryJSON[];
}

const JSONToTaskSummaries = (m: TaskSummariesJSON): TaskSummaries => {
  return {
    name: m.name || "",
    tasks: m.tasks && m.tasks.map(JSONToTaskSummary),
  };
};

export interface TaskSpecSummary {
  taskName: string;
  dimensions?: string[];
  taskExecutor: string;
}

interface TaskSpecSummaryJSON {
  task_name?: string;
  dimensions?: string[];
  task_executor?: string;
}

const JSONToTaskSpecSummary = (m: TaskSpecSummaryJSON): TaskSpecSummary => {
  return {
    taskName: m.task_name || "",
    dimensions: m.dimensions,
    taskExecutor: m.task_executor || "",
  };
};

export interface TaskStats {
  totalOverheadS: string;
  downloadOverheadS: string;
  uploadOverheadS: string;
}

interface TaskStatsJSON {
  total_overhead_s?: string;
  download_overhead_s?: string;
  upload_overhead_s?: string;
}

const JSONToTaskStats = (m: TaskStatsJSON): TaskStats => {
  return {
    totalOverheadS: m.total_overhead_s || "",
    downloadOverheadS: m.download_overhead_s || "",
    uploadOverheadS: m.upload_overhead_s || "",
  };
};

export interface Job {
  buildbucketBuildId: string;
  buildbucketLeaseKey: string;
  createdAt?: string;
  dbModifiedAt?: string;
  dependencies?: TaskDependencies[];
  finishedAt?: string;
  id: string;
  isForce: boolean;
  name: string;
  priority: string;
  repoState?: RepoState;
  requestedAt?: string;
  startedAt?: string;
  status: JobStatus;
  statusDetails: string;
  tasks?: TaskSummaries[];
  taskSpecSummaries?: TaskSpecSummary[];
}

interface JobJSON {
  buildbucket_build_id?: string;
  buildbucket_lease_key?: string;
  created_at?: string;
  db_modified_at?: string;
  dependencies?: TaskDependenciesJSON[];
  finished_at?: string;
  id?: string;
  is_force?: boolean;
  name?: string;
  priority?: string;
  repo_state?: RepoStateJSON;
  requested_at?: string;
  started_at?: string;
  status?: string;
  status_details?: string;
  tasks?: TaskSummariesJSON[];
  task_spec_summaries?: TaskSpecSummaryJSON[];
}

const JSONToJob = (m: JobJSON): Job => {
  return {
    buildbucketBuildId: m.buildbucket_build_id || "",
    buildbucketLeaseKey: m.buildbucket_lease_key || "",
    createdAt: m.created_at,
    dbModifiedAt: m.db_modified_at,
    dependencies: m.dependencies && m.dependencies.map(JSONToTaskDependencies),
    finishedAt: m.finished_at,
    id: m.id || "",
    isForce: m.is_force || false,
    name: m.name || "",
    priority: m.priority || "",
    repoState: m.repo_state && JSONToRepoState(m.repo_state),
    requestedAt: m.requested_at,
    startedAt: m.started_at,
    status: (m.status || Object.keys(JobStatus)[0]) as JobStatus,
    statusDetails: m.status_details || "",
    tasks: m.tasks && m.tasks.map(JSONToTaskSummaries),
    taskSpecSummaries: m.task_spec_summaries && m.task_spec_summaries.map(JSONToTaskSpecSummary),
  };
};

export interface TaskSchedulerService {
  triggerJobs: (triggerJobsRequest: TriggerJobsRequest) => Promise<TriggerJobsResponse>;
  getJob: (getJobRequest: GetJobRequest) => Promise<GetJobResponse>;
  cancelJob: (cancelJobRequest: CancelJobRequest) => Promise<CancelJobResponse>;
  searchJobs: (searchJobsRequest: SearchJobsRequest) => Promise<SearchJobsResponse>;
  getTask: (getTaskRequest: GetTaskRequest) => Promise<GetTaskResponse>;
  searchTasks: (searchTasksRequest: SearchTasksRequest) => Promise<SearchTasksResponse>;
  getSkipTaskRules: (getSkipTaskRulesRequest: GetSkipTaskRulesRequest) => Promise<GetSkipTaskRulesResponse>;
  addSkipTaskRule: (addSkipTaskRuleRequest: AddSkipTaskRuleRequest) => Promise<AddSkipTaskRuleResponse>;
  deleteSkipTaskRule: (deleteSkipTaskRuleRequest: DeleteSkipTaskRuleRequest) => Promise<DeleteSkipTaskRuleResponse>;
}

export class TaskSchedulerServiceClient implements TaskSchedulerService {
  private hostname: string;
  private fetch: Fetch;
  private writeCamelCase: boolean;
  private pathPrefix = "/twirp/task_scheduler.rpc.TaskSchedulerService/";
  private optionsOverride: object;

  constructor(hostname: string, fetch: Fetch, writeCamelCase = false, optionsOverride: any = {}) {
    this.hostname = hostname;
    this.fetch = fetch;
    this.writeCamelCase = writeCamelCase;
    this.optionsOverride = optionsOverride;
  }

  triggerJobs(triggerJobsRequest: TriggerJobsRequest): Promise<TriggerJobsResponse> {
    const url = this.hostname + this.pathPrefix + "TriggerJobs";
    let body: TriggerJobsRequest | TriggerJobsRequestJSON = triggerJobsRequest;
    if (!this.writeCamelCase) {
      body = TriggerJobsRequestToJSON(triggerJobsRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToTriggerJobsResponse);
    });
  }

  getJob(getJobRequest: GetJobRequest): Promise<GetJobResponse> {
    const url = this.hostname + this.pathPrefix + "GetJob";
    let body: GetJobRequest | GetJobRequestJSON = getJobRequest;
    if (!this.writeCamelCase) {
      body = GetJobRequestToJSON(getJobRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetJobResponse);
    });
  }

  cancelJob(cancelJobRequest: CancelJobRequest): Promise<CancelJobResponse> {
    const url = this.hostname + this.pathPrefix + "CancelJob";
    let body: CancelJobRequest | CancelJobRequestJSON = cancelJobRequest;
    if (!this.writeCamelCase) {
      body = CancelJobRequestToJSON(cancelJobRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToCancelJobResponse);
    });
  }

  searchJobs(searchJobsRequest: SearchJobsRequest): Promise<SearchJobsResponse> {
    const url = this.hostname + this.pathPrefix + "SearchJobs";
    let body: SearchJobsRequest | SearchJobsRequestJSON = searchJobsRequest;
    if (!this.writeCamelCase) {
      body = SearchJobsRequestToJSON(searchJobsRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToSearchJobsResponse);
    });
  }

  getTask(getTaskRequest: GetTaskRequest): Promise<GetTaskResponse> {
    const url = this.hostname + this.pathPrefix + "GetTask";
    let body: GetTaskRequest | GetTaskRequestJSON = getTaskRequest;
    if (!this.writeCamelCase) {
      body = GetTaskRequestToJSON(getTaskRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetTaskResponse);
    });
  }

  searchTasks(searchTasksRequest: SearchTasksRequest): Promise<SearchTasksResponse> {
    const url = this.hostname + this.pathPrefix + "SearchTasks";
    let body: SearchTasksRequest | SearchTasksRequestJSON = searchTasksRequest;
    if (!this.writeCamelCase) {
      body = SearchTasksRequestToJSON(searchTasksRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToSearchTasksResponse);
    });
  }

  getSkipTaskRules(getSkipTaskRulesRequest: GetSkipTaskRulesRequest): Promise<GetSkipTaskRulesResponse> {
    const url = this.hostname + this.pathPrefix + "GetSkipTaskRules";
    let body: GetSkipTaskRulesRequest | GetSkipTaskRulesRequestJSON = getSkipTaskRulesRequest;
    if (!this.writeCamelCase) {
      body = GetSkipTaskRulesRequestToJSON(getSkipTaskRulesRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetSkipTaskRulesResponse);
    });
  }

  addSkipTaskRule(addSkipTaskRuleRequest: AddSkipTaskRuleRequest): Promise<AddSkipTaskRuleResponse> {
    const url = this.hostname + this.pathPrefix + "AddSkipTaskRule";
    let body: AddSkipTaskRuleRequest | AddSkipTaskRuleRequestJSON = addSkipTaskRuleRequest;
    if (!this.writeCamelCase) {
      body = AddSkipTaskRuleRequestToJSON(addSkipTaskRuleRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAddSkipTaskRuleResponse);
    });
  }

  deleteSkipTaskRule(deleteSkipTaskRuleRequest: DeleteSkipTaskRuleRequest): Promise<DeleteSkipTaskRuleResponse> {
    const url = this.hostname + this.pathPrefix + "DeleteSkipTaskRule";
    let body: DeleteSkipTaskRuleRequest | DeleteSkipTaskRuleRequestJSON = deleteSkipTaskRuleRequest;
    if (!this.writeCamelCase) {
      body = DeleteSkipTaskRuleRequestToJSON(deleteSkipTaskRuleRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToDeleteSkipTaskRuleResponse);
    });
  }
}
