import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export enum Mode {
  RUNNING = "RUNNING",
  STOPPED = "STOPPED",
  DRY_RUN = "DRY_RUN",
}

export enum Strategy {
  BATCH = "BATCH",
  N_BATCH = "N_BATCH",
  SINGLE = "SINGLE",
}

export enum TryJob_Result {
  UNKNOWN = "UNKNOWN",
  SUCCESS = "SUCCESS",
  FAILURE = "FAILURE",
  CANCELED = "CANCELED",
}

export enum TryJob_Status {
  SCHEDULED = "SCHEDULED",
  STARTED = "STARTED",
  COMPLETED = "COMPLETED",
}

export enum AutoRollCL_Result {
  IN_PROGRESS = "IN_PROGRESS",
  SUCCESS = "SUCCESS",
  FAILURE = "FAILURE",
  DRY_RUN_IN_PROGRESS = "DRY_RUN_IN_PROGRESS",
  DRY_RUN_SUCCESS = "DRY_RUN_SUCCESS",
  DRY_RUN_FAILURE = "DRY_RUN_FAILURE",
}

export enum ManualRoll_Result {
  UNKNOWN = "UNKNOWN",
  FAILURE = "FAILURE",
  SUCCESS = "SUCCESS",
}

export enum ManualRoll_Status {
  PENDING = "PENDING",
  STARTED = "STARTED",
  COMPLETED = "COMPLETED",
}

export interface AutoRollMiniStatus {
  rollerId: string;
  childName: string;
  parentName: string;
  mode: Mode;
  currentRollRev: string;
  lastRollRev: string;
  numFailed: number;
  numBehind: number;
}

interface AutoRollMiniStatusJSON {
  roller_id?: string;
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
    rollerId: m.roller_id || "",
    childName: m.child_name || "",
    parentName: m.parent_name || "",
    mode: (m.mode || Object.keys(Mode)[0]) as Mode,
    currentRollRev: m.current_roll_rev || "",
    lastRollRev: m.last_roll_rev || "",
    numFailed: m.num_failed || 0,
    numBehind: m.num_behind || 0,
  };
};

export interface TryJob {
  name: string;
  status: TryJob_Status;
  result: TryJob_Result;
  url: string;
  category: string;
}

interface TryJobJSON {
  name?: string;
  status?: string;
  result?: string;
  url?: string;
  category?: string;
}

const JSONToTryJob = (m: TryJobJSON): TryJob => {
  return {
    name: m.name || "",
    status: (m.status || Object.keys(TryJob_Status)[0]) as TryJob_Status,
    result: (m.result || Object.keys(TryJob_Result)[0]) as TryJob_Result,
    url: m.url || "",
    category: m.category || "",
  };
};

export interface AutoRollCL {
  id: string;
  result: AutoRollCL_Result;
  subject: string;
  rollingTo: string;
  rollingFrom: string;
  created?: string;
  modified?: string;
  tryJobs?: TryJob[];
}

interface AutoRollCLJSON {
  id?: string;
  result?: string;
  subject?: string;
  rolling_to?: string;
  rolling_from?: string;
  created?: string;
  modified?: string;
  try_jobs?: TryJobJSON[];
}

const JSONToAutoRollCL = (m: AutoRollCLJSON): AutoRollCL => {
  return {
    id: m.id || "",
    result: (m.result || Object.keys(AutoRollCL_Result)[0]) as AutoRollCL_Result,
    subject: m.subject || "",
    rollingTo: m.rolling_to || "",
    rollingFrom: m.rolling_from || "",
    created: m.created,
    modified: m.modified,
    tryJobs: m.try_jobs && m.try_jobs.map(JSONToTryJob),
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
  childBugLink: string;
  parentBugLink: string;
  parentWaterfall: string;
  rollerId: string;
  supportsManualRolls: boolean;
  timeWindow: string;
}

interface AutoRollConfigJSON {
  child_bug_link?: string;
  parent_bug_link?: string;
  parent_waterfall?: string;
  roller_id?: string;
  supports_manual_rolls?: boolean;
  time_window?: string;
}

const JSONToAutoRollConfig = (m: AutoRollConfigJSON): AutoRollConfig => {
  return {
    childBugLink: m.child_bug_link || "",
    parentBugLink: m.parent_bug_link || "",
    parentWaterfall: m.parent_waterfall || "",
    rollerId: m.roller_id || "",
    supportsManualRolls: m.supports_manual_rolls || false,
    timeWindow: m.time_window || "",
  };
};

export interface ModeChange {
  rollerId: string;
  mode: Mode;
  user: string;
  time?: string;
  message: string;
}

interface ModeChangeJSON {
  roller_id?: string;
  mode?: string;
  user?: string;
  time?: string;
  message?: string;
}

const JSONToModeChange = (m: ModeChangeJSON): ModeChange => {
  return {
    rollerId: m.roller_id || "",
    mode: (m.mode || Object.keys(Mode)[0]) as Mode,
    user: m.user || "",
    time: m.time,
    message: m.message || "",
  };
};

export interface StrategyChange {
  rollerId: string;
  strategy: Strategy;
  user: string;
  time?: string;
  message: string;
}

interface StrategyChangeJSON {
  roller_id?: string;
  strategy?: string;
  user?: string;
  time?: string;
  message?: string;
}

const JSONToStrategyChange = (m: StrategyChangeJSON): StrategyChange => {
  return {
    rollerId: m.roller_id || "",
    strategy: (m.strategy || Object.keys(Strategy)[0]) as Strategy,
    user: m.user || "",
    time: m.time,
    message: m.message || "",
  };
};

export interface ManualRoll {
  id: string;
  rollerId: string;
  revision: string;
  requester: string;
  result: ManualRoll_Result;
  status: ManualRoll_Status;
  timestamp?: string;
  url: string;
  dryRun: boolean;
  noEmail: boolean;
  noResolveRevision: boolean;
}

interface ManualRollJSON {
  id?: string;
  roller_id?: string;
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

const JSONToManualRoll = (m: ManualRollJSON): ManualRoll => {
  return {
    id: m.id || "",
    rollerId: m.roller_id || "",
    revision: m.revision || "",
    requester: m.requester || "",
    result: (m.result || Object.keys(ManualRoll_Result)[0]) as ManualRoll_Result,
    status: (m.status || Object.keys(ManualRoll_Status)[0]) as ManualRoll_Status,
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
  fullHistoryUrl: string;
  issueUrlBase: string;
  mode?: ModeChange;
  strategy?: StrategyChange;
  notRolledRevisions?: Revision[];
  currentRoll?: AutoRollCL;
  lastRoll?: AutoRollCL;
  recentRolls?: AutoRollCL[];
  manualRolls?: ManualRoll[];
  error: string;
  throttledUntil?: string;
}

interface AutoRollStatusJSON {
  mini_status?: AutoRollMiniStatusJSON;
  status?: string;
  config?: AutoRollConfigJSON;
  full_history_url?: string;
  issue_url_base?: string;
  mode?: ModeChangeJSON;
  strategy?: StrategyChangeJSON;
  not_rolled_revisions?: RevisionJSON[];
  current_roll?: AutoRollCLJSON;
  last_roll?: AutoRollCLJSON;
  recent_rolls?: AutoRollCLJSON[];
  manual_rolls?: ManualRollJSON[];
  error?: string;
  throttled_until?: string;
}

const JSONToAutoRollStatus = (m: AutoRollStatusJSON): AutoRollStatus => {
  return {
    miniStatus: m.mini_status && JSONToAutoRollMiniStatus(m.mini_status),
    status: m.status || "",
    config: m.config && JSONToAutoRollConfig(m.config),
    fullHistoryUrl: m.full_history_url || "",
    issueUrlBase: m.issue_url_base || "",
    mode: m.mode && JSONToModeChange(m.mode),
    strategy: m.strategy && JSONToStrategyChange(m.strategy),
    notRolledRevisions: m.not_rolled_revisions && m.not_rolled_revisions.map(JSONToRevision),
    currentRoll: m.current_roll && JSONToAutoRollCL(m.current_roll),
    lastRoll: m.last_roll && JSONToAutoRollCL(m.last_roll),
    recentRolls: m.recent_rolls && m.recent_rolls.map(JSONToAutoRollCL),
    manualRolls: m.manual_rolls && m.manual_rolls.map(JSONToManualRoll),
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
  rollerId: string;
}

interface GetMiniStatusRequestJSON {
  roller_id?: string;
}

const GetMiniStatusRequestToJSON = (m: GetMiniStatusRequest): GetMiniStatusRequestJSON => {
  return {
    roller_id: m.rollerId,
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
  rollerId: string;
}

interface GetStatusRequestJSON {
  roller_id?: string;
}

const GetStatusRequestToJSON = (m: GetStatusRequest): GetStatusRequestJSON => {
  return {
    roller_id: m.rollerId,
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
  rollerId: string;
  mode: Mode;
  message: string;
}

interface SetModeRequestJSON {
  roller_id?: string;
  mode?: string;
  message?: string;
}

const SetModeRequestToJSON = (m: SetModeRequest): SetModeRequestJSON => {
  return {
    roller_id: m.rollerId,
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
  rollerId: string;
  strategy: Strategy;
  message: string;
}

interface SetStrategyRequestJSON {
  roller_id?: string;
  strategy?: string;
  message?: string;
}

const SetStrategyRequestToJSON = (m: SetStrategyRequest): SetStrategyRequestJSON => {
  return {
    roller_id: m.rollerId,
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
  rollerId: string;
  revision: string;
}

interface CreateManualRollRequestJSON {
  roller_id?: string;
  revision?: string;
}

const CreateManualRollRequestToJSON = (m: CreateManualRollRequest): CreateManualRollRequestJSON => {
  return {
    roller_id: m.rollerId,
    revision: m.revision,
  };
};

export interface CreateManualRollResponse {
  roll?: ManualRoll;
}

interface CreateManualRollResponseJSON {
  roll?: ManualRollJSON;
}

const JSONToCreateManualRollResponse = (m: CreateManualRollResponseJSON): CreateManualRollResponse => {
  return {
    roll: m.roll && JSONToManualRoll(m.roll),
  };
};

export interface UnthrottleRequest {
  rollerId: string;
}

interface UnthrottleRequestJSON {
  roller_id?: string;
}

const UnthrottleRequestToJSON = (m: UnthrottleRequest): UnthrottleRequestJSON => {
  return {
    roller_id: m.rollerId,
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
