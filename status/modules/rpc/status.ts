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
  swarmingUrl: string;
  taskSchedulerUrl: string;
  commits?: LongCommit[];
  branchHeads?: Branch[];
  tasks?: Task[];
  comments?: Comment[];
}

interface IncrementalUpdateJSON {
  swarming_url?: string;
  task_scheduler_url?: string;
  commits?: LongCommitJSON[];
  branch_heads?: BranchJSON[];
  tasks?: TaskJSON[];
  comments?: CommentJSON[];
}

const JSONToIncrementalUpdate = (m: IncrementalUpdateJSON): IncrementalUpdate => {
  return {
    swarmingUrl: m.swarming_url || "",
    taskSchedulerUrl: m.task_scheduler_url || "",
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
}

interface TaskJSON {
  commits?: string[];
  name?: string;
  id?: string;
  revision?: string;
  status?: string;
  swarming_task_id?: string;
}

const JSONToTask = (m: TaskJSON): Task => {
  return {
    commits: m.commits,
    name: m.name || "",
    id: m.id || "",
    revision: m.revision || "",
    status: m.status || "",
    swarmingTaskId: m.swarming_task_id || "",
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

export interface StatusService {
  getIncrementalCommits: (getIncrementalCommitsRequest: GetIncrementalCommitsRequest) => Promise<GetIncrementalCommitsResponse>;
  addComment: (addCommentRequest: AddCommentRequest) => Promise<AddCommentResponse>;
  deleteComment: (deleteCommentRequest: DeleteCommentRequest) => Promise<DeleteCommentResponse>;
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
}
