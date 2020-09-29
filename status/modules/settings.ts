/**
 * This file lists helpers for returning global constants in Status. These should not change once
 * an instance has booted up, so they need not be returned via JSON requests.
 *
 * Settings are set via <script> on pages in status/pages, filled in by serverside template.
 */

class StatusSettings {
  public swarmingUrl: string = '';
  public taskSchedulerUrl: string = '';
  public defaultRepo: string = '';
}

// swarmingUrl: Base URL for linking to swarming task data.
export function swarmingUrl() {
  return (<any>window).StatusSettings?.swarmingUrl;
}

// taskSchedulerUrl: Base URL for linking to Task Scheduler data.
export function taskSchedulerUrl() {
  return (<any>window).StatusSettings?.taskSchedulerUrl;
}

// defaultRepo: Repo to use on initial load.
export function defaultRepo() {
  return (<any>window).StatusSettings?.defaultRepo;
}

// SetTestSettings: Inject setting values for tests.
export function SetTestSettings(s: StatusSettings) {
  (<any>window).StatusSettings = s;
}
