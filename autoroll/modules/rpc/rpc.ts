import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export enum Mode {
  MODE_RUNNING = "MODE_RUNNING",
  MODE_STOPPED = "MODE_STOPPED",
  MODE_DRY_RUN = "MODE_DRY_RUN",
}

export enum Strategy {
  STRATEGY_BATCH = "STRATEGY_BATCH",
  STRATEGY_N_BATCH = "STRATEGY_N_BATCH",
  STRATEGY_SINGLE = "STRATEGY_SINGLE",
}

export interface AutoRollMiniStatus {
  roller: string;
  childName: string;
  parentName: string;
  mode: Mode;
  currentRollRev: string;
  lastRollRev: string;
  numFailed: number;
  numBehind: number;
}

interface AutoRollMiniStatusJSON {
  roller?: string;
  child_name?: string;
  parent_name?: string;
  mode?: string;
  current_roll_rev?: string;
  last_roll_rev?: string;
  num_failed?: number;
  num_behind?: number;
}

const JSONToAutoRollMiniStatus = (m: AutoRollMiniStatusJSON): AutoRollMiniStatus => {
  return {
    roller: m.roller || "",
    childName: m.child_name || "",
    parentName: m.parent_name || "",
    mode: m.mode as Mode,
    currentRollRev: m.current_roll_rev || "",
    lastRollRev: m.last_roll_rev || "",
    numFailed: m.num_failed || 0,
    numBehind: m.num_behind || 0,
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
  rollingTo: string;
  rollingFrom: string;
  created?: string;
  modified?: string;
  tryResults?: TryResult[];
}

interface AutoRollCLJSON {
  id?: string;
  result?: string;
  subject?: string;
  rolling_to?: string;
  rolling_from?: string;
  created?: string;
  modified?: string;
  try_results?: TryResultJSON[];
}

const JSONToAutoRollCL = (m: AutoRollCLJSON): AutoRollCL => {
  return {
    id: m.id || "",
    result: m.result || "",
    subject: m.subject || "",
    rollingTo: m.rolling_to || "",
    rollingFrom: m.rolling_from || "",
    created: m.created,
    modified: m.modified,
    tryResults: m.try_results && m.try_results.map(JSONToTryResult),
  };
};

export interface Revision {
  id: string;
  display: string;
  description: string;
  time?: string;
  url: string;
}

interface RevisionJSON {
  id?: string;
  display?: string;
  description?: string;
  time?: string;
  url?: string;
}

const JSONToRevision = (m: RevisionJSON): Revision => {
  return {
    id: m.id || "",
    display: m.display || "",
    description: m.description || "",
    time: m.time,
    url: m.url || "",
  };
};

export interface AutoRollConfig {
  parentWaterfall: string;
  rollerName: string;
  supportsManualRolls: boolean;
  timeWindow: string;
}

interface AutoRollConfigJSON {
  parent_waterfall?: string;
  roller_name?: string;
  supports_manual_rolls?: boolean;
  time_window?: string;
}

const JSONToAutoRollConfig = (m: AutoRollConfigJSON): AutoRollConfig => {
  return {
    parentWaterfall: m.parent_waterfall || "",
    rollerName: m.roller_name || "",
    supportsManualRolls: m.supports_manual_rolls || false,
    timeWindow: m.time_window || "",
  };
};

export interface ModeChange {
  roller: string;
  mode: Mode;
  user: string;
  time?: string;
  message: string;
}

interface ModeChangeJSON {
  roller?: string;
  mode?: string;
  user?: string;
  time?: string;
  message?: string;
}

const JSONToModeChange = (m: ModeChangeJSON): ModeChange => {
  return {
    roller: m.roller || "",
    mode: m.mode as Mode,
    user: m.user || "",
    time: m.time,
    message: m.message || "",
  };
};

export interface StrategyChange {
  roller: string;
  strategy: Strategy;
  user: string;
  time?: string;
  message: string;
}

interface StrategyChangeJSON {
  roller?: string;
  strategy?: string;
  user?: string;
  time?: string;
  message?: string;
}

const JSONToStrategyChange = (m: StrategyChangeJSON): StrategyChange => {
  return {
    roller: m.roller || "",
    strategy: m.strategy as Strategy,
    user: m.user || "",
    time: m.time,
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
  timestamp?: string;
  url: string;
  dryRun: boolean;
  noEmail: boolean;
  noResolveRevision: boolean;
}

interface ManualRollRequestJSON {
  id?: string;
  roller?: string;
  revision?: string;
  requester?: string;
  result?: string;
  status?: string;
  timestamp?: string;
  url?: string;
  dry_run?: boolean;
  no_email?: boolean;
  no_resolve_revision?: boolean;
}

const JSONToManualRollRequest = (m: ManualRollRequestJSON): ManualRollRequest => {
  return {
    id: m.id || "",
    roller: m.roller || "",
    revision: m.revision || "",
    requester: m.requester || "",
    result: m.result || "",
    status: m.status || "",
    timestamp: m.timestamp,
    url: m.url || "",
    dryRun: m.dry_run || false,
    noEmail: m.no_email || false,
    noResolveRevision: m.no_resolve_revision || false,
  };
};

export interface AutoRollStatus {
  miniStatus?: AutoRollMiniStatus;
  status: string;
  config?: AutoRollConfig;
  childhead: string;
  fullHistoryUrl: string;
  issueUrlBase: string;
  mode?: ModeChange;
  strategy?: StrategyChange;
  notRolledRevisions?: Revision[];
  currentRoll?: AutoRollCL;
  lastRoll?: AutoRollCL;
  recent?: AutoRollCL[];
  validModes?: string[];
  validStrategies?: string[];
  manualRequests?: ManualRollRequest[];
  error: string;
  throttledUntil?: string;
}

interface AutoRollStatusJSON {
  mini_status?: AutoRollMiniStatusJSON;
  status?: string;
  config?: AutoRollConfigJSON;
  childHead?: string;
  full_history_url?: string;
  issue_url_base?: string;
  mode?: ModeChangeJSON;
  strategy?: StrategyChangeJSON;
  not_rolled_revisions?: RevisionJSON[];
  current_roll?: AutoRollCLJSON;
  last_roll?: AutoRollCLJSON;
  recent?: AutoRollCLJSON[];
  valid_modes?: string[];
  valid_strategies?: string[];
  manual_requests?: ManualRollRequestJSON[];
  error?: string;
  throttled_until?: string;
}

const JSONToAutoRollStatus = (m: AutoRollStatusJSON): AutoRollStatus => {
  return {
    miniStatus: m.mini_status && JSONToAutoRollMiniStatus(m.mini_status),
    status: m.status || "",
    config: m.config && JSONToAutoRollConfig(m.config),
    childhead: m.childHead || "",
    fullHistoryUrl: m.full_history_url || "",
    issueUrlBase: m.issue_url_base || "",
    mode: m.mode && JSONToModeChange(m.mode),
    strategy: m.strategy && JSONToStrategyChange(m.strategy),
    notRolledRevisions: m.not_rolled_revisions && m.not_rolled_revisions.map(JSONToRevision),
    currentRoll: m.current_roll && JSONToAutoRollCL(m.current_roll),
    lastRoll: m.last_roll && JSONToAutoRollCL(m.last_roll),
    recent: m.recent && m.recent.map(JSONToAutoRollCL),
    validModes: m.valid_modes,
    validStrategies: m.valid_strategies,
    manualRequests: m.manual_requests && m.manual_requests.map(JSONToManualRollRequest),
    error: m.error || "",
    throttledUntil: m.throttled_until,
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

export interface GetRollersResponse {
  rollers?: AutoRollMiniStatus[];
}

interface GetRollersResponseJSON {
  rollers?: AutoRollMiniStatusJSON[];
}

const JSONToGetRollersResponse = (m: GetRollersResponseJSON): GetRollersResponse => {
  return {
    rollers: m.rollers && m.rollers.map(JSONToAutoRollMiniStatus),
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

export interface GetMiniStatusResponse {
  status?: AutoRollMiniStatus;
}

interface GetMiniStatusResponseJSON {
  status?: AutoRollMiniStatusJSON;
}

const JSONToGetMiniStatusResponse = (m: GetMiniStatusResponseJSON): GetMiniStatusResponse => {
  return {
    status: m.status && JSONToAutoRollMiniStatus(m.status),
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

export interface GetStatusResponse {
  status?: AutoRollStatus;
}

interface GetStatusResponseJSON {
  status?: AutoRollStatusJSON;
}

const JSONToGetStatusResponse = (m: GetStatusResponseJSON): GetStatusResponse => {
  return {
    status: m.status && JSONToAutoRollStatus(m.status),
  };
};

export interface SetModeRequest {
  roller: string;
  mode: Mode;
  message: string;
}

interface SetModeRequestJSON {
  roller?: string;
  mode?: string;
  message?: string;
}

const SetModeRequestToJSON = (m: SetModeRequest): SetModeRequestJSON => {
  return {
    roller: m.roller,
    mode: m.mode,
    message: m.message,
  };
};

export interface SetModeResponse {
  status?: AutoRollStatus;
}

interface SetModeResponseJSON {
  status?: AutoRollStatusJSON;
}

const JSONToSetModeResponse = (m: SetModeResponseJSON): SetModeResponse => {
  return {
    status: m.status && JSONToAutoRollStatus(m.status),
  };
};

export interface SetStrategyRequest {
  roller: string;
  strategy: Strategy;
  message: string;
}

interface SetStrategyRequestJSON {
  roller?: string;
  strategy?: string;
  message?: string;
}

const SetStrategyRequestToJSON = (m: SetStrategyRequest): SetStrategyRequestJSON => {
  return {
    roller: m.roller,
    strategy: m.strategy,
    message: m.message,
  };
};

export interface SetStrategyResponse {
  status?: AutoRollStatus;
}

interface SetStrategyResponseJSON {
  status?: AutoRollStatusJSON;
}

const JSONToSetStrategyResponse = (m: SetStrategyResponseJSON): SetStrategyResponse => {
  return {
    status: m.status && JSONToAutoRollStatus(m.status),
  };
};

export interface CreateManualRollRequest {
  roller: string;
  revision: string;
}

interface CreateManualRollRequestJSON {
  roller?: string;
  revision?: string;
}

const CreateManualRollRequestToJSON = (m: CreateManualRollRequest): CreateManualRollRequestJSON => {
  return {
    roller: m.roller,
    revision: m.revision,
  };
};

export interface CreateManualRollResponse {
  request?: ManualRollRequest;
}

interface CreateManualRollResponseJSON {
  request?: ManualRollRequestJSON;
}

const JSONToCreateManualRollResponse = (m: CreateManualRollResponseJSON): CreateManualRollResponse => {
  return {
    request: m.request && JSONToManualRollRequest(m.request),
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

export interface AutoRollService {
  getRollers: (getRollersRequest: GetRollersRequest) => Promise<GetRollersResponse>;
  getMiniStatus: (getMiniStatusRequest: GetMiniStatusRequest) => Promise<GetMiniStatusResponse>;
  getStatus: (getStatusRequest: GetStatusRequest) => Promise<GetStatusResponse>;
  setMode: (setModeRequest: SetModeRequest) => Promise<SetModeResponse>;
  setStrategy: (setStrategyRequest: SetStrategyRequest) => Promise<SetStrategyResponse>;
  createManualRoll: (createManualRollRequest: CreateManualRollRequest) => Promise<CreateManualRollResponse>;
  unthrottle: (unthrottleRequest: UnthrottleRequest) => Promise<UnthrottleResponse>;
}

export class AutoRollServiceClient implements AutoRollService {
  private hostname: string;
  private fetch: Fetch;
  private writeCamelCase: boolean;
  private pathPrefix = "/twirp/autoroll.rpc.AutoRollService/";
  private optionsOverride: object;

  constructor(hostname: string, fetch: Fetch, writeCamelCase = false, optionsOverride: any = {}) {
    this.hostname = hostname;
    this.fetch = fetch;
    this.writeCamelCase = writeCamelCase;
    this.optionsOverride = optionsOverride;
  }

  getRollers(getRollersRequest: GetRollersRequest): Promise<GetRollersResponse> {
    const url = this.hostname + this.pathPrefix + "GetRollers";
    let body: GetRollersRequest | GetRollersRequestJSON = getRollersRequest;
    if (!this.writeCamelCase) {
      body = GetRollersRequestToJSON(getRollersRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetRollersResponse);
    });
  }

  getMiniStatus(getMiniStatusRequest: GetMiniStatusRequest): Promise<GetMiniStatusResponse> {
    const url = this.hostname + this.pathPrefix + "GetMiniStatus";
    let body: GetMiniStatusRequest | GetMiniStatusRequestJSON = getMiniStatusRequest;
    if (!this.writeCamelCase) {
      body = GetMiniStatusRequestToJSON(getMiniStatusRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetMiniStatusResponse);
    });
  }

  getStatus(getStatusRequest: GetStatusRequest): Promise<GetStatusResponse> {
    const url = this.hostname + this.pathPrefix + "GetStatus";
    let body: GetStatusRequest | GetStatusRequestJSON = getStatusRequest;
    if (!this.writeCamelCase) {
      body = GetStatusRequestToJSON(getStatusRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetStatusResponse);
    });
  }

  setMode(setModeRequest: SetModeRequest): Promise<SetModeResponse> {
    const url = this.hostname + this.pathPrefix + "SetMode";
    let body: SetModeRequest | SetModeRequestJSON = setModeRequest;
    if (!this.writeCamelCase) {
      body = SetModeRequestToJSON(setModeRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToSetModeResponse);
    });
  }

  setStrategy(setStrategyRequest: SetStrategyRequest): Promise<SetStrategyResponse> {
    const url = this.hostname + this.pathPrefix + "SetStrategy";
    let body: SetStrategyRequest | SetStrategyRequestJSON = setStrategyRequest;
    if (!this.writeCamelCase) {
      body = SetStrategyRequestToJSON(setStrategyRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToSetStrategyResponse);
    });
  }

  createManualRoll(createManualRollRequest: CreateManualRollRequest): Promise<CreateManualRollResponse> {
    const url = this.hostname + this.pathPrefix + "CreateManualRoll";
    let body: CreateManualRollRequest | CreateManualRollRequestJSON = createManualRollRequest;
    if (!this.writeCamelCase) {
      body = CreateManualRollRequestToJSON(createManualRollRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToCreateManualRollResponse);
    });
  }

  unthrottle(unthrottleRequest: UnthrottleRequest): Promise<UnthrottleResponse> {
    const url = this.hostname + this.pathPrefix + "Unthrottle";
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
