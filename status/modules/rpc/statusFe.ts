import { createTwirpRequest, throwTwirpError, Fetch } from './twirp';

export interface IncrementalCommitsRequest {
  from: string;
  to: string;
}

interface IncrementalCommitsRequestJSON {
  from?: string;
  to?: string;
}

const IncrementalCommitsRequestToJSON = (m: IncrementalCommitsRequest): IncrementalCommitsRequestJSON => ({
  from: m.from,
  to: m.to,
});

export interface IncrementalCommitsResponse {
  metadata: string;
  data?: string[];
}

interface IncrementalCommitsResponseJSON {
  metadata?: string;
  data?: string[];
}

const JSONToIncrementalCommitsResponse = (m: IncrementalCommitsResponseJSON): IncrementalCommitsResponse => ({
  metadata: m.metadata || '',
  data: m.data,
});

export interface StatusFe {
  getIncrementalCommits: (incrementalCommitsRequest: IncrementalCommitsRequest)=> Promise<IncrementalCommitsResponse>;
}

export class StatusFeClient implements StatusFe {
  private hostname: string;

  private fetch: Fetch;

  private writeCamelCase: boolean;

  private pathPrefix = '/twirp/status.rpc.StatusFe/';

  private optionsOverride: object;

  constructor(hostname: string, fetch: Fetch, writeCamelCase = false, optionsOverride: any = {}) {
    this.hostname = hostname;
    this.fetch = fetch;
    this.writeCamelCase = writeCamelCase;
    this.optionsOverride = optionsOverride;
  }

  getIncrementalCommits(incrementalCommitsRequest: IncrementalCommitsRequest): Promise<IncrementalCommitsResponse> {
    const url = `${this.hostname + this.pathPrefix}GetIncrementalCommits`;
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
