import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export interface IncrementalCommitsRequest {
  from: number;
  to: number;
  n: number;
  pod: string;
  repopath: string;
}

interface IncrementalCommitsRequestJSON {
  from?: number;
  to?: number;
  n?: number;
  pod?: string;
  repoPath?: string;
}

const IncrementalCommitsRequestToJSON = (m: IncrementalCommitsRequest): IncrementalCommitsRequestJSON => {
  return {
    from: m.from,
    to: m.to,
    n: m.n,
    pod: m.pod,
    repoPath: m.repopath,
  };
};

export interface IncrementalCommitsResponse {
  metadata?: ResponseMetadata;
  update?: IncrementalUpdate;
}

interface IncrementalCommitsResponseJSON {
  metadata?: ResponseMetadataJSON;
  update?: IncrementalUpdateJSON;
}

const JSONToIncrementalCommitsResponse = (m: IncrementalCommitsResponseJSON): IncrementalCommitsResponse => {
  return {
    metadata: m.metadata && JSONToResponseMetadata(m.metadata),
    update: m.update && JSONToIncrementalUpdate(m.update),
  };
};

export interface IncrementalUpdate {
  branchheads?: Branch[];
  commits?: LongCommit[];
  startover: boolean;
  swarmingurl: string;
  tasks?: Task[];
  taskschedulerurl: string;
  comments?: Comment[];
  timestamp: string;
}

interface IncrementalUpdateJSON {
  branchHeads?: BranchJSON[];
  commits?: LongCommitJSON[];
  startOver?: boolean;
  swarmingUrl?: string;
  tasks?: TaskJSON[];
  taskSchedulerUrl?: string;
  comments?: CommentJSON[];
  timestamp?: string;
}

const JSONToIncrementalUpdate = (m: IncrementalUpdateJSON): IncrementalUpdate => {
  return {
    branchheads: m.branchHeads && m.branchHeads.map(JSONToBranch),
    commits: m.commits && m.commits.map(JSONToLongCommit),
    startover: m.startOver || false,
    swarmingurl: m.swarmingUrl || "",
    tasks: m.tasks && m.tasks.map(JSONToTask),
    taskschedulerurl: m.taskSchedulerUrl || "",
    comments: m.comments && m.comments.map(JSONToComment),
    timestamp: m.timestamp || "",
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
  swarmingtaskid: string;
}

interface TaskJSON {
  commits?: string[];
  name?: string;
  id?: string;
  revision?: string;
  status?: string;
  swarmingTaskId?: string;
}

const JSONToTask = (m: TaskJSON): Task => {
  return {
    commits: m.commits,
    name: m.name || "",
    id: m.id || "",
    revision: m.revision || "",
    status: m.status || "",
    swarmingtaskid: m.swarmingTaskId || "",
  };
};

export interface LongCommit {
  hash: string;
  author: string;
  subject: string;
  parents?: string[];
  body: string;
  timestamp: string;
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
    timestamp: m.timestamp || "",
  };
};

export interface Comment {
  id: string;
  repo: string;
  revision: string;
  timestamp: string;
  user: string;
  message: string;
  deleted: boolean;
  ignorefailure: boolean;
  flaky: boolean;
  taskspecname: string;
  taskid: string;
  commithash: string;
}

interface CommentJSON {
  id?: string;
  repo?: string;
  revision?: string;
  timestamp?: string;
  user?: string;
  message?: string;
  deleted?: boolean;
  ignoreFailure?: boolean;
  flaky?: boolean;
  taskSpecName?: string;
  taskId?: string;
  commitHash?: string;
}

const JSONToComment = (m: CommentJSON): Comment => {
  return {
    id: m.id || "",
    repo: m.repo || "",
    revision: m.revision || "",
    timestamp: m.timestamp || "",
    user: m.user || "",
    message: m.message || "",
    deleted: m.deleted || false,
    ignorefailure: m.ignoreFailure || false,
    flaky: m.flaky || false,
    taskspecname: m.taskSpecName || "",
    taskid: m.taskId || "",
    commithash: m.commitHash || "",
  };
};

export interface ResponseMetadata {
  pod: string;
}

interface ResponseMetadataJSON {
  pod?: string;
}

const JSONToResponseMetadata = (m: ResponseMetadataJSON): ResponseMetadata => {
  return {
    pod: m.pod || "",
  };
};

export interface StatusFe {
  getIncrementalCommits: (incrementalCommitsRequest: IncrementalCommitsRequest) => Promise<IncrementalCommitsResponse>;
}

export class StatusFeClient implements StatusFe {
  private hostname: string;
  private fetch: Fetch;
  private writeCamelCase: boolean;
  private pathPrefix = "/twirp/status.StatusFe/";
  private optionsOverride: object;

  constructor(hostname: string, fetch: Fetch, writeCamelCase = false, optionsOverride: any = {}) {
    this.hostname = hostname;
    this.fetch = fetch;
    this.writeCamelCase = writeCamelCase;
    this.optionsOverride = optionsOverride;
  }

  getIncrementalCommits(incrementalCommitsRequest: IncrementalCommitsRequest): Promise<IncrementalCommitsResponse> {
    const url = this.hostname + this.pathPrefix + "GetIncrementalCommits";
    let body: IncrementalCommitsRequest | IncrementalCommitsRequestJSON = incrementalCommitsRequest;
    if (!this.writeCamelCase) {
      body = IncrementalCommitsRequestToJSON(incrementalCommitsRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToIncrementalCommitsResponse);
    });
  }
}
