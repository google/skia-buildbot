import {createTwirpRequest, throwTwirpError, Fetch} from './twirp';

export enum Mode {
  RUNNING = "RUNNING",
  STOPPED = "STOPPED",
  DRY_RUN = "DRY_RUN",
  OFFLINE = "OFFLINE",
}

export enum CommitMsgConfig_BuiltIn {
  DEFAULT = "DEFAULT",
  ANDROID = "ANDROID",
  ANDROID_NO_CR = "ANDROID_NO_CR",
  CANARY = "CANARY",
}

export enum GerritConfig_Config {
  ANDROID = "ANDROID",
  ANGLE = "ANGLE",
  CHROMIUM = "CHROMIUM",
  CHROMIUM_NO_CQ = "CHROMIUM_NO_CQ",
  LIBASSISTANT = "LIBASSISTANT",
  CHROMIUM_BOT_COMMIT = "CHROMIUM_BOT_COMMIT",
  CHROMIUM_BOT_COMMIT_NO_CQ = "CHROMIUM_BOT_COMMIT_NO_CQ",
  ANDROID_NO_CR = "ANDROID_NO_CR",
  ANDROID_NO_CR_NO_PR = "ANDROID_NO_CR_NO_PR",
  CHROMIUM_NO_CR = "CHROMIUM_NO_CR",
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
  MANUAL_ROLL_CREATION_FAILED = "MANUAL_ROLL_CREATION_FAILED",
}

export interface Config {
  rollerName: string;
  childBugLink: string;
  childDisplayName: string;
  parentBugLink: string;
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
  dryRunCooldown: string;
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
  google3RepoManager?: Google3RepoManagerConfig;
  notifiers?: NotifierConfig[];
  safetyThrottle?: ThrottleConfig;
  transitiveDeps?: TransitiveDepConfig[];
  useWorkloadIdentity: boolean;
  validModes?: Mode[];
  maxRollCqAttempts: number;
  maxRollClsToSameRevision: number;
}

interface ConfigJSON {
  roller_name?: string;
  child_bug_link?: string;
  child_display_name?: string;
  parent_bug_link?: string;
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
  dry_run_cooldown?: string;
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
  google3_repo_manager?: Google3RepoManagerConfigJSON;
  notifiers?: NotifierConfigJSON[];
  safety_throttle?: ThrottleConfigJSON;
  transitive_deps?: TransitiveDepConfigJSON[];
  use_workload_identity?: boolean;
  valid_modes?: string[];
  max_roll_cq_attempts?: number;
  max_roll_cls_to_same_revision?: number;
}

export interface CommitMsgConfig {
  bugProject: string;
  childLogUrlTmpl: string;
  cqExtraTrybots?: string[];
  cqDoNotCancelTrybots: boolean;
  includeLog: boolean;
  includeRevisionCount: boolean;
  includeTbrLine: boolean;
  includeTests: boolean;
  extraFooters?: string[];
  wordWrap: number;
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
  extra_footers?: string[];
  word_wrap?: number;
  built_in?: string;
  custom?: string;
}

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

export interface GitHubConfig {
  repoOwner: string;
  repoName: string;
  checksWaitFor?: string[];
  tokenSecret: string;
  sshKeySecret: string;
}

interface GitHubConfigJSON {
  repo_owner?: string;
  repo_name?: string;
  checks_wait_for?: string[];
  token_secret?: string;
  ssh_key_secret?: string;
}

export interface Google3Config {
}

interface Google3ConfigJSON {
}

export interface KubernetesConfig {
  cpu: string;
  memory: string;
  readinessFailureThreshold: number;
  readinessInitialDelaySeconds: number;
  readinessPeriodSeconds: number;
  disk: string;
  image: string;
  extraFlags?: string[];
}

interface KubernetesConfigJSON {
  cpu?: string;
  memory?: string;
  readiness_failure_threshold?: number;
  readiness_initial_delay_seconds?: number;
  readiness_period_seconds?: number;
  disk?: string;
  image?: string;
  extra_flags?: string[];
}

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

export interface AndroidRepoManagerConfig {
  childRepoUrl: string;
  childBranch: string;
  childPath: string;
  parentRepoUrl: string;
  parentBranch: string;
  childRevLinkTmpl: string;
  childSubdir: string;
  metadata?: AndroidRepoManagerConfig_ProjectMetadataFileConfig;
  includeAuthorsAsReviewers: boolean;
  preUploadCommands?: PreUploadConfig;
  autoApproverSecret: string;
  defaultBugProject: string;
}

interface AndroidRepoManagerConfigJSON {
  child_repo_url?: string;
  child_branch?: string;
  child_path?: string;
  parent_repo_url?: string;
  parent_branch?: string;
  child_rev_link_tmpl?: string;
  child_subdir?: string;
  metadata?: AndroidRepoManagerConfig_ProjectMetadataFileConfigJSON;
  include_authors_as_reviewers?: boolean;
  pre_upload_commands?: PreUploadConfigJSON;
  auto_approver_secret?: string;
  default_bug_project?: string;
}

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

export interface FreeTypeRepoManagerConfig {
  parent?: FreeTypeParentConfig;
  child?: GitilesChildConfig;
}

interface FreeTypeRepoManagerConfigJSON {
  parent?: FreeTypeParentConfigJSON;
  child?: GitilesChildConfigJSON;
}

export interface Google3RepoManagerConfig {
  childBranch: string;
  childRepo: string;
}

interface Google3RepoManagerConfigJSON {
  child_branch?: string;
  child_repo?: string;
}

export interface ParentChildRepoManagerConfig {
  copyParent?: CopyParentConfig;
  depsLocalGithubParent?: DEPSLocalGitHubParentConfig;
  depsLocalGerritParent?: DEPSLocalGerritParentConfig;
  gitCheckoutGithubFileParent?: GitCheckoutGitHubFileParentConfig;
  gitilesParent?: GitilesParentConfig;
  goModGerritParent?: GoModGerritParentConfig;
  gitCheckoutGerritParent?: GitCheckoutGerritParentConfig;
  cipdChild?: CIPDChildConfig;
  fuchsiaSdkChild?: FuchsiaSDKChildConfig;
  gitCheckoutChild?: GitCheckoutChildConfig;
  gitCheckoutGithubChild?: GitCheckoutGitHubChildConfig;
  gitilesChild?: GitilesChildConfig;
  gitSemverChild?: GitSemVerChildConfig;
  semverGcsChild?: SemVerGCSChildConfig;
  dockerChild?: DockerChildConfig;
  buildbucketRevisionFilter?: BuildbucketRevisionFilterConfig[];
  cipdRevisionFilter?: CIPDRevisionFilterConfig[];
  validHttpRevisionFilter?: ValidHttpRevisionFilterConfig[];
}

interface ParentChildRepoManagerConfigJSON {
  copy_parent?: CopyParentConfigJSON;
  deps_local_github_parent?: DEPSLocalGitHubParentConfigJSON;
  deps_local_gerrit_parent?: DEPSLocalGerritParentConfigJSON;
  git_checkout_github_file_parent?: GitCheckoutGitHubFileParentConfigJSON;
  gitiles_parent?: GitilesParentConfigJSON;
  go_mod_gerrit_parent?: GoModGerritParentConfigJSON;
  git_checkout_gerrit_parent?: GitCheckoutGerritParentConfigJSON;
  cipd_child?: CIPDChildConfigJSON;
  fuchsia_sdk_child?: FuchsiaSDKChildConfigJSON;
  git_checkout_child?: GitCheckoutChildConfigJSON;
  git_checkout_github_child?: GitCheckoutGitHubChildConfigJSON;
  gitiles_child?: GitilesChildConfigJSON;
  git_semver_child?: GitSemVerChildConfigJSON;
  semver_gcs_child?: SemVerGCSChildConfigJSON;
  docker_child?: DockerChildConfigJSON;
  buildbucket_revision_filter?: BuildbucketRevisionFilterConfigJSON[];
  cipd_revision_filter?: CIPDRevisionFilterConfigJSON[];
  valid_http_revision_filter?: ValidHttpRevisionFilterConfigJSON[];
}

export interface CopyParentConfig_CopyEntry {
  srcRelPath: string;
  dstRelPath: string;
}

interface CopyParentConfig_CopyEntryJSON {
  src_rel_path?: string;
  dst_rel_path?: string;
}

export interface CopyParentConfig {
  gitiles?: GitilesParentConfig;
  copies?: CopyParentConfig_CopyEntry[];
}

interface CopyParentConfigJSON {
  gitiles?: GitilesParentConfigJSON;
  copies?: CopyParentConfig_CopyEntryJSON[];
}

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

export interface DEPSLocalGerritParentConfig {
  depsLocal?: DEPSLocalParentConfig;
  gerrit?: GerritConfig;
}

interface DEPSLocalGerritParentConfigJSON {
  deps_local?: DEPSLocalParentConfigJSON;
  gerrit?: GerritConfigJSON;
}

export interface GitCheckoutGitHubParentConfig {
  gitCheckout?: GitCheckoutParentConfig;
  forkRepoUrl: string;
}

interface GitCheckoutGitHubParentConfigJSON {
  git_checkout?: GitCheckoutParentConfigJSON;
  fork_repo_url?: string;
}

export interface GitCheckoutGerritParentConfig {
  gitCheckout?: GitCheckoutParentConfig;
  preUploadCommands?: PreUploadConfig;
}

interface GitCheckoutGerritParentConfigJSON {
  git_checkout?: GitCheckoutParentConfigJSON;
  pre_upload_commands?: PreUploadConfigJSON;
}

export interface GitCheckoutGitHubFileParentConfig {
  gitCheckout?: GitCheckoutGitHubParentConfig;
  preUploadCommands?: PreUploadConfig;
}

interface GitCheckoutGitHubFileParentConfigJSON {
  git_checkout?: GitCheckoutGitHubParentConfigJSON;
  pre_upload_commands?: PreUploadConfigJSON;
}

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

export interface GitilesConfig {
  branch: string;
  repoUrl: string;
  dependencies?: VersionFileConfig[];
  defaultBugProject: string;
}

interface GitilesConfigJSON {
  branch?: string;
  repo_url?: string;
  dependencies?: VersionFileConfigJSON[];
  default_bug_project?: string;
}

export interface GoModGerritParentConfig {
  goMod?: GoModParentConfig;
  gerrit?: GerritConfig;
}

interface GoModGerritParentConfigJSON {
  go_mod?: GoModParentConfigJSON;
  gerrit?: GerritConfigJSON;
}

export interface GoModParentConfig {
  gitCheckout?: GitCheckoutConfig;
  modulePath: string;
  findAndReplace?: string[];
  preUploadCommands?: PreUploadConfig;
  goCmd: string;
}

interface GoModParentConfigJSON {
  git_checkout?: GitCheckoutConfigJSON;
  module_path?: string;
  find_and_replace?: string[];
  pre_upload_commands?: PreUploadConfigJSON;
  go_cmd?: string;
}

export interface DEPSLocalParentConfig {
  gitCheckout?: GitCheckoutParentConfig;
  childPath: string;
  childSubdir: string;
  checkoutPath: string;
  gclientSpec: string;
  runHooks: boolean;
  preUploadCommands?: PreUploadConfig;
  parentSubdir: string;
}

interface DEPSLocalParentConfigJSON {
  git_checkout?: GitCheckoutParentConfigJSON;
  child_path?: string;
  child_subdir?: string;
  checkout_path?: string;
  gclient_spec?: string;
  run_hooks?: boolean;
  pre_upload_commands?: PreUploadConfigJSON;
  parent_subdir?: string;
}

export interface GitCheckoutParentConfig {
  gitCheckout?: GitCheckoutConfig;
  dep?: DependencyConfig;
}

interface GitCheckoutParentConfigJSON {
  git_checkout?: GitCheckoutConfigJSON;
  dep?: DependencyConfigJSON;
}

export interface FreeTypeParentConfig {
  gitiles?: GitilesParentConfig;
}

interface FreeTypeParentConfigJSON {
  gitiles?: GitilesParentConfigJSON;
}

export interface CIPDChildConfig {
  name: string;
  tag: string;
  gitilesRepo: string;
  revisionIdTag: string;
  revisionIdTagStripKey: boolean;
  sourceRepo?: GitilesConfig;
  platform?: string[];
}

interface CIPDChildConfigJSON {
  name?: string;
  tag?: string;
  gitiles_repo?: string;
  revision_id_tag?: string;
  revision_id_tag_strip_key?: boolean;
  source_repo?: GitilesConfigJSON;
  platform?: string[];
}

export interface FuchsiaSDKChildConfig {
  includeMacSdk: boolean;
  gcsBucket: string;
  latestLinuxPath: string;
  latestMacPath: string;
  tarballLinuxPathTmpl: string;
}

interface FuchsiaSDKChildConfigJSON {
  include_mac_sdk?: boolean;
  gcs_bucket?: string;
  latest_linux_path?: string;
  latest_mac_path?: string;
  tarball_linux_path_tmpl?: string;
}

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

export interface GCSChildConfig {
  gcsBucket: string;
  gcsPath: string;
  revisionIdRegex: string;
}

interface GCSChildConfigJSON {
  gcs_bucket?: string;
  gcs_path?: string;
  revision_id_regex?: string;
}

export interface GitCheckoutChildConfig {
  gitCheckout?: GitCheckoutConfig;
}

interface GitCheckoutChildConfigJSON {
  git_checkout?: GitCheckoutConfigJSON;
}

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

export interface GitilesChildConfig {
  gitiles?: GitilesConfig;
  path: string;
}

interface GitilesChildConfigJSON {
  gitiles?: GitilesConfigJSON;
  path?: string;
}

export interface GitSemVerChildConfig {
  gitiles?: GitilesConfig;
  versionRegex: string;
}

interface GitSemVerChildConfigJSON {
  gitiles?: GitilesConfigJSON;
  version_regex?: string;
}

export interface DockerChildConfig {
  registry: string;
  repository: string;
  tag: string;
}

interface DockerChildConfigJSON {
  registry?: string;
  repository?: string;
  tag?: string;
}

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

export interface EmailNotifierConfig {
  emails?: string[];
}

interface EmailNotifierConfigJSON {
  emails?: string[];
}

export interface ChatNotifierConfig {
  roomId: string;
}

interface ChatNotifierConfigJSON {
  room_id?: string;
}

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

export interface PubSubNotifierConfig {
  topic: string;
}

interface PubSubNotifierConfigJSON {
  topic?: string;
}

export interface ThrottleConfig {
  attemptCount: number;
  timeWindow: string;
}

interface ThrottleConfigJSON {
  attempt_count?: number;
  time_window?: string;
}

export interface TransitiveDepConfig {
  child?: VersionFileConfig;
  parent?: VersionFileConfig;
  logUrlTmpl: string;
}

interface TransitiveDepConfigJSON {
  child?: VersionFileConfigJSON;
  parent?: VersionFileConfigJSON;
  log_url_tmpl?: string;
}

export interface VersionFileConfig {
  id: string;
  file?: VersionFileConfig_File[];
}

interface VersionFileConfigJSON {
  id?: string;
  file?: VersionFileConfig_FileJSON[];
}

export interface VersionFileConfig_File {
  path: string;
  regex: string;
  regexReplaceAll: boolean;
}

interface VersionFileConfig_FileJSON {
  path?: string;
  regex?: string;
  regex_replace_all?: boolean;
}

export interface DependencyConfig {
  primary?: VersionFileConfig;
  transitive?: TransitiveDepConfig[];
  findAndReplace?: string[];
}

interface DependencyConfigJSON {
  primary?: VersionFileConfigJSON;
  transitive?: TransitiveDepConfigJSON[];
  find_and_replace?: string[];
}

export interface GitCheckoutConfig {
  branch: string;
  repoUrl: string;
  revLinkTmpl: string;
  dependencies?: VersionFileConfig[];
  defaultBugProject: string;
}

interface GitCheckoutConfigJSON {
  branch?: string;
  repo_url?: string;
  rev_link_tmpl?: string;
  dependencies?: VersionFileConfigJSON[];
  default_bug_project?: string;
}

export interface BuildbucketRevisionFilterConfig {
  project: string;
  bucket: string;
  buildsetCommitTmpl: string;
  builder?: string[];
}

interface BuildbucketRevisionFilterConfigJSON {
  project?: string;
  bucket?: string;
  buildset_commit_tmpl?: string;
  builder?: string[];
}

export interface CIPDRevisionFilterConfig {
  package?: string[];
  platform?: string[];
  tagKey: string;
}

interface CIPDRevisionFilterConfigJSON {
  package?: string[];
  platform?: string[];
  tag_key?: string;
}

export interface ValidHttpRevisionFilterConfig {
  fileUrl: string;
  regex: string;
}

interface ValidHttpRevisionFilterConfigJSON {
  file_url?: string;
  regex?: string;
}

export interface PreUploadConfig {
  cipdPackage?: PreUploadCIPDPackageConfig[];
  command?: PreUploadCommandConfig[];
}

interface PreUploadConfigJSON {
  cipd_package?: PreUploadCIPDPackageConfigJSON[];
  command?: PreUploadCommandConfigJSON[];
}

export interface PreUploadCommandConfig {
  command: string;
  cwd: string;
  env?: string[];
  ignoreFailure: boolean;
}

interface PreUploadCommandConfigJSON {
  command?: string;
  cwd?: string;
  env?: string[];
  ignore_failure?: boolean;
}

export interface PreUploadCIPDPackageConfig {
  name: string;
  path: string;
  version: string;
}

interface PreUploadCIPDPackageConfigJSON {
  name?: string;
  path?: string;
  version?: string;
}

export interface Configs {
  config?: Config[];
}

interface ConfigsJSON {
  config?: ConfigJSON[];
}
