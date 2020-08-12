
import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';


export interface IncrementalCommitsRequest {
    from: string;
    to: string;
    
}

interface IncrementalCommitsRequestJSON {
    from: string;
    to: string;
    
}


const IncrementalCommitsRequestToJSON = (m: IncrementalCommitsRequest): IncrementalCommitsRequestJSON => {
    return {
        from: m.from,
        to: m.to,
        
    };
};

export interface IncrementalCommitsResponse {
    metadata: ResponseMetadata;
    update: IncrementalUpdate;
    
}

interface IncrementalCommitsResponseJSON {
    metadata: ResponseMetadataJSON;
    update: IncrementalUpdateJSON;
    
}


const JSONToIncrementalCommitsResponse = (m: IncrementalCommitsResponse | IncrementalCommitsResponseJSON): IncrementalCommitsResponse => {
    
    return {
        metadata: JSONToResponseMetadata(m.metadata),
        update: JSONToIncrementalUpdate(m.update),
        
    };
};

export interface IncrementalUpdate {
    branchheads: Branch[];
    commits: LongCommit[];
    startover: boolean;
    swarmingurl: string;
    tasks: Task[];
    taskschedulerurl: string;
    comments: Comment[];
    timestamp: string;
    
}

interface IncrementalUpdateJSON {
    branchHeads: BranchJSON[];
    commits: LongCommitJSON[];
    startOver: boolean;
    swarmingUrl: string;
    tasks: TaskJSON[];
    taskSchedulerUrl: string;
    comments: CommentJSON[];
    timestamp: string;
    
}


const JSONToIncrementalUpdate = (m: IncrementalUpdate | IncrementalUpdateJSON): IncrementalUpdate => {
    
    return {
        branchheads: ((((m as IncrementalUpdate).branchheads) ? (m as IncrementalUpdate).branchheads : (m as IncrementalUpdateJSON).branchHeads) as (Branch | BranchJSON)[]).map(JSONToBranch),
        commits: (m.commits as (LongCommit | LongCommitJSON)[]).map(JSONToLongCommit),
        startover: (((m as IncrementalUpdate).startover) ? (m as IncrementalUpdate).startover : (m as IncrementalUpdateJSON).startOver),
        swarmingurl: (((m as IncrementalUpdate).swarmingurl) ? (m as IncrementalUpdate).swarmingurl : (m as IncrementalUpdateJSON).swarmingUrl),
        tasks: (m.tasks as (Task | TaskJSON)[]).map(JSONToTask),
        taskschedulerurl: (((m as IncrementalUpdate).taskschedulerurl) ? (m as IncrementalUpdate).taskschedulerurl : (m as IncrementalUpdateJSON).taskSchedulerUrl),
        comments: (m.comments as (Comment | CommentJSON)[]).map(JSONToComment),
        timestamp: m.timestamp,
        
    };
};

export interface Branch {
    name: string;
    head: string;
    
}

interface BranchJSON {
    name: string;
    head: string;
    
}


const JSONToBranch = (m: Branch | BranchJSON): Branch => {
    
    return {
        name: m.name,
        head: m.head,
        
    };
};

export interface Task {
    commits: string[];
    name: string;
    id: string;
    revision: string;
    status: string;
    swarmingtaskid: string;
    
}

interface TaskJSON {
    commits: string[];
    name: string;
    id: string;
    revision: string;
    status: string;
    swarmingTaskId: string;
    
}


const JSONToTask = (m: Task | TaskJSON): Task => {
    
    return {
        commits: m.commits,
        name: m.name,
        id: m.id,
        revision: m.revision,
        status: m.status,
        swarmingtaskid: (((m as Task).swarmingtaskid) ? (m as Task).swarmingtaskid : (m as TaskJSON).swarmingTaskId),
        
    };
};

export interface LongCommit {
    hash: string;
    author: string;
    subject: string;
    parents: string[];
    body: string;
    timestamp: string;
    
}

interface LongCommitJSON {
    hash: string;
    author: string;
    subject: string;
    parents: string[];
    body: string;
    timestamp: string;
    
}


const JSONToLongCommit = (m: LongCommit | LongCommitJSON): LongCommit => {
    
    return {
        hash: m.hash,
        author: m.author,
        subject: m.subject,
        parents: m.parents,
        body: m.body,
        timestamp: m.timestamp,
        
    };
};

export interface CommitComments {
    
}

interface CommitCommentsJSON {
    
}


export interface Comments {
    comments: Comment[];
    
}

interface CommentsJSON {
    comments: CommentJSON[];
    
}


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
    id: string;
    repo: string;
    revision: string;
    timestamp: string;
    user: string;
    message: string;
    deleted: boolean;
    ignoreFailure: boolean;
    flaky: boolean;
    taskSpecName: string;
    taskId: string;
    commitHash: string;
    
}


const JSONToComment = (m: Comment | CommentJSON): Comment => {
    
    return {
        id: m.id,
        repo: m.repo,
        revision: m.revision,
        timestamp: m.timestamp,
        user: m.user,
        message: m.message,
        deleted: m.deleted,
        ignorefailure: (((m as Comment).ignorefailure) ? (m as Comment).ignorefailure : (m as CommentJSON).ignoreFailure),
        flaky: m.flaky,
        taskspecname: (((m as Comment).taskspecname) ? (m as Comment).taskspecname : (m as CommentJSON).taskSpecName),
        taskid: (((m as Comment).taskid) ? (m as Comment).taskid : (m as CommentJSON).taskId),
        commithash: (((m as Comment).commithash) ? (m as Comment).commithash : (m as CommentJSON).commitHash),
        
    };
};

export interface ResponseMetadata {
    pod: string;
    
}

interface ResponseMetadataJSON {
    pod: string;
    
}


const JSONToResponseMetadata = (m: ResponseMetadata | ResponseMetadataJSON): ResponseMetadata => {
    
    return {
        pod: m.pod,
        
    };
};

export interface StatusFe {
    getIncrementalCommits: (incrementalCommitsRequest: IncrementalCommitsRequest) => Promise<IncrementalCommitsResponse>;
    
}

export class DefaultStatusFe implements StatusFe {
    private hostname: string;
    private fetch: Fetch;
    private writeCamelCase: boolean;
    private pathPrefix = "/twirp/status.StatusFe/";

    constructor(hostname: string, fetch: Fetch, writeCamelCase = false) {
        this.hostname = hostname;
        this.fetch = fetch;
        this.writeCamelCase = writeCamelCase;
    }
    getIncrementalCommits(incrementalCommitsRequest: IncrementalCommitsRequest): Promise<IncrementalCommitsResponse> {
        const url = this.hostname + this.pathPrefix + "GetIncrementalCommits";
        let body: IncrementalCommitsRequest | IncrementalCommitsRequestJSON = incrementalCommitsRequest;
        if (!this.writeCamelCase) {
            body = IncrementalCommitsRequestToJSON(incrementalCommitsRequest);
        }
        return this.fetch(createTwirpRequest(url, body)).then((resp) => {
            if (!resp.ok) {
                return throwTwirpError(resp);
            }

            return resp.json().then(JSONToIncrementalCommitsResponse);
        });
    }
    
}

