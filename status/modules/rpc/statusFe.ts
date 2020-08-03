
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
    startover: boolean;
    swarmingurl: string;
    tasks: Task[];
    taskschedulerurl: string;
    
}

interface IncrementalUpdateJSON {
    startOver: boolean;
    swarmingUrl: string;
    tasks: TaskJSON[];
    taskSchedulerUrl: string;
    
}


const JSONToIncrementalUpdate = (m: IncrementalUpdate | IncrementalUpdateJSON): IncrementalUpdate => {
    
    return {
        startover: (((m as IncrementalUpdate).startover) ? (m as IncrementalUpdate).startover : (m as IncrementalUpdateJSON).startOver),
        swarmingurl: (((m as IncrementalUpdate).swarmingurl) ? (m as IncrementalUpdate).swarmingurl : (m as IncrementalUpdateJSON).swarmingUrl),
        tasks: (m.tasks as (Task | TaskJSON)[]).map(JSONToTask),
        taskschedulerurl: (((m as IncrementalUpdate).taskschedulerurl) ? (m as IncrementalUpdate).taskschedulerurl : (m as IncrementalUpdateJSON).taskSchedulerUrl),
        
    };
};

export interface Task {
    commits: string[];
    name: string;
    id: string;
    revision: string;
    swarmingtaskid: string;
    
}

interface TaskJSON {
    commits: string[];
    name: string;
    id: string;
    revision: string;
    swarmingTaskId: string;
    
}


const JSONToTask = (m: Task | TaskJSON): Task => {
    
    return {
        commits: m.commits,
        name: m.name,
        id: m.id,
        revision: m.revision,
        swarmingtaskid: (((m as Task).swarmingtaskid) ? (m as Task).swarmingtaskid : (m as TaskJSON).swarmingTaskId),
        
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
    private pathPrefix = "/twirp/autoroll.rpc.StatusFe/";

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

