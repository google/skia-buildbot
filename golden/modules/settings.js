/**
 * This file lists helpers for returning global constants in Gold. These should not change once
 * an instance has booted up, so they need not be returned via JSON requests.
 *
 * Settings are expected to be in window.GoldSettings and the functions here are nice helpers
 * for that, so as to demystify "where do these values come from?"
 */

/**
 * @return {string}
 */
export function title() {
  return window.GoldSettings && window.GoldSettings.title;
}

/**
 * @return {string}
 */
export function defaultCorpus() {
  return window.GoldSettings && window.GoldSettings.defaultCorpus;
}

/**
 * @return {string}
 */
export function baseRepoURL() {
  return window.GoldSettings && window.GoldSettings.baseRepoURL;
}

/**
 * @return {string}
 */
export function codeReviewURLTemplate() {
  return window.GoldSettings && window.GoldSettings.crsTemplate;
}

/**
 * @param newSettings {Object}
 */
export function testOnlySetSettings(newSettings) {
  window.GoldSettings = newSettings;
}
