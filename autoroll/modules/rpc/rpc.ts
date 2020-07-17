import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export interface AutoRollMiniStatus {
  roller: string;
  childname: string;
  parentname: string;
  mode: string;
  currentrollrev: string;
  lastrollrev: string;
  numfailed: number;
  numbehind: number;
}

interface AutoRollMiniStatusJSON {
  roller?: string;
  childName?: string;
  parentName?: string;
  mode?: string;
  currentRollRev?: string;
  lastRollRev?: string;
  numFailed?: number;
  numBehind?: number;
}

const JSONToAutoRollMiniStatus = (m: AutoRollMiniStatusJSON): AutoRollMiniStatus => {
  return {
    roller: m.roller || "",
    childname: m.childName || "",
    parentname: m.parentName || "",
    mode: m.mode || "",
    currentrollrev: m.currentRollRev || "",
    lastrollrev: m.lastRollRev || "",
    numfailed: m.numFailed || 0,
    numbehind: m.numBehind || 0,
  };
};

export interface AutoRollMiniStatuses {
  statuses?: AutoRollMiniStatus[];
}

interface AutoRollMiniStatusesJSON {
  statuses?: AutoRollMiniStatusJSON[];
}

const JSONToAutoRollMiniStatuses = (m: AutoRollMiniStatusesJSON): AutoRollMiniStatuses => {
  return {
    statuses: m.statuses && m.statuses.map(JSONToAutoRollMiniStatus),
  };
};

export interface TryResult {
  name: string;
  status: string;
  result: string;
  url: string;
  category: string;
}

interface TryResultJSON {
  name?: string;
  status?: string;
  result?: string;
  url?: string;
  category?: string;
}

const JSONToTryResult = (m: TryResultJSON): TryResult => {
  return {
    name: m.name || "",
    status: m.status || "",
    result: m.result || "",
    url: m.url || "",
    category: m.category || "",
  };
};

export interface AutoRollCL {
  id: string;
  result: string;
  subject: string;
  rollingto: string;
  rollingfrom: string;
  created: number;
  modified: number;
  tryresults?: TryResult[];
}

interface AutoRollCLJSON {
  id?: string;
  result?: string;
  subject?: string;
  rollingTo?: string;
  rollingFrom?: string;
  created?: number;
  modified?: number;
  tryResults?: TryResultJSON[];
}

const JSONToAutoRollCL = (m: AutoRollCLJSON): AutoRollCL => {
  return {
    id: m.id || "",
    result: m.result || "",
    subject: m.subject || "",
    rollingto: m.rollingTo || "",
    rollingfrom: m.rollingFrom || "",
    created: m.created || 0,
    modified: m.modified || 0,
    tryresults: m.tryResults && m.tryResults.map(JSONToTryResult),
  };
};

export interface Revision {
  id: string;
  display: string;
  description: string;
  time: number;
  url: string;
}

interface RevisionJSON {
  id?: string;
  display?: string;
  description?: string;
  time?: number;
  url?: string;
}

const JSONToRevision = (m: RevisionJSON): Revision => {
  return {
    id: m.id || "",
    display: m.display || "",
    description: m.description || "",
    time: m.time || 0,
    url: m.url || "",
  };
};

export interface AutoRollConfig {
  parentwaterfall: string;
  supportsmanualrolls: boolean;
  timewindow: string;
}

interface AutoRollConfigJSON {
  parentWaterfall?: string;
  supportsManualRolls?: boolean;
  timeWindow?: string;
}

const JSONToAutoRollConfig = (m: AutoRollConfigJSON): AutoRollConfig => {
  return {
    parentwaterfall: m.parentWaterfall || "",
    supportsmanualrolls: m.supportsManualRolls || false,
    timewindow: m.timeWindow || "",
  };
};

export interface ModeChange {
  roller: string;
  mode: string;
  user: string;
  time: number;
  message: string;
}

interface ModeChangeJSON {
  roller?: string;
  mode?: string;
  user?: string;
  time?: number;
  message?: string;
}

const JSONToModeChange = (m: ModeChangeJSON): ModeChange => {
  return {
    roller: m.roller || "",
    mode: m.mode || "",
    user: m.user || "",
    time: m.time || 0,
    message: m.message || "",
  };
};

export interface StrategyChange {
  roller: string;
  strategy: string;
  user: string;
  time: number;
  message: string;
}

interface StrategyChangeJSON {
  roller?: string;
  strategy?: string;
  user?: string;
  time?: number;
  message?: string;
}

const JSONToStrategyChange = (m: StrategyChangeJSON): StrategyChange => {
  return {
    roller: m.roller || "",
    strategy: m.strategy || "",
    user: m.user || "",
    time: m.time || 0,
    message: m.message || "",
  };
};

export interface ManualRollRequest {
  id: string;
  roller: string;
  revision: string;
  requester: string;
  result: string;
  status: string;
  timestamp: number;
  url: string;
  dryrun: boolean;
  noemail: boolean;
  noresolverevision: boolean;
}

interface ManualRollRequestJSON {
  id?: string;
  roller?: string;
  revision?: string;
  requester?: string;
  result?: string;
  status?: string;
  timestamp?: number;
  url?: string;
  dryRun?: boolean;
  noEmail?: boolean;
  noResolveRevision?: boolean;
}

const JSONToManualRollRequest = (m: ManualRollRequestJSON): ManualRollRequest => {
  return {
    id: m.id || "",
    roller: m.roller || "",
    revision: m.revision || "",
    requester: m.requester || "",
    result: m.result || "",
    status: m.status || "",
    timestamp: m.timestamp || 0,
    url: m.url || "",
    dryrun: m.dryRun || false,
    noemail: m.noEmail || false,
    noresolverevision: m.noResolveRevision || false,
  };
};

export interface AutoRollStatus {
  ministatus?: AutoRollMiniStatus;
  status: string;
  config?: AutoRollConfig;
  childhead: string;
  fullhistoryurl: string;
  issueurlbase: string;
  mode?: ModeChange;
  strategy?: StrategyChange;
  notrolledrevisions?: Revision[];
  currentroll?: AutoRollCL;
  lastroll?: AutoRollCL;
  recent?: AutoRollCL[];
  validmodes?: string[];
  validstrategies?: string[];
  manualrequests?: ManualRollRequest[];
  error: string;
  throttleduntil: number;
}

interface AutoRollStatusJSON {
  miniStatus?: AutoRollMiniStatusJSON;
  status?: string;
  config?: AutoRollConfigJSON;
  childHead?: string;
  fullHistoryUrl?: string;
  issueUrlBase?: string;
  mode?: ModeChangeJSON;
  strategy?: StrategyChangeJSON;
  notRolledRevisions?: RevisionJSON[];
  currentRoll?: AutoRollCLJSON;
  LastRoll?: AutoRollCLJSON;
  recent?: AutoRollCLJSON[];
  validModes?: string[];
  validStrategies?: string[];
  manualRequests?: ManualRollRequestJSON[];
  error?: string;
  throttledUntil?: number;
}

const JSONToAutoRollStatus = (m: AutoRollStatusJSON): AutoRollStatus => {
  return {
    ministatus: m.miniStatus && JSONToAutoRollMiniStatus(m.miniStatus),
    status: m.status || "",
    config: m.config && JSONToAutoRollConfig(m.config),
    childhead: m.childHead || "",
    fullhistoryurl: m.fullHistoryUrl || "",
    issueurlbase: m.issueUrlBase || "",
    mode: m.mode && JSONToModeChange(m.mode),
    strategy: m.strategy && JSONToStrategyChange(m.strategy),
    notrolledrevisions: m.notRolledRevisions && m.notRolledRevisions.map(JSONToRevision),
    currentroll: m.currentRoll && JSONToAutoRollCL(m.currentRoll),
    lastroll: m.LastRoll && JSONToAutoRollCL(m.LastRoll),
    recent: m.recent && m.recent.map(JSONToAutoRollCL),
    validmodes: m.validModes,
    validstrategies: m.validStrategies,
    manualrequests: m.manualRequests && m.manualRequests.map(JSONToManualRollRequest),
    error: m.error || "",
    throttleduntil: m.throttledUntil || 0,
  };
};

export interface GetRollersRequest {
}

interface GetRollersRequestJSON {
}

const GetRollersRequestToJSON = (m: GetRollersRequest): GetRollersRequestJSON => {
  return {
  };
};

export interface GetMiniStatusRequest {
  roller: string;
}

interface GetMiniStatusRequestJSON {
  roller?: string;
}

const GetMiniStatusRequestToJSON = (m: GetMiniStatusRequest): GetMiniStatusRequestJSON => {
  return {
    roller: m.roller,
  };
};

export interface GetStatusRequest {
  roller: string;
}

interface GetStatusRequestJSON {
  roller?: string;
}

const GetStatusRequestToJSON = (m: GetStatusRequest): GetStatusRequestJSON => {
  return {
    roller: m.roller,
  };
};

export interface SetModeRequest {
  roller: string;
  mode: string;
  user: string;
  message: string;
}

interface SetModeRequestJSON {
  roller?: string;
  mode?: string;
  user?: string;
  message?: string;
}

const SetModeRequestToJSON = (m: SetModeRequest): SetModeRequestJSON => {
  return {
    roller: m.roller,
    mode: m.mode,
    user: m.user,
    message: m.message,
  };
};

export interface SetStrategyRequest {
  roller: string;
  strategy: string;
  user: string;
  message: string;
}

interface SetStrategyRequestJSON {
  roller?: string;
  strategy?: string;
  user?: string;
  message?: string;
}

const SetStrategyRequestToJSON = (m: SetStrategyRequest): SetStrategyRequestJSON => {
  return {
    roller: m.roller,
    strategy: m.strategy,
    user: m.user,
    message: m.message,
  };
};

export interface CreateManualRollRequest {
  roller: string;
  revision: string;
  user: string;
}

interface CreateManualRollRequestJSON {
  roller?: string;
  revision?: string;
  user?: string;
}

const CreateManualRollRequestToJSON = (m: CreateManualRollRequest): CreateManualRollRequestJSON => {
  return {
    roller: m.roller,
    revision: m.revision,
    user: m.user,
  };
};

export interface UnthrottleRequest {
  roller: string;
}

interface UnthrottleRequestJSON {
  roller?: string;
}

const UnthrottleRequestToJSON = (m: UnthrottleRequest): UnthrottleRequestJSON => {
  return {
    roller: m.roller,
  };
};

export interface UnthrottleResponse {
}

interface UnthrottleResponseJSON {
}

const JSONToUnthrottleResponse = (m: UnthrottleResponseJSON): UnthrottleResponse => {
  return {
  };
};

export interface AutoRollRPCs {
  view_GetRollers: (getRollersRequest: GetRollersRequest) => Promise<AutoRollMiniStatuses>;
  view_GetMiniStatus: (getMiniStatusRequest: GetMiniStatusRequest) => Promise<AutoRollMiniStatus>;
  view_GetStatus: (getStatusRequest: GetStatusRequest) => Promise<AutoRollStatus>;
  edit_SetMode: (setModeRequest: SetModeRequest) => Promise<AutoRollStatus>;
  edit_SetStrategy: (setStrategyRequest: SetStrategyRequest) => Promise<AutoRollStatus>;
  edit_CreateManualRoll: (createManualRollRequest: CreateManualRollRequest) => Promise<ManualRollRequest>;
  edit_Unthrottle: (unthrottleRequest: UnthrottleRequest) => Promise<UnthrottleResponse>;
}

export class AutoRollRPCsClient implements AutoRollRPCs {
  private hostname: string;
  private fetch: Fetch;
  private writeCamelCase: boolean;
  private pathPrefix = "/twirp/autoroll.rpc.AutoRollRPCs/";
  private optionsOverride: object;

  constructor(hostname: string, fetch: Fetch, writeCamelCase = false, optionsOverride: any = {}) {
    this.hostname = hostname;
    this.fetch = fetch;
    this.writeCamelCase = writeCamelCase;
    this.optionsOverride = optionsOverride;
  }

  view_GetRollers(getRollersRequest: GetRollersRequest): Promise<AutoRollMiniStatuses> {
    const url = this.hostname + this.pathPrefix + "View_GetRollers";
    let body: GetRollersRequest | GetRollersRequestJSON = getRollersRequest;
    if (!this.writeCamelCase) {
      body = GetRollersRequestToJSON(getRollersRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAutoRollMiniStatuses);
    });
  }

  view_GetMiniStatus(getMiniStatusRequest: GetMiniStatusRequest): Promise<AutoRollMiniStatus> {
    const url = this.hostname + this.pathPrefix + "View_GetMiniStatus";
    let body: GetMiniStatusRequest | GetMiniStatusRequestJSON = getMiniStatusRequest;
    if (!this.writeCamelCase) {
      body = GetMiniStatusRequestToJSON(getMiniStatusRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAutoRollMiniStatus);
    });
  }

  view_GetStatus(getStatusRequest: GetStatusRequest): Promise<AutoRollStatus> {
    const url = this.hostname + this.pathPrefix + "View_GetStatus";
    let body: GetStatusRequest | GetStatusRequestJSON = getStatusRequest;
    if (!this.writeCamelCase) {
      body = GetStatusRequestToJSON(getStatusRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        console.log("THIS IS NOT OKAY");
        console.log(resp);
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAutoRollStatus);
    });
  }

  edit_SetMode(setModeRequest: SetModeRequest): Promise<AutoRollStatus> {
    const url = this.hostname + this.pathPrefix + "Edit_SetMode";
    let body: SetModeRequest | SetModeRequestJSON = setModeRequest;
    if (!this.writeCamelCase) {
      body = SetModeRequestToJSON(setModeRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAutoRollStatus);
    });
  }

  edit_SetStrategy(setStrategyRequest: SetStrategyRequest): Promise<AutoRollStatus> {
    const url = this.hostname + this.pathPrefix + "Edit_SetStrategy";
    let body: SetStrategyRequest | SetStrategyRequestJSON = setStrategyRequest;
    if (!this.writeCamelCase) {
      body = SetStrategyRequestToJSON(setStrategyRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToAutoRollStatus);
    });
  }

  edit_CreateManualRoll(createManualRollRequest: CreateManualRollRequest): Promise<ManualRollRequest> {
    const url = this.hostname + this.pathPrefix + "Edit_CreateManualRoll";
    let body: CreateManualRollRequest | CreateManualRollRequestJSON = createManualRollRequest;
    if (!this.writeCamelCase) {
      body = CreateManualRollRequestToJSON(createManualRollRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToManualRollRequest);
    });
  }

  edit_Unthrottle(unthrottleRequest: UnthrottleRequest): Promise<UnthrottleResponse> {
    const url = this.hostname + this.pathPrefix + "Edit_Unthrottle";
    let body: UnthrottleRequest | UnthrottleRequestJSON = unthrottleRequest;
    if (!this.writeCamelCase) {
      body = UnthrottleRequestToJSON(unthrottleRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToUnthrottleResponse);
    });
  }
}
