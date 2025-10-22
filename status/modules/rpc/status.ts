import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export interface GetIncrementalCommitsRequest {
  from?: string;
  to?: string;
  n: number;
  pod: string;
  repoPath: string;
}

interface GetIncrementalCommitsRequestJSON {
  from?: string;
  to?: string;
  n?: number;
  pod?: string;
  repo_path?: string;
}

const GetIncrementalCommitsRequestToJSON = (m: GetIncrementalCommitsRequest): GetIncrementalCommitsRequestJSON => {
  return {
    from: m.from,
    to: m.to,
    n: m.n,
    pod: m.pod,
    repo_path: m.repoPath,
  };
};

export interface GetIncrementalCommitsResponse {
  metadata?: ResponseMetadata;
  update?: IncrementalUpdate;
}

interface GetIncrementalCommitsResponseJSON {
  metadata?: ResponseMetadataJSON;
  update?: IncrementalUpdateJSON;
}

const JSONToGetIncrementalCommitsResponse = (m: GetIncrementalCommitsResponseJSON): GetIncrementalCommitsResponse => {
  return {
    metadata: m.metadata && JSONToResponseMetadata(m.metadata),
    update: m.update && JSONToIncrementalUpdate(m.update),
  };
};

export interface IncrementalUpdate {
  commits?: LongCommit[];
  branchHeads?: Branch[];
  tasks?: Task[];
  comments?: Comment[];
}

interface IncrementalUpdateJSON {
  commits?: LongCommitJSON[];
  branch_heads?: BranchJSON[];
  tasks?: TaskJSON[];
  comments?: CommentJSON[];
}

const JSONToIncrementalUpdate = (m: IncrementalUpdateJSON): IncrementalUpdate => {
  return {
    commits: m.commits && m.commits.map(JSONToLongCommit),
    branchHeads: m.branch_heads && m.branch_heads.map(JSONToBranch),
    tasks: m.tasks && m.tasks.map(JSONToTask),
    comments: m.comments && m.comments.map(JSONToComment),
  };
};

export interface Branch {
  name: string;
  head: string;
}

interface BranchJSON {
  name?: string;
  head?: string;
}

const JSONToBranch = (m: BranchJSON): Branch => {
  return {
    name: m.name || "",
    head: m.head || "",
  };
};

export interface Task {
  commits?: string[];
  name: string;
  id: string;
  revision: string;
  status: string;
  swarmingTaskId: string;
  taskExecutor: string;
}

interface TaskJSON {
  commits?: string[];
  name?: string;
  id?: string;
  revision?: string;
  status?: string;
  swarming_task_id?: string;
  task_executor?: string;
}

const JSONToTask = (m: TaskJSON): Task => {
  return {
    commits: m.commits,
    name: m.name || "",
    id: m.id || "",
    revision: m.revision || "",
    status: m.status || "",
    swarmingTaskId: m.swarming_task_id || "",
    taskExecutor: m.task_executor || "",
  };
};

export interface LongCommit {
  hash: string;
  author: string;
  subject: string;
  parents?: string[];
  body: string;
  timestamp?: string;
}

interface LongCommitJSON {
  hash?: string;
  author?: string;
  subject?: string;
  parents?: string[];
  body?: string;
  timestamp?: string;
}

const JSONToLongCommit = (m: LongCommitJSON): LongCommit => {
  return {
    hash: m.hash || "",
    author: m.author || "",
    subject: m.subject || "",
    parents: m.parents,
    body: m.body || "",
    timestamp: m.timestamp,
  };
};

export interface Comment {
  id: string;
  repo: string;
  timestamp?: string;
  user: string;
  message: string;
  deleted: boolean;
  ignoreFailure: boolean;
  flaky: boolean;
  taskSpecName: string;
  taskId: string;
  commit: string;
}

interface CommentJSON {
  id?: string;
  repo?: string;
  timestamp?: string;
  user?: string;
  message?: string;
  deleted?: boolean;
  ignore_failure?: boolean;
  flaky?: boolean;
  task_spec_name?: string;
  task_id?: string;
  commit?: string;
}

const JSONToComment = (m: CommentJSON): Comment => {
  return {
    id: m.id || "",
    repo: m.repo || "",
    timestamp: m.timestamp,
    user: m.user || "",
    message: m.message || "",
    deleted: m.deleted || false,
    ignoreFailure: m.ignore_failure || false,
    flaky: m.flaky || false,
    taskSpecName: m.task_spec_name || "",
    taskId: m.task_id || "",
    commit: m.commit || "",
  };
};

export interface ResponseMetadata {
  startOver: boolean;
  pod: string;
  timestamp?: string;
}

interface ResponseMetadataJSON {
  start_over?: boolean;
  pod?: string;
  timestamp?: string;
}

const JSONToResponseMetadata = (m: ResponseMetadataJSON): ResponseMetadata => {
  return {
    startOver: m.start_over || false,
    pod: m.pod || "",
    timestamp: m.timestamp,
  };
};

export interface AddCommentRequest {
  repo: string;
  commit: string;
  taskSpec: string;
  taskId: string;
  message: string;
  flaky: boolean;
  ignoreFailure: boolean;
}

interface AddCommentRequestJSON {
  repo?: string;
  commit?: string;
  task_spec?: string;
  task_id?: string;
  message?: string;
  flaky?: boolean;
  ignore_failure?: boolean;
}

const AddCommentRequestToJSON = (m: AddCommentRequest): AddCommentRequestJSON => {
  return {
    repo: m.repo,
    commit: m.commit,
    task_spec: m.taskSpec,
    task_id: m.taskId,
    message: m.message,
    flaky: m.flaky,
    ignore_failure: m.ignoreFailure,
  };
};

export interface AddCommentResponse {
  timestamp?: string;
}

interface AddCommentResponseJSON {
  timestamp?: string;
}

const JSONToAddCommentResponse = (m: AddCommentResponseJSON): AddCommentResponse => {
  return {
    timestamp: m.timestamp,
  };
};

export interface DeleteCommentRequest {
  repo: string;
  commit: string;
  taskSpec: string;
  taskId: string;
  timestamp?: string;
}

interface DeleteCommentRequestJSON {
  repo?: string;
  commit?: string;
  task_spec?: string;
  task_id?: string;
  timestamp?: string;
}

const DeleteCommentRequestToJSON = (m: DeleteCommentRequest): DeleteCommentRequestJSON => {
  return {
    repo: m.repo,
    commit: m.commit,
    task_spec: m.taskSpec,
    task_id: m.taskId,
    timestamp: m.timestamp,
  };
};

export interface DeleteCommentResponse {
}

interface DeleteCommentResponseJSON {
}

const JSONToDeleteCommentResponse = (m: DeleteCommentResponseJSON): DeleteCommentResponse => {
  return {
  };
};

export interface GetAutorollerStatusesRequest {
}

interface GetAutorollerStatusesRequestJSON {
}

const GetAutorollerStatusesRequestToJSON = (m: GetAutorollerStatusesRequest): GetAutorollerStatusesRequestJSON => {
  return {
  };
};

export interface GetAutorollerStatusesResponse {
  rollers?: AutorollerStatus[];
}

interface GetAutorollerStatusesResponseJSON {
  rollers?: AutorollerStatusJSON[];
}

const JSONToGetAutorollerStatusesResponse = (m: GetAutorollerStatusesResponseJSON): GetAutorollerStatusesResponse => {
  return {
    rollers: m.rollers && m.rollers.map(JSONToAutorollerStatus),
  };
};

export interface AutorollerStatus {
  name: string;
  currentRollRev: string;
  lastRollRev: string;
  mode: string;
  numFailed: number;
  numBehind: number;
  url: string;
}

interface AutorollerStatusJSON {
  name?: string;
  current_roll_rev?: string;
  last_roll_rev?: string;
  mode?: string;
  num_failed?: number;
  num_behind?: number;
  url?: string;
}

const JSONToAutorollerStatus = (m: AutorollerStatusJSON): AutorollerStatus => {
  return {
    name: m.name || "",
    currentRollRev: m.current_roll_rev || "",
    lastRollRev: m.last_roll_rev || "",
    mode: m.mode || "",
    numFailed: m.num_failed || 0,
    numBehind: m.num_behind || 0,
    url: m.url || "",
  };
};

export interface GetBotUsageRequest {
}

interface GetBotUsageRequestJSON {
}

const GetBotUsageRequestToJSON = (m: GetBotUsageRequest): GetBotUsageRequestJSON => {
  return {
  };
};

export interface GetBotUsageResponse {
  botSets?: BotSet[];
}

interface GetBotUsageResponseJSON {
  bot_sets?: BotSetJSON[];
}

const JSONToGetBotUsageResponse = (m: GetBotUsageResponseJSON): GetBotUsageResponse => {
  return {
    botSets: m.bot_sets && m.bot_sets.map(JSONToBotSet),
  };
};

export interface BotSet_DimensionsEntry {
  [key: string]: string;
}

interface BotSet_DimensionsEntryJSON {
  [key: string]: string;
}

export interface BotSet {
  dimensions?: BotSet_DimensionsEntry;
  botCount: number;
  cqTasks: number;
  msPerCq: number;
  totalTasks: number;
  msPerCommit: number;
}

interface BotSetJSON {
  dimensions?: BotSet_DimensionsEntryJSON;
  bot_count?: number;
  cq_tasks?: number;
  ms_per_cq?: number;
  total_tasks?: number;
  ms_per_commit?: number;
}

const JSONToBotSet = (m: BotSetJSON): BotSet => {
  return {
    dimensions: m.dimensions,
    botCount: m.bot_count || 0,
    cqTasks: m.cq_tasks || 0,
    msPerCq: m.ms_per_cq || 0,
    totalTasks: m.total_tasks || 0,
    msPerCommit: m.ms_per_commit || 0,
  };
};

export interface StatusService {
  getIncrementalCommits: (getIncrementalCommitsRequest: GetIncrementalCommitsRequest) => Promise<GetIncrementalCommitsResponse>;
  addComment: (addCommentRequest: AddCommentRequest) => Promise<AddCommentResponse>;
  deleteComment: (deleteCommentRequest: DeleteCommentRequest) => Promise<DeleteCommentResponse>;
  getAutorollerStatuses: (getAutorollerStatusesRequest: GetAutorollerStatusesRequest) => Promise<GetAutorollerStatusesResponse>;
  getBotUsage: (getBotUsageRequest: GetBotUsageRequest) => Promise<GetBotUsageResponse>;
}

export class StatusServiceClient implements StatusService {
  private hostname: string;
  private fetch: Fetch;
  private writeCamelCase: boolean;
  private pathPrefix = "/twirp/status.StatusService/";
  private optionsOverride: object;

  constructor(hostname: string, fetch: Fetch, writeCamelCase = false, optionsOverride: any = {}) {
    this.hostname = hostname;
    this.fetch = fetch;
    this.writeCamelCase = writeCamelCase;
    this.optionsOverride = optionsOverride;
  }

  getIncrementalCommits(getIncrementalCommitsRequest: GetIncrementalCommitsRequest): Promise<GetIncrementalCommitsResponse> {
    const url = this.hostname + this.pathPrefix + "GetIncrementalCommits";
    let body: GetIncrementalCommitsRequest | GetIncrementalCommitsRequestJSON = getIncrementalCommitsRequest;
    if (!this.writeCamelCase) {
      body = GetIncrementalCommitsRequestToJSON(getIncrementalCommitsRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetIncrementalCommitsResponse);
    });
  }

  addComment(addCommentRequest: AddCommentRequest): Promise<AddCommentResponse> {
    const url = this.hostname + this.pathPrefix + "AddComment";
    let body: AddCommentRequest | AddCommentRequestJSON = addCommentRequest;
    if (!this.writeCamelCase) {
      body = AddCommentRequestToJSON(addCommentRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAddCommentResponse);
    });
  }

  deleteComment(deleteCommentRequest: DeleteCommentRequest): Promise<DeleteCommentResponse> {
    const url = this.hostname + this.pathPrefix + "DeleteComment";
    let body: DeleteCommentRequest | DeleteCommentRequestJSON = deleteCommentRequest;
    if (!this.writeCamelCase) {
      body = DeleteCommentRequestToJSON(deleteCommentRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToDeleteCommentResponse);
    });
  }

  getAutorollerStatuses(getAutorollerStatusesRequest: GetAutorollerStatusesRequest): Promise<GetAutorollerStatusesResponse> {
    const url = this.hostname + this.pathPrefix + "GetAutorollerStatuses";
    let body: GetAutorollerStatusesRequest | GetAutorollerStatusesRequestJSON = getAutorollerStatusesRequest;
    if (!this.writeCamelCase) {
      body = GetAutorollerStatusesRequestToJSON(getAutorollerStatusesRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetAutorollerStatusesResponse);
    });
  }

  getBotUsage(getBotUsageRequest: GetBotUsageRequest): Promise<GetBotUsageResponse> {
    const url = this.hostname + this.pathPrefix + "GetBotUsage";
    let body: GetBotUsageRequest | GetBotUsageRequestJSON = getBotUsageRequest;
    if (!this.writeCamelCase) {
      body = GetBotUsageRequestToJSON(getBotUsageRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetBotUsageResponse);
    });
  }
}
