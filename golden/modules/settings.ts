/**
 * This file lists helpers for returning global constants in Gold. These should not change once
 * an instance has booted up, so they need not be returned via JSON requests.
 *
 * Settings are expected to be in window.GoldSettings and the functions here are nice helpers
 * for that, so as to demystify "where do these values come from?"
 */

export interface GoldSettings {
  title?: string;
  defaultCorpus?: string;
  baseRepoURL?: string;
}

function getSettings() {
  return (window as any).GoldSettings as GoldSettings | undefined;
}

export function title() {
  return getSettings()?.title || '';
}

export function defaultCorpus() {
  return getSettings()?.defaultCorpus || '';
}

export function baseRepoURL() {
  return getSettings()?.baseRepoURL || '';
}

export function testOnlySetSettings(newSettings: GoldSettings) {
  (window as any).GoldSettings = newSettings;
}
