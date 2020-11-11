/**
 * This file lists helpers for returning global constants in Status. These should not change once
 * an instance has booted up, so they need not be returned via JSON requests.
 *
 * Settings are set via <script> on pages in status/pages, filled in by serverside template.
 */

class StatusSettings {
  public swarmingUrl: string = '';
  // Url with '{{TaskID}}' as a placeholder.
  public logsUrlTemplate: string = '';
  public taskSchedulerUrl: string = '';
  public defaultRepo: string = '';
  public repos: Map<string, string> = new Map();
}

function settings(): StatusSettings | undefined {
  return (<any>window).StatusSettings;
}

// swarmingUrl: Base URL for linking to swarming task data.
export function swarmingUrl() {
  return settings()?.swarmingUrl;
}

// taskSchedulerUrl: Base URL for linking to Task Scheduler data.
export function taskSchedulerUrl() {
  return settings()?.taskSchedulerUrl;
}

// logsUrl: Returns a logsUrl for the given taskId.
export function logsUrl(taskId: string): string {
  const temp = settings()?.logsUrlTemplate;
  if (!temp) {
    return '#';
  }
  if (temp.includes('annotations')) {
    if (!taskId.endsWith('0')) {
      return '#';
    }
    // Hack because chromium logs replaces a persistent trailing '0' with a '1' for log urls.
    taskId = taskId.slice(0, -1) + '1';
  }
  return temp.replace('{{TaskID}}', taskId);
}

// defaultRepo: Repo to use on initial load.
export function defaultRepo() {
  return settings()?.defaultRepo || '';
}

// repos: List of available repos.
export function repos() {
  const r = settings()?.repos;
  return r ? [...r.keys()] : [];
}

// revisionUrlTemplate: Returns the base url for a repo's revisions. Can be
// concatenated with a hash to form a valid url.
export function revisionUrlTemplate(repo: string) {
  return settings()?.repos.get(repo);
}

// SetTestSettings: Inject setting values for tests.
export function SetTestSettings(s: StatusSettings) {
  (<any>window).StatusSettings = s;
}
