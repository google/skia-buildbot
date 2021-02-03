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

export enum PreUploadStep {
  ANGLE_CODE_GENERATION = "ANGLE_CODE_GENERATION",
  ANGLE_GN_TO_BP = "ANGLE_GN_TO_BP",
  ANGLE_ROLL_CHROMIUM = "ANGLE_ROLL_CHROMIUM",
  GO_GENERATE_CIPD = "GO_GENERATE_CIPD",
  FLUTTER_LICENSE_SCRIPTS = "FLUTTER_LICENSE_SCRIPTS",
  FLUTTER_LICENSE_SCRIPTS_FOR_DART = "FLUTTER_LICENSE_SCRIPTS_FOR_DART",
  FLUTTER_LICENSE_SCRIPTS_FOR_FUCHSIA = "FLUTTER_LICENSE_SCRIPTS_FOR_FUCHSIA",
  SKIA_GN_TO_BP = "SKIA_GN_TO_BP",
  TRAIN_INFRA = "TRAIN_INFRA",
  UPDATE_FLUTTER_DEPS_FOR_DART = "UPDATE_FLUTTER_DEPS_FOR_DART",
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

export enum CommitMsgConfig_BuiltIn {
  DEFAULT = "DEFAULT",
  ANDROID = "ANDROID",
}

export enum GerritConfig_Config {
  ANDROID = "ANDROID",
  ANGLE = "ANGLE",
  CHROMIUM = "CHROMIUM",
  CHROMIUM_NO_CQ = "CHROMIUM_NO_CQ",
  LIBASSISTANT = "LIBASSISTANT",
}

export enum NotifierConfig_LogLevel {
  SILENT = "SILENT",
  ERROR = "ERROR",
  WARNING = "WARNING",
  INFO = "INFO",
  DEBUG = "DEBUG",
}

export enum NotifierConfig_MsgType {
  ISSUE_UPDATE = "ISSUE_UPDATE",
  LAST_N_FAILED = "LAST_N_FAILED",
  MODE_CHANGE = "MODE_CHANGE",
  NEW_FAILURE = "NEW_FAILURE",
  NEW_SUCCESS = "NEW_SUCCESS",
  ROLL_CREATION_FAILED = "ROLL_CREATION_FAILED",
  SAFETY_THROTTLE = "SAFETY_THROTTLE",
  STRATEGY_CHANGE = "STRATEGY_CHANGE",
  SUCCESS_THROTTLE = "SUCCESS_THROTTLE",
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
  parentWaterfall: string;
  rollerId: string;
  supportsManualRolls: boolean;
  timeWindow: string;
}

interface AutoRollConfigJSON {
  parent_waterfall?: string;
  roller_id?: string;
  supports_manual_rolls?: boolean;
  time_window?: string;
}

const JSONToAutoRollConfig = (m: AutoRollConfigJSON): AutoRollConfig => {
  return {
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

export interface GetConfigRequest {
  rollerId: string;
}

interface GetConfigRequestJSON {
  roller_id?: string;
}

const GetConfigRequestToJSON = (m: GetConfigRequest): GetConfigRequestJSON => {
  return {
    roller_id: m.rollerId,
  };
};

export interface GetConfigResponse {
  config?: Config;
}

interface GetConfigResponseJSON {
  config?: ConfigJSON;
}

const JSONToGetConfigResponse = (m: GetConfigResponseJSON): GetConfigResponse => {
  return {
    config: m.config && JSONToConfig(m.config),
  };
};

export interface PutConfigRequest {
  config?: Config;
}

interface PutConfigRequestJSON {
  config?: ConfigJSON;
}

const PutConfigRequestToJSON = (m: PutConfigRequest): PutConfigRequestJSON => {
  return {
    config: m.config && ConfigToJSON(m.config),
  };
};

export interface PutConfigResponse {
  cl: string;
}

interface PutConfigResponseJSON {
  cl?: string;
}

const JSONToPutConfigResponse = (m: PutConfigResponseJSON): PutConfigResponse => {
  return {
    cl: m.cl || "",
  };
};

export interface Config {
  rollerName: string;
  childDisplayName: string;
  parentDisplayName: string;
  parentWaterfall: string;
  ownerPrimary: string;
  ownerSecondary: string;
  contacts?: string[];
  serviceAccount: string;
  isInternal: boolean;
  reviewer?: string[];
  reviewerBackup?: string[];
  rollCooldown: string;
  timeWindow: string;
  supportsManualRolls: boolean;
  commitMsg?: CommitMsgConfig;
  gerrit?: GerritConfig;
  github?: GitHubConfig;
  google3?: Google3Config;
  kubernetes?: KubernetesConfig;
  parentChildRepoManager?: ParentChildRepoManagerConfig;
  androidRepoManager?: AndroidRepoManagerConfig;
  commandRepoManager?: CommandRepoManagerConfig;
  freetypeRepoManager?: FreeTypeRepoManagerConfig;
  fuchsiaSdkAndroidRepoManager?: FuchsiaSDKAndroidRepoManagerConfig;
  google3RepoManager?: Google3RepoManagerConfig;
  notifiers?: NotifierConfig[];
  safetyThrottle?: ThrottleConfig;
  transitiveDeps?: TransitiveDepConfig[];
}

interface ConfigJSON {
  roller_name?: string;
  child_display_name?: string;
  parent_display_name?: string;
  parent_waterfall?: string;
  owner_primary?: string;
  owner_secondary?: string;
  contacts?: string[];
  service_account?: string;
  is_internal?: boolean;
  reviewer?: string[];
  reviewer_backup?: string[];
  roll_cooldown?: string;
  time_window?: string;
  supports_manual_rolls?: boolean;
  commit_msg?: CommitMsgConfigJSON;
  gerrit?: GerritConfigJSON;
  github?: GitHubConfigJSON;
  google3?: Google3ConfigJSON;
  kubernetes?: KubernetesConfigJSON;
  parent_child_repo_manager?: ParentChildRepoManagerConfigJSON;
  android_repo_manager?: AndroidRepoManagerConfigJSON;
  command_repo_manager?: CommandRepoManagerConfigJSON;
  freetype_repo_manager?: FreeTypeRepoManagerConfigJSON;
  fuchsia_sdk_android_repo_manager?: FuchsiaSDKAndroidRepoManagerConfigJSON;
  google3_repo_manager?: Google3RepoManagerConfigJSON;
  notifiers?: NotifierConfigJSON[];
  safety_throttle?: ThrottleConfigJSON;
  transitive_deps?: TransitiveDepConfigJSON[];
}

const ConfigToJSON = (m: Config): ConfigJSON => {
  return {
    roller_name: m.rollerName,
    child_display_name: m.childDisplayName,
    parent_display_name: m.parentDisplayName,
    parent_waterfall: m.parentWaterfall,
    owner_primary: m.ownerPrimary,
    owner_secondary: m.ownerSecondary,
    contacts: m.contacts,
    service_account: m.serviceAccount,
    is_internal: m.isInternal,
    reviewer: m.reviewer,
    reviewer_backup: m.reviewerBackup,
    roll_cooldown: m.rollCooldown,
    time_window: m.timeWindow,
    supports_manual_rolls: m.supportsManualRolls,
    commit_msg: m.commitMsg && CommitMsgConfigToJSON(m.commitMsg),
    gerrit: m.gerrit && GerritConfigToJSON(m.gerrit),
    github: m.github && GitHubConfigToJSON(m.github),
    google3: m.google3 && Google3ConfigToJSON(m.google3),
    kubernetes: m.kubernetes && KubernetesConfigToJSON(m.kubernetes),
    parent_child_repo_manager: m.parentChildRepoManager && ParentChildRepoManagerConfigToJSON(m.parentChildRepoManager),
    android_repo_manager: m.androidRepoManager && AndroidRepoManagerConfigToJSON(m.androidRepoManager),
    command_repo_manager: m.commandRepoManager && CommandRepoManagerConfigToJSON(m.commandRepoManager),
    freetype_repo_manager: m.freetypeRepoManager && FreeTypeRepoManagerConfigToJSON(m.freetypeRepoManager),
    fuchsia_sdk_android_repo_manager: m.fuchsiaSdkAndroidRepoManager && FuchsiaSDKAndroidRepoManagerConfigToJSON(m.fuchsiaSdkAndroidRepoManager),
    google3_repo_manager: m.google3RepoManager && Google3RepoManagerConfigToJSON(m.google3RepoManager),
    notifiers: m.notifiers && m.notifiers.map(NotifierConfigToJSON),
    safety_throttle: m.safetyThrottle && ThrottleConfigToJSON(m.safetyThrottle),
    transitive_deps: m.transitiveDeps && m.transitiveDeps.map(TransitiveDepConfigToJSON),
  };
};

const JSONToConfig = (m: ConfigJSON): Config => {
  return {
    rollerName: m.roller_name || "",
    childDisplayName: m.child_display_name || "",
    parentDisplayName: m.parent_display_name || "",
    parentWaterfall: m.parent_waterfall || "",
    ownerPrimary: m.owner_primary || "",
    ownerSecondary: m.owner_secondary || "",
    contacts: m.contacts,
    serviceAccount: m.service_account || "",
    isInternal: m.is_internal || false,
    reviewer: m.reviewer,
    reviewerBackup: m.reviewer_backup,
    rollCooldown: m.roll_cooldown || "",
    timeWindow: m.time_window || "",
    supportsManualRolls: m.supports_manual_rolls || false,
    commitMsg: m.commit_msg && JSONToCommitMsgConfig(m.commit_msg),
    gerrit: m.gerrit && JSONToGerritConfig(m.gerrit),
    github: m.github && JSONToGitHubConfig(m.github),
    google3: m.google3 && JSONToGoogle3Config(m.google3),
    kubernetes: m.kubernetes && JSONToKubernetesConfig(m.kubernetes),
    parentChildRepoManager: m.parent_child_repo_manager && JSONToParentChildRepoManagerConfig(m.parent_child_repo_manager),
    androidRepoManager: m.android_repo_manager && JSONToAndroidRepoManagerConfig(m.android_repo_manager),
    commandRepoManager: m.command_repo_manager && JSONToCommandRepoManagerConfig(m.command_repo_manager),
    freetypeRepoManager: m.freetype_repo_manager && JSONToFreeTypeRepoManagerConfig(m.freetype_repo_manager),
    fuchsiaSdkAndroidRepoManager: m.fuchsia_sdk_android_repo_manager && JSONToFuchsiaSDKAndroidRepoManagerConfig(m.fuchsia_sdk_android_repo_manager),
    google3RepoManager: m.google3_repo_manager && JSONToGoogle3RepoManagerConfig(m.google3_repo_manager),
    notifiers: m.notifiers && m.notifiers.map(JSONToNotifierConfig),
    safetyThrottle: m.safety_throttle && JSONToThrottleConfig(m.safety_throttle),
    transitiveDeps: m.transitive_deps && m.transitive_deps.map(JSONToTransitiveDepConfig),
  };
};

export interface CommitMsgConfig {
  bugProject: string;
  childLogUrlTmpl: string;
  cqExtraTrybots?: string[];
  cqDoNotCancelTrybots: boolean;
  includeLog: boolean;
  includeRevisionCount: boolean;
  includeTbrLine: boolean;
  includeTests: boolean;
  builtIn: CommitMsgConfig_BuiltIn;
  custom: string;
}

interface CommitMsgConfigJSON {
  bug_project?: string;
  child_log_url_tmpl?: string;
  cq_extra_trybots?: string[];
  cq_do_not_cancel_trybots?: boolean;
  include_log?: boolean;
  include_revision_count?: boolean;
  include_tbr_line?: boolean;
  include_tests?: boolean;
  built_in?: string;
  custom?: string;
}

const CommitMsgConfigToJSON = (m: CommitMsgConfig): CommitMsgConfigJSON => {
  return {
    bug_project: m.bugProject,
    child_log_url_tmpl: m.childLogUrlTmpl,
    cq_extra_trybots: m.cqExtraTrybots,
    cq_do_not_cancel_trybots: m.cqDoNotCancelTrybots,
    include_log: m.includeLog,
    include_revision_count: m.includeRevisionCount,
    include_tbr_line: m.includeTbrLine,
    include_tests: m.includeTests,
    built_in: m.builtIn,
    custom: m.custom,
  };
};

const JSONToCommitMsgConfig = (m: CommitMsgConfigJSON): CommitMsgConfig => {
  return {
    bugProject: m.bug_project || "",
    childLogUrlTmpl: m.child_log_url_tmpl || "",
    cqExtraTrybots: m.cq_extra_trybots,
    cqDoNotCancelTrybots: m.cq_do_not_cancel_trybots || false,
    includeLog: m.include_log || false,
    includeRevisionCount: m.include_revision_count || false,
    includeTbrLine: m.include_tbr_line || false,
    includeTests: m.include_tests || false,
    builtIn: (m.built_in || Object.keys(CommitMsgConfig_BuiltIn)[0]) as CommitMsgConfig_BuiltIn,
    custom: m.custom || "",
  };
};

export interface GerritConfig {
  url: string;
  project: string;
  config: GerritConfig_Config;
}

interface GerritConfigJSON {
  url?: string;
  project?: string;
  config?: string;
}

const GerritConfigToJSON = (m: GerritConfig): GerritConfigJSON => {
  return {
    url: m.url,
    project: m.project,
    config: m.config,
  };
};

const JSONToGerritConfig = (m: GerritConfigJSON): GerritConfig => {
  return {
    url: m.url || "",
    project: m.project || "",
    config: (m.config || Object.keys(GerritConfig_Config)[0]) as GerritConfig_Config,
  };
};

export interface GitHubConfig {
  repoOwner: string;
  repoName: string;
  checksWaitFor?: string[];
}

interface GitHubConfigJSON {
  repo_owner?: string;
  repo_name?: string;
  checks_wait_for?: string[];
}

const GitHubConfigToJSON = (m: GitHubConfig): GitHubConfigJSON => {
  return {
    repo_owner: m.repoOwner,
    repo_name: m.repoName,
    checks_wait_for: m.checksWaitFor,
  };
};

const JSONToGitHubConfig = (m: GitHubConfigJSON): GitHubConfig => {
  return {
    repoOwner: m.repo_owner || "",
    repoName: m.repo_name || "",
    checksWaitFor: m.checks_wait_for,
  };
};

export interface Google3Config {
}

interface Google3ConfigJSON {
}

const Google3ConfigToJSON = (m: Google3Config): Google3ConfigJSON => {
  return {
  };
};

const JSONToGoogle3Config = (m: Google3ConfigJSON): Google3Config => {
  return {
  };
};

export interface KubernetesConfig {
  cpu: string;
  memory: string;
  readinessFailureThreshold: number;
  readinessInitialDelaySeconds: number;
  readinessPeriodSeconds: number;
  disk: string;
  secrets?: KubernetesSecret[];
}

interface KubernetesConfigJSON {
  cpu?: string;
  memory?: string;
  readiness_failure_threshold?: number;
  readiness_initial_delay_seconds?: number;
  readiness_period_seconds?: number;
  disk?: string;
  secrets?: KubernetesSecretJSON[];
}

const KubernetesConfigToJSON = (m: KubernetesConfig): KubernetesConfigJSON => {
  return {
    cpu: m.cpu,
    memory: m.memory,
    readiness_failure_threshold: m.readinessFailureThreshold,
    readiness_initial_delay_seconds: m.readinessInitialDelaySeconds,
    readiness_period_seconds: m.readinessPeriodSeconds,
    disk: m.disk,
    secrets: m.secrets && m.secrets.map(KubernetesSecretToJSON),
  };
};

const JSONToKubernetesConfig = (m: KubernetesConfigJSON): KubernetesConfig => {
  return {
    cpu: m.cpu || "",
    memory: m.memory || "",
    readinessFailureThreshold: m.readiness_failure_threshold || 0,
    readinessInitialDelaySeconds: m.readiness_initial_delay_seconds || 0,
    readinessPeriodSeconds: m.readiness_period_seconds || 0,
    disk: m.disk || "",
    secrets: m.secrets && m.secrets.map(JSONToKubernetesSecret),
  };
};

export interface KubernetesSecret {
  name: string;
  mountPath: string;
}

interface KubernetesSecretJSON {
  name?: string;
  mount_path?: string;
}

const KubernetesSecretToJSON = (m: KubernetesSecret): KubernetesSecretJSON => {
  return {
    name: m.name,
    mount_path: m.mountPath,
  };
};

const JSONToKubernetesSecret = (m: KubernetesSecretJSON): KubernetesSecret => {
  return {
    name: m.name || "",
    mountPath: m.mount_path || "",
  };
};

export interface AndroidRepoManagerConfig_ProjectMetadataFileConfig {
  filePath: string;
  name: string;
  description: string;
  homePage: string;
  gitUrl: string;
  licenseType: string;
}

interface AndroidRepoManagerConfig_ProjectMetadataFileConfigJSON {
  file_path?: string;
  name?: string;
  description?: string;
  home_page?: string;
  git_url?: string;
  license_type?: string;
}

const AndroidRepoManagerConfig_ProjectMetadataFileConfigToJSON = (m: AndroidRepoManagerConfig_ProjectMetadataFileConfig): AndroidRepoManagerConfig_ProjectMetadataFileConfigJSON => {
  return {
    file_path: m.filePath,
    name: m.name,
    description: m.description,
    home_page: m.homePage,
    git_url: m.gitUrl,
    license_type: m.licenseType,
  };
};

const JSONToAndroidRepoManagerConfig_ProjectMetadataFileConfig = (m: AndroidRepoManagerConfig_ProjectMetadataFileConfigJSON): AndroidRepoManagerConfig_ProjectMetadataFileConfig => {
  return {
    filePath: m.file_path || "",
    name: m.name || "",
    description: m.description || "",
    homePage: m.home_page || "",
    gitUrl: m.git_url || "",
    licenseType: m.license_type || "",
  };
};

export interface AndroidRepoManagerConfig {
  childRepoUrl: string;
  childBranch: string;
  childPath: string;
  parentRepoUrl: string;
  parentBranch: string;
  childRevLinkTmpl: string;
  childSubdir: string;
  preUploadSteps?: PreUploadStep[];
  metadata?: AndroidRepoManagerConfig_ProjectMetadataFileConfig;
}

interface AndroidRepoManagerConfigJSON {
  child_repo_url?: string;
  child_branch?: string;
  child_path?: string;
  parent_repo_url?: string;
  parent_branch?: string;
  child_rev_link_tmpl?: string;
  child_subdir?: string;
  pre_upload_steps?: string[];
  metadata?: AndroidRepoManagerConfig_ProjectMetadataFileConfigJSON;
}

const AndroidRepoManagerConfigToJSON = (m: AndroidRepoManagerConfig): AndroidRepoManagerConfigJSON => {
  return {
    child_repo_url: m.childRepoUrl,
    child_branch: m.childBranch,
    child_path: m.childPath,
    parent_repo_url: m.parentRepoUrl,
    parent_branch: m.parentBranch,
    child_rev_link_tmpl: m.childRevLinkTmpl,
    child_subdir: m.childSubdir,
    pre_upload_steps: m.preUploadSteps,
    metadata: m.metadata && AndroidRepoManagerConfig_ProjectMetadataFileConfigToJSON(m.metadata),
  };
};

const JSONToAndroidRepoManagerConfig = (m: AndroidRepoManagerConfigJSON): AndroidRepoManagerConfig => {
  return {
    childRepoUrl: m.child_repo_url || "",
    childBranch: m.child_branch || "",
    childPath: m.child_path || "",
    parentRepoUrl: m.parent_repo_url || "",
    parentBranch: m.parent_branch || "",
    childRevLinkTmpl: m.child_rev_link_tmpl || "",
    childSubdir: m.child_subdir || "",
    preUploadSteps: (m.pre_upload_steps || []) as PreUploadStep[],
    metadata: m.metadata && JSONToAndroidRepoManagerConfig_ProjectMetadataFileConfig(m.metadata),
  };
};

export interface CommandRepoManagerConfig_CommandConfig {
  command?: string[];
  dir: string;
  env?: string[];
}

interface CommandRepoManagerConfig_CommandConfigJSON {
  command?: string[];
  dir?: string;
  env?: string[];
}

const CommandRepoManagerConfig_CommandConfigToJSON = (m: CommandRepoManagerConfig_CommandConfig): CommandRepoManagerConfig_CommandConfigJSON => {
  return {
    command: m.command,
    dir: m.dir,
    env: m.env,
  };
};

const JSONToCommandRepoManagerConfig_CommandConfig = (m: CommandRepoManagerConfig_CommandConfigJSON): CommandRepoManagerConfig_CommandConfig => {
  return {
    command: m.command,
    dir: m.dir || "",
    env: m.env,
  };
};

export interface CommandRepoManagerConfig {
  gitCheckout?: GitCheckoutConfig;
  shortRevRegex: string;
  getTipRev?: CommandRepoManagerConfig_CommandConfig;
  getPinnedRev?: CommandRepoManagerConfig_CommandConfig;
  setPinnedRev?: CommandRepoManagerConfig_CommandConfig;
}

interface CommandRepoManagerConfigJSON {
  git_checkout?: GitCheckoutConfigJSON;
  short_rev_regex?: string;
  get_tip_rev?: CommandRepoManagerConfig_CommandConfigJSON;
  get_pinned_rev?: CommandRepoManagerConfig_CommandConfigJSON;
  set_pinned_rev?: CommandRepoManagerConfig_CommandConfigJSON;
}

const CommandRepoManagerConfigToJSON = (m: CommandRepoManagerConfig): CommandRepoManagerConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutConfigToJSON(m.gitCheckout),
    short_rev_regex: m.shortRevRegex,
    get_tip_rev: m.getTipRev && CommandRepoManagerConfig_CommandConfigToJSON(m.getTipRev),
    get_pinned_rev: m.getPinnedRev && CommandRepoManagerConfig_CommandConfigToJSON(m.getPinnedRev),
    set_pinned_rev: m.setPinnedRev && CommandRepoManagerConfig_CommandConfigToJSON(m.setPinnedRev),
  };
};

const JSONToCommandRepoManagerConfig = (m: CommandRepoManagerConfigJSON): CommandRepoManagerConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutConfig(m.git_checkout),
    shortRevRegex: m.short_rev_regex || "",
    getTipRev: m.get_tip_rev && JSONToCommandRepoManagerConfig_CommandConfig(m.get_tip_rev),
    getPinnedRev: m.get_pinned_rev && JSONToCommandRepoManagerConfig_CommandConfig(m.get_pinned_rev),
    setPinnedRev: m.set_pinned_rev && JSONToCommandRepoManagerConfig_CommandConfig(m.set_pinned_rev),
  };
};

export interface FreeTypeRepoManagerConfig {
  parent?: FreeTypeParentConfig;
  child?: GitilesChildConfig;
}

interface FreeTypeRepoManagerConfigJSON {
  parent?: FreeTypeParentConfigJSON;
  child?: GitilesChildConfigJSON;
}

const FreeTypeRepoManagerConfigToJSON = (m: FreeTypeRepoManagerConfig): FreeTypeRepoManagerConfigJSON => {
  return {
    parent: m.parent && FreeTypeParentConfigToJSON(m.parent),
    child: m.child && GitilesChildConfigToJSON(m.child),
  };
};

const JSONToFreeTypeRepoManagerConfig = (m: FreeTypeRepoManagerConfigJSON): FreeTypeRepoManagerConfig => {
  return {
    parent: m.parent && JSONToFreeTypeParentConfig(m.parent),
    child: m.child && JSONToGitilesChildConfig(m.child),
  };
};

export interface FuchsiaSDKAndroidRepoManagerConfig {
  parent?: GitCheckoutParentConfig;
  child?: FuchsiaSDKChildConfig;
  genSdkBpRepo: string;
  genSdkBpBranch: string;
}

interface FuchsiaSDKAndroidRepoManagerConfigJSON {
  parent?: GitCheckoutParentConfigJSON;
  child?: FuchsiaSDKChildConfigJSON;
  gen_sdk_bp_repo?: string;
  gen_sdk_bp_branch?: string;
}

const FuchsiaSDKAndroidRepoManagerConfigToJSON = (m: FuchsiaSDKAndroidRepoManagerConfig): FuchsiaSDKAndroidRepoManagerConfigJSON => {
  return {
    parent: m.parent && GitCheckoutParentConfigToJSON(m.parent),
    child: m.child && FuchsiaSDKChildConfigToJSON(m.child),
    gen_sdk_bp_repo: m.genSdkBpRepo,
    gen_sdk_bp_branch: m.genSdkBpBranch,
  };
};

const JSONToFuchsiaSDKAndroidRepoManagerConfig = (m: FuchsiaSDKAndroidRepoManagerConfigJSON): FuchsiaSDKAndroidRepoManagerConfig => {
  return {
    parent: m.parent && JSONToGitCheckoutParentConfig(m.parent),
    child: m.child && JSONToFuchsiaSDKChildConfig(m.child),
    genSdkBpRepo: m.gen_sdk_bp_repo || "",
    genSdkBpBranch: m.gen_sdk_bp_branch || "",
  };
};

export interface Google3RepoManagerConfig {
  childBranch: string;
  childRepo: string;
}

interface Google3RepoManagerConfigJSON {
  child_branch?: string;
  child_repo?: string;
}

const Google3RepoManagerConfigToJSON = (m: Google3RepoManagerConfig): Google3RepoManagerConfigJSON => {
  return {
    child_branch: m.childBranch,
    child_repo: m.childRepo,
  };
};

const JSONToGoogle3RepoManagerConfig = (m: Google3RepoManagerConfigJSON): Google3RepoManagerConfig => {
  return {
    childBranch: m.child_branch || "",
    childRepo: m.child_repo || "",
  };
};

export interface ParentChildRepoManagerConfig {
  copyParent?: CopyParentConfig;
  depsLocalGithubParent?: DEPSLocalGitHubParentConfig;
  depsLocalGerritParent?: DEPSLocalGerritParentConfig;
  gitCheckoutGithubFileParent?: GitCheckoutGitHubFileParentConfig;
  gitilesParent?: GitilesParentConfig;
  cipdChild?: CIPDChildConfig;
  fuchsiaSdkChild?: FuchsiaSDKChildConfig;
  gitCheckoutChild?: GitCheckoutChildConfig;
  gitCheckoutGithubChild?: GitCheckoutGitHubChildConfig;
  gitilesChild?: GitilesChildConfig;
  semverGcsChild?: SemVerGCSChildConfig;
  buildbucketRevisionFilter?: BuildbucketRevisionFilterConfig;
}

interface ParentChildRepoManagerConfigJSON {
  copy_parent?: CopyParentConfigJSON;
  deps_local_github_parent?: DEPSLocalGitHubParentConfigJSON;
  deps_local_gerrit_parent?: DEPSLocalGerritParentConfigJSON;
  git_checkout_github_file_parent?: GitCheckoutGitHubFileParentConfigJSON;
  gitiles_parent?: GitilesParentConfigJSON;
  cipd_child?: CIPDChildConfigJSON;
  fuchsia_sdk_child?: FuchsiaSDKChildConfigJSON;
  git_checkout_child?: GitCheckoutChildConfigJSON;
  git_checkout_github_child?: GitCheckoutGitHubChildConfigJSON;
  gitiles_child?: GitilesChildConfigJSON;
  semver_gcs_child?: SemVerGCSChildConfigJSON;
  buildbucket_revision_filter?: BuildbucketRevisionFilterConfigJSON;
}

const ParentChildRepoManagerConfigToJSON = (m: ParentChildRepoManagerConfig): ParentChildRepoManagerConfigJSON => {
  return {
    copy_parent: m.copyParent && CopyParentConfigToJSON(m.copyParent),
    deps_local_github_parent: m.depsLocalGithubParent && DEPSLocalGitHubParentConfigToJSON(m.depsLocalGithubParent),
    deps_local_gerrit_parent: m.depsLocalGerritParent && DEPSLocalGerritParentConfigToJSON(m.depsLocalGerritParent),
    git_checkout_github_file_parent: m.gitCheckoutGithubFileParent && GitCheckoutGitHubFileParentConfigToJSON(m.gitCheckoutGithubFileParent),
    gitiles_parent: m.gitilesParent && GitilesParentConfigToJSON(m.gitilesParent),
    cipd_child: m.cipdChild && CIPDChildConfigToJSON(m.cipdChild),
    fuchsia_sdk_child: m.fuchsiaSdkChild && FuchsiaSDKChildConfigToJSON(m.fuchsiaSdkChild),
    git_checkout_child: m.gitCheckoutChild && GitCheckoutChildConfigToJSON(m.gitCheckoutChild),
    git_checkout_github_child: m.gitCheckoutGithubChild && GitCheckoutGitHubChildConfigToJSON(m.gitCheckoutGithubChild),
    gitiles_child: m.gitilesChild && GitilesChildConfigToJSON(m.gitilesChild),
    semver_gcs_child: m.semverGcsChild && SemVerGCSChildConfigToJSON(m.semverGcsChild),
    buildbucket_revision_filter: m.buildbucketRevisionFilter && BuildbucketRevisionFilterConfigToJSON(m.buildbucketRevisionFilter),
  };
};

const JSONToParentChildRepoManagerConfig = (m: ParentChildRepoManagerConfigJSON): ParentChildRepoManagerConfig => {
  return {
    copyParent: m.copy_parent && JSONToCopyParentConfig(m.copy_parent),
    depsLocalGithubParent: m.deps_local_github_parent && JSONToDEPSLocalGitHubParentConfig(m.deps_local_github_parent),
    depsLocalGerritParent: m.deps_local_gerrit_parent && JSONToDEPSLocalGerritParentConfig(m.deps_local_gerrit_parent),
    gitCheckoutGithubFileParent: m.git_checkout_github_file_parent && JSONToGitCheckoutGitHubFileParentConfig(m.git_checkout_github_file_parent),
    gitilesParent: m.gitiles_parent && JSONToGitilesParentConfig(m.gitiles_parent),
    cipdChild: m.cipd_child && JSONToCIPDChildConfig(m.cipd_child),
    fuchsiaSdkChild: m.fuchsia_sdk_child && JSONToFuchsiaSDKChildConfig(m.fuchsia_sdk_child),
    gitCheckoutChild: m.git_checkout_child && JSONToGitCheckoutChildConfig(m.git_checkout_child),
    gitCheckoutGithubChild: m.git_checkout_github_child && JSONToGitCheckoutGitHubChildConfig(m.git_checkout_github_child),
    gitilesChild: m.gitiles_child && JSONToGitilesChildConfig(m.gitiles_child),
    semverGcsChild: m.semver_gcs_child && JSONToSemVerGCSChildConfig(m.semver_gcs_child),
    buildbucketRevisionFilter: m.buildbucket_revision_filter && JSONToBuildbucketRevisionFilterConfig(m.buildbucket_revision_filter),
  };
};

export interface CopyParentConfig_CopyEntry {
  srcRelPath: string;
  dstRelPath: string;
}

interface CopyParentConfig_CopyEntryJSON {
  src_rel_path?: string;
  dst_rel_path?: string;
}

const CopyParentConfig_CopyEntryToJSON = (m: CopyParentConfig_CopyEntry): CopyParentConfig_CopyEntryJSON => {
  return {
    src_rel_path: m.srcRelPath,
    dst_rel_path: m.dstRelPath,
  };
};

const JSONToCopyParentConfig_CopyEntry = (m: CopyParentConfig_CopyEntryJSON): CopyParentConfig_CopyEntry => {
  return {
    srcRelPath: m.src_rel_path || "",
    dstRelPath: m.dst_rel_path || "",
  };
};

export interface CopyParentConfig {
  gitiles?: GitilesParentConfig;
  copies?: CopyParentConfig_CopyEntry[];
}

interface CopyParentConfigJSON {
  gitiles?: GitilesParentConfigJSON;
  copies?: CopyParentConfig_CopyEntryJSON[];
}

const CopyParentConfigToJSON = (m: CopyParentConfig): CopyParentConfigJSON => {
  return {
    gitiles: m.gitiles && GitilesParentConfigToJSON(m.gitiles),
    copies: m.copies && m.copies.map(CopyParentConfig_CopyEntryToJSON),
  };
};

const JSONToCopyParentConfig = (m: CopyParentConfigJSON): CopyParentConfig => {
  return {
    gitiles: m.gitiles && JSONToGitilesParentConfig(m.gitiles),
    copies: m.copies && m.copies.map(JSONToCopyParentConfig_CopyEntry),
  };
};

export interface DEPSLocalGitHubParentConfig {
  depsLocal?: DEPSLocalParentConfig;
  github?: GitHubConfig;
  forkRepoUrl: string;
}

interface DEPSLocalGitHubParentConfigJSON {
  deps_local?: DEPSLocalParentConfigJSON;
  github?: GitHubConfigJSON;
  fork_repo_url?: string;
}

const DEPSLocalGitHubParentConfigToJSON = (m: DEPSLocalGitHubParentConfig): DEPSLocalGitHubParentConfigJSON => {
  return {
    deps_local: m.depsLocal && DEPSLocalParentConfigToJSON(m.depsLocal),
    github: m.github && GitHubConfigToJSON(m.github),
    fork_repo_url: m.forkRepoUrl,
  };
};

const JSONToDEPSLocalGitHubParentConfig = (m: DEPSLocalGitHubParentConfigJSON): DEPSLocalGitHubParentConfig => {
  return {
    depsLocal: m.deps_local && JSONToDEPSLocalParentConfig(m.deps_local),
    github: m.github && JSONToGitHubConfig(m.github),
    forkRepoUrl: m.fork_repo_url || "",
  };
};

export interface DEPSLocalGerritParentConfig {
  depsLocal?: DEPSLocalParentConfig;
  gerrit?: GerritConfig;
}

interface DEPSLocalGerritParentConfigJSON {
  deps_local?: DEPSLocalParentConfigJSON;
  gerrit?: GerritConfigJSON;
}

const DEPSLocalGerritParentConfigToJSON = (m: DEPSLocalGerritParentConfig): DEPSLocalGerritParentConfigJSON => {
  return {
    deps_local: m.depsLocal && DEPSLocalParentConfigToJSON(m.depsLocal),
    gerrit: m.gerrit && GerritConfigToJSON(m.gerrit),
  };
};

const JSONToDEPSLocalGerritParentConfig = (m: DEPSLocalGerritParentConfigJSON): DEPSLocalGerritParentConfig => {
  return {
    depsLocal: m.deps_local && JSONToDEPSLocalParentConfig(m.deps_local),
    gerrit: m.gerrit && JSONToGerritConfig(m.gerrit),
  };
};

export interface GitCheckoutGitHubParentConfig {
  gitCheckout?: GitCheckoutParentConfig;
  forkRepoUrl: string;
}

interface GitCheckoutGitHubParentConfigJSON {
  git_checkout?: GitCheckoutParentConfigJSON;
  fork_repo_url?: string;
}

const GitCheckoutGitHubParentConfigToJSON = (m: GitCheckoutGitHubParentConfig): GitCheckoutGitHubParentConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutParentConfigToJSON(m.gitCheckout),
    fork_repo_url: m.forkRepoUrl,
  };
};

const JSONToGitCheckoutGitHubParentConfig = (m: GitCheckoutGitHubParentConfigJSON): GitCheckoutGitHubParentConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutParentConfig(m.git_checkout),
    forkRepoUrl: m.fork_repo_url || "",
  };
};

export interface GitCheckoutGitHubFileParentConfig {
  gitCheckout?: GitCheckoutGitHubParentConfig;
  preUploadSteps?: PreUploadStep[];
}

interface GitCheckoutGitHubFileParentConfigJSON {
  git_checkout?: GitCheckoutGitHubParentConfigJSON;
  pre_upload_steps?: string[];
}

const GitCheckoutGitHubFileParentConfigToJSON = (m: GitCheckoutGitHubFileParentConfig): GitCheckoutGitHubFileParentConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutGitHubParentConfigToJSON(m.gitCheckout),
    pre_upload_steps: m.preUploadSteps,
  };
};

const JSONToGitCheckoutGitHubFileParentConfig = (m: GitCheckoutGitHubFileParentConfigJSON): GitCheckoutGitHubFileParentConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutGitHubParentConfig(m.git_checkout),
    preUploadSteps: (m.pre_upload_steps || []) as PreUploadStep[],
  };
};

export interface GitilesParentConfig {
  gitiles?: GitilesConfig;
  dep?: DependencyConfig;
  gerrit?: GerritConfig;
}

interface GitilesParentConfigJSON {
  gitiles?: GitilesConfigJSON;
  dep?: DependencyConfigJSON;
  gerrit?: GerritConfigJSON;
}

const GitilesParentConfigToJSON = (m: GitilesParentConfig): GitilesParentConfigJSON => {
  return {
    gitiles: m.gitiles && GitilesConfigToJSON(m.gitiles),
    dep: m.dep && DependencyConfigToJSON(m.dep),
    gerrit: m.gerrit && GerritConfigToJSON(m.gerrit),
  };
};

const JSONToGitilesParentConfig = (m: GitilesParentConfigJSON): GitilesParentConfig => {
  return {
    gitiles: m.gitiles && JSONToGitilesConfig(m.gitiles),
    dep: m.dep && JSONToDependencyConfig(m.dep),
    gerrit: m.gerrit && JSONToGerritConfig(m.gerrit),
  };
};

export interface GitilesConfig {
  branch: string;
  repoUrl: string;
  dependencies?: VersionFileConfig[];
}

interface GitilesConfigJSON {
  branch?: string;
  repo_url?: string;
  dependencies?: VersionFileConfigJSON[];
}

const GitilesConfigToJSON = (m: GitilesConfig): GitilesConfigJSON => {
  return {
    branch: m.branch,
    repo_url: m.repoUrl,
    dependencies: m.dependencies && m.dependencies.map(VersionFileConfigToJSON),
  };
};

const JSONToGitilesConfig = (m: GitilesConfigJSON): GitilesConfig => {
  return {
    branch: m.branch || "",
    repoUrl: m.repo_url || "",
    dependencies: m.dependencies && m.dependencies.map(JSONToVersionFileConfig),
  };
};

export interface DEPSLocalParentConfig {
  gitCheckout?: GitCheckoutParentConfig;
  childPath: string;
  childSubdir: string;
  checkoutPath: string;
  gclientSpec: string;
  preUploadSteps?: PreUploadStep[];
  runHooks: boolean;
}

interface DEPSLocalParentConfigJSON {
  git_checkout?: GitCheckoutParentConfigJSON;
  child_path?: string;
  child_subdir?: string;
  checkout_path?: string;
  gclient_spec?: string;
  pre_upload_steps?: string[];
  run_hooks?: boolean;
}

const DEPSLocalParentConfigToJSON = (m: DEPSLocalParentConfig): DEPSLocalParentConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutParentConfigToJSON(m.gitCheckout),
    child_path: m.childPath,
    child_subdir: m.childSubdir,
    checkout_path: m.checkoutPath,
    gclient_spec: m.gclientSpec,
    pre_upload_steps: m.preUploadSteps,
    run_hooks: m.runHooks,
  };
};

const JSONToDEPSLocalParentConfig = (m: DEPSLocalParentConfigJSON): DEPSLocalParentConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutParentConfig(m.git_checkout),
    childPath: m.child_path || "",
    childSubdir: m.child_subdir || "",
    checkoutPath: m.checkout_path || "",
    gclientSpec: m.gclient_spec || "",
    preUploadSteps: (m.pre_upload_steps || []) as PreUploadStep[],
    runHooks: m.run_hooks || false,
  };
};

export interface GitCheckoutParentConfig {
  gitCheckout?: GitCheckoutConfig;
  dep?: DependencyConfig;
}

interface GitCheckoutParentConfigJSON {
  git_checkout?: GitCheckoutConfigJSON;
  dep?: DependencyConfigJSON;
}

const GitCheckoutParentConfigToJSON = (m: GitCheckoutParentConfig): GitCheckoutParentConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutConfigToJSON(m.gitCheckout),
    dep: m.dep && DependencyConfigToJSON(m.dep),
  };
};

const JSONToGitCheckoutParentConfig = (m: GitCheckoutParentConfigJSON): GitCheckoutParentConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutConfig(m.git_checkout),
    dep: m.dep && JSONToDependencyConfig(m.dep),
  };
};

export interface FreeTypeParentConfig {
  gitiles?: GitilesParentConfig;
}

interface FreeTypeParentConfigJSON {
  gitiles?: GitilesParentConfigJSON;
}

const FreeTypeParentConfigToJSON = (m: FreeTypeParentConfig): FreeTypeParentConfigJSON => {
  return {
    gitiles: m.gitiles && GitilesParentConfigToJSON(m.gitiles),
  };
};

const JSONToFreeTypeParentConfig = (m: FreeTypeParentConfigJSON): FreeTypeParentConfig => {
  return {
    gitiles: m.gitiles && JSONToGitilesParentConfig(m.gitiles),
  };
};

export interface CIPDChildConfig {
  name: string;
  tag: string;
}

interface CIPDChildConfigJSON {
  name?: string;
  tag?: string;
}

const CIPDChildConfigToJSON = (m: CIPDChildConfig): CIPDChildConfigJSON => {
  return {
    name: m.name,
    tag: m.tag,
  };
};

const JSONToCIPDChildConfig = (m: CIPDChildConfigJSON): CIPDChildConfig => {
  return {
    name: m.name || "",
    tag: m.tag || "",
  };
};

export interface FuchsiaSDKChildConfig {
  includeMacSdk: boolean;
}

interface FuchsiaSDKChildConfigJSON {
  include_mac_sdk?: boolean;
}

const FuchsiaSDKChildConfigToJSON = (m: FuchsiaSDKChildConfig): FuchsiaSDKChildConfigJSON => {
  return {
    include_mac_sdk: m.includeMacSdk,
  };
};

const JSONToFuchsiaSDKChildConfig = (m: FuchsiaSDKChildConfigJSON): FuchsiaSDKChildConfig => {
  return {
    includeMacSdk: m.include_mac_sdk || false,
  };
};

export interface SemVerGCSChildConfig {
  gcs?: GCSChildConfig;
  shortRevRegex: string;
  versionRegex: string;
}

interface SemVerGCSChildConfigJSON {
  gcs?: GCSChildConfigJSON;
  short_rev_regex?: string;
  version_regex?: string;
}

const SemVerGCSChildConfigToJSON = (m: SemVerGCSChildConfig): SemVerGCSChildConfigJSON => {
  return {
    gcs: m.gcs && GCSChildConfigToJSON(m.gcs),
    short_rev_regex: m.shortRevRegex,
    version_regex: m.versionRegex,
  };
};

const JSONToSemVerGCSChildConfig = (m: SemVerGCSChildConfigJSON): SemVerGCSChildConfig => {
  return {
    gcs: m.gcs && JSONToGCSChildConfig(m.gcs),
    shortRevRegex: m.short_rev_regex || "",
    versionRegex: m.version_regex || "",
  };
};

export interface GCSChildConfig {
  gcsBucket: string;
  gcsPath: string;
}

interface GCSChildConfigJSON {
  gcs_bucket?: string;
  gcs_path?: string;
}

const GCSChildConfigToJSON = (m: GCSChildConfig): GCSChildConfigJSON => {
  return {
    gcs_bucket: m.gcsBucket,
    gcs_path: m.gcsPath,
  };
};

const JSONToGCSChildConfig = (m: GCSChildConfigJSON): GCSChildConfig => {
  return {
    gcsBucket: m.gcs_bucket || "",
    gcsPath: m.gcs_path || "",
  };
};

export interface GitCheckoutChildConfig {
  gitCheckout?: GitCheckoutConfig;
}

interface GitCheckoutChildConfigJSON {
  git_checkout?: GitCheckoutConfigJSON;
}

const GitCheckoutChildConfigToJSON = (m: GitCheckoutChildConfig): GitCheckoutChildConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutConfigToJSON(m.gitCheckout),
  };
};

const JSONToGitCheckoutChildConfig = (m: GitCheckoutChildConfigJSON): GitCheckoutChildConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutConfig(m.git_checkout),
  };
};

export interface GitCheckoutGitHubChildConfig {
  gitCheckout?: GitCheckoutChildConfig;
  repoOwner: string;
  repoName: string;
}

interface GitCheckoutGitHubChildConfigJSON {
  git_checkout?: GitCheckoutChildConfigJSON;
  repo_owner?: string;
  repo_name?: string;
}

const GitCheckoutGitHubChildConfigToJSON = (m: GitCheckoutGitHubChildConfig): GitCheckoutGitHubChildConfigJSON => {
  return {
    git_checkout: m.gitCheckout && GitCheckoutChildConfigToJSON(m.gitCheckout),
    repo_owner: m.repoOwner,
    repo_name: m.repoName,
  };
};

const JSONToGitCheckoutGitHubChildConfig = (m: GitCheckoutGitHubChildConfigJSON): GitCheckoutGitHubChildConfig => {
  return {
    gitCheckout: m.git_checkout && JSONToGitCheckoutChildConfig(m.git_checkout),
    repoOwner: m.repo_owner || "",
    repoName: m.repo_name || "",
  };
};

export interface GitilesChildConfig {
  gitiles?: GitilesConfig;
  path: string;
}

interface GitilesChildConfigJSON {
  gitiles?: GitilesConfigJSON;
  path?: string;
}

const GitilesChildConfigToJSON = (m: GitilesChildConfig): GitilesChildConfigJSON => {
  return {
    gitiles: m.gitiles && GitilesConfigToJSON(m.gitiles),
    path: m.path,
  };
};

const JSONToGitilesChildConfig = (m: GitilesChildConfigJSON): GitilesChildConfig => {
  return {
    gitiles: m.gitiles && JSONToGitilesConfig(m.gitiles),
    path: m.path || "",
  };
};

export interface NotifierConfig {
  logLevel: NotifierConfig_LogLevel;
  msgType?: NotifierConfig_MsgType[];
  email?: EmailNotifierConfig;
  chat?: ChatNotifierConfig;
  monorail?: MonorailNotifierConfig;
  pubsub?: PubSubNotifierConfig;
  subject: string;
}

interface NotifierConfigJSON {
  log_level?: string;
  msg_type?: string[];
  email?: EmailNotifierConfigJSON;
  chat?: ChatNotifierConfigJSON;
  monorail?: MonorailNotifierConfigJSON;
  pubsub?: PubSubNotifierConfigJSON;
  subject?: string;
}

const NotifierConfigToJSON = (m: NotifierConfig): NotifierConfigJSON => {
  return {
    log_level: m.logLevel,
    msg_type: m.msgType,
    email: m.email && EmailNotifierConfigToJSON(m.email),
    chat: m.chat && ChatNotifierConfigToJSON(m.chat),
    monorail: m.monorail && MonorailNotifierConfigToJSON(m.monorail),
    pubsub: m.pubsub && PubSubNotifierConfigToJSON(m.pubsub),
    subject: m.subject,
  };
};

const JSONToNotifierConfig = (m: NotifierConfigJSON): NotifierConfig => {
  return {
    logLevel: (m.log_level || Object.keys(NotifierConfig_LogLevel)[0]) as NotifierConfig_LogLevel,
    msgType: (m.msg_type || []) as NotifierConfig_MsgType[],
    email: m.email && JSONToEmailNotifierConfig(m.email),
    chat: m.chat && JSONToChatNotifierConfig(m.chat),
    monorail: m.monorail && JSONToMonorailNotifierConfig(m.monorail),
    pubsub: m.pubsub && JSONToPubSubNotifierConfig(m.pubsub),
    subject: m.subject || "",
  };
};

export interface EmailNotifierConfig {
  emails?: string[];
}

interface EmailNotifierConfigJSON {
  emails?: string[];
}

const EmailNotifierConfigToJSON = (m: EmailNotifierConfig): EmailNotifierConfigJSON => {
  return {
    emails: m.emails,
  };
};

const JSONToEmailNotifierConfig = (m: EmailNotifierConfigJSON): EmailNotifierConfig => {
  return {
    emails: m.emails,
  };
};

export interface ChatNotifierConfig {
  roomId: string;
}

interface ChatNotifierConfigJSON {
  room_id?: string;
}

const ChatNotifierConfigToJSON = (m: ChatNotifierConfig): ChatNotifierConfigJSON => {
  return {
    room_id: m.roomId,
  };
};

const JSONToChatNotifierConfig = (m: ChatNotifierConfigJSON): ChatNotifierConfig => {
  return {
    roomId: m.room_id || "",
  };
};

export interface MonorailNotifierConfig {
  project: string;
  owner: string;
  cc?: string[];
  components?: string[];
  labels?: string[];
}

interface MonorailNotifierConfigJSON {
  project?: string;
  owner?: string;
  cc?: string[];
  components?: string[];
  labels?: string[];
}

const MonorailNotifierConfigToJSON = (m: MonorailNotifierConfig): MonorailNotifierConfigJSON => {
  return {
    project: m.project,
    owner: m.owner,
    cc: m.cc,
    components: m.components,
    labels: m.labels,
  };
};

const JSONToMonorailNotifierConfig = (m: MonorailNotifierConfigJSON): MonorailNotifierConfig => {
  return {
    project: m.project || "",
    owner: m.owner || "",
    cc: m.cc,
    components: m.components,
    labels: m.labels,
  };
};

export interface PubSubNotifierConfig {
  topic: string;
}

interface PubSubNotifierConfigJSON {
  topic?: string;
}

const PubSubNotifierConfigToJSON = (m: PubSubNotifierConfig): PubSubNotifierConfigJSON => {
  return {
    topic: m.topic,
  };
};

const JSONToPubSubNotifierConfig = (m: PubSubNotifierConfigJSON): PubSubNotifierConfig => {
  return {
    topic: m.topic || "",
  };
};

export interface ThrottleConfig {
  attemptCount: number;
  timeWindow: string;
}

interface ThrottleConfigJSON {
  attempt_count?: number;
  time_window?: string;
}

const ThrottleConfigToJSON = (m: ThrottleConfig): ThrottleConfigJSON => {
  return {
    attempt_count: m.attemptCount,
    time_window: m.timeWindow,
  };
};

const JSONToThrottleConfig = (m: ThrottleConfigJSON): ThrottleConfig => {
  return {
    attemptCount: m.attempt_count || 0,
    timeWindow: m.time_window || "",
  };
};

export interface TransitiveDepConfig {
  child?: VersionFileConfig;
  parent?: VersionFileConfig;
}

interface TransitiveDepConfigJSON {
  child?: VersionFileConfigJSON;
  parent?: VersionFileConfigJSON;
}

const TransitiveDepConfigToJSON = (m: TransitiveDepConfig): TransitiveDepConfigJSON => {
  return {
    child: m.child && VersionFileConfigToJSON(m.child),
    parent: m.parent && VersionFileConfigToJSON(m.parent),
  };
};

const JSONToTransitiveDepConfig = (m: TransitiveDepConfigJSON): TransitiveDepConfig => {
  return {
    child: m.child && JSONToVersionFileConfig(m.child),
    parent: m.parent && JSONToVersionFileConfig(m.parent),
  };
};

export interface VersionFileConfig {
  id: string;
  path: string;
}

interface VersionFileConfigJSON {
  id?: string;
  path?: string;
}

const VersionFileConfigToJSON = (m: VersionFileConfig): VersionFileConfigJSON => {
  return {
    id: m.id,
    path: m.path,
  };
};

const JSONToVersionFileConfig = (m: VersionFileConfigJSON): VersionFileConfig => {
  return {
    id: m.id || "",
    path: m.path || "",
  };
};

export interface DependencyConfig {
  primary?: VersionFileConfig;
  transitive?: TransitiveDepConfig[];
}

interface DependencyConfigJSON {
  primary?: VersionFileConfigJSON;
  transitive?: TransitiveDepConfigJSON[];
}

const DependencyConfigToJSON = (m: DependencyConfig): DependencyConfigJSON => {
  return {
    primary: m.primary && VersionFileConfigToJSON(m.primary),
    transitive: m.transitive && m.transitive.map(TransitiveDepConfigToJSON),
  };
};

const JSONToDependencyConfig = (m: DependencyConfigJSON): DependencyConfig => {
  return {
    primary: m.primary && JSONToVersionFileConfig(m.primary),
    transitive: m.transitive && m.transitive.map(JSONToTransitiveDepConfig),
  };
};

export interface GitCheckoutConfig {
  branch: string;
  repoUrl: string;
  revLinkTmpl: string;
  dependencies?: VersionFileConfig[];
}

interface GitCheckoutConfigJSON {
  branch?: string;
  repo_url?: string;
  rev_link_tmpl?: string;
  dependencies?: VersionFileConfigJSON[];
}

const GitCheckoutConfigToJSON = (m: GitCheckoutConfig): GitCheckoutConfigJSON => {
  return {
    branch: m.branch,
    repo_url: m.repoUrl,
    rev_link_tmpl: m.revLinkTmpl,
    dependencies: m.dependencies && m.dependencies.map(VersionFileConfigToJSON),
  };
};

const JSONToGitCheckoutConfig = (m: GitCheckoutConfigJSON): GitCheckoutConfig => {
  return {
    branch: m.branch || "",
    repoUrl: m.repo_url || "",
    revLinkTmpl: m.rev_link_tmpl || "",
    dependencies: m.dependencies && m.dependencies.map(JSONToVersionFileConfig),
  };
};

export interface BuildbucketRevisionFilterConfig {
  project: string;
  bucket: string;
}

interface BuildbucketRevisionFilterConfigJSON {
  project?: string;
  bucket?: string;
}

const BuildbucketRevisionFilterConfigToJSON = (m: BuildbucketRevisionFilterConfig): BuildbucketRevisionFilterConfigJSON => {
  return {
    project: m.project,
    bucket: m.bucket,
  };
};

const JSONToBuildbucketRevisionFilterConfig = (m: BuildbucketRevisionFilterConfigJSON): BuildbucketRevisionFilterConfig => {
  return {
    project: m.project || "",
    bucket: m.bucket || "",
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
  getConfig: (getConfigRequest: GetConfigRequest) => Promise<GetConfigResponse>;
  putConfig: (putConfigRequest: PutConfigRequest) => Promise<PutConfigResponse>;
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

  getConfig(getConfigRequest: GetConfigRequest): Promise<GetConfigResponse> {
    const url = this.hostname + this.pathPrefix + "GetConfig";
    let body: GetConfigRequest | GetConfigRequestJSON = getConfigRequest;
    if (!this.writeCamelCase) {
      body = GetConfigRequestToJSON(getConfigRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToGetConfigResponse);
    });
  }

  putConfig(putConfigRequest: PutConfigRequest): Promise<PutConfigResponse> {
    const url = this.hostname + this.pathPrefix + "PutConfig";
    let body: PutConfigRequest | PutConfigRequestJSON = putConfigRequest;
    if (!this.writeCamelCase) {
      body = PutConfigRequestToJSON(putConfigRequest);
    }
    return this.fetch(createTwirpRequest(url, body, this.optionsOverride)).then((resp) => {
      if (!resp.ok) {
        return throwTwirpError(resp);
      }

      return resp.json().then(JSONToPutConfigResponse);
    });
  }
}
