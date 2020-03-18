/**
 * Utility javascript functions used across the different CT FE pages.
 */
import { pad } from 'common-sk/modules/human';

/**
 * Converts the timestamp used in CTFE DB into a user friendly string.
 */
export function getFormattedTimestamp(timestamp) {
  if (!timestamp) {
    return '<pending>';
  }
  return getTimestamp(timestamp).toLocaleString();
}

const two_digits = 10**2;
const four_digits = 10**4;
/**
 * Converts the timestamp used in CTFE DB into a Javascript timestamp.
 */
export function getTimestamp(timestamp) {
  if (!timestamp) {
    return timestamp;
  }
  const date = new Date();
  // Timestamp is of the form YYYYMMDDhhmmss.
  // Consume the pieces off the right to build the date.
  const consumeDigits = (n) => {
    const first_n_digits = timestamp % (10**n);
    timestamp = (timestamp - first_n_digits) / (10**n)
    return first_n_digits;
  };
  date.setUTCSeconds(consumeDigits(2));
  date.setUTCMinutes(consumeDigits(2));
  date.setUTCHours(consumeDigits(2));
  date.setUTCDate(consumeDigits(2));
  date.setUTCMonth(consumeDigits(2) - 1); // Month is 0 based in JS.
  date.setUTCFullYear(consumeDigits(4));
  return date;
}

/**
 * Convert from Javascript Date to timestamp recognized by CTFE DB.
 */
export function getCtDbTimestamp(d) {
  const timestamp = String(d.getUTCFullYear()) + pad(d.getUTCMonth() + 1, 2)
                  + pad(d.getUTCDate(), 2) + pad(d.getUTCHours(), 2)
                  + pad(d.getUTCMinutes(), 2) + pad(d.getUTCSeconds(), 2);
  return timestamp;
}

/**
 * List of task types and the associated urls to fetch and delete them.
 */
export const taskDescriptors = [
  {
    type: 'ChromiumPerf',
    get_url: '/_/get_chromium_perf_tasks',
    delete_url: '/_/delete_chromium_perf_task',
  },
  {
    type: 'ChromiumAnalysis',
    get_url: '/_/get_chromium_analysis_tasks',
    delete_url: '/_/delete_chromium_analysis_task',
  },
  {
    type: 'MetricsAnalysis',
    get_url: '/_/get_metrics_analysis_tasks',
    delete_url: '/_/delete_metrics_analysis_task',
  },
  {
    type: 'CaptureSkps',
    get_url: '/_/get_capture_skp_tasks',
    delete_url: '/_/delete_capture_skps_task',
  },
  {
    type: 'LuaScript',
    get_url: '/_/get_lua_script_tasks',
    delete_url: '/_/delete_lua_script_task',
  },
  {
    type: 'ChromiumBuild',
    get_url: '/_/get_chromium_build_tasks',
    delete_url: '/_/delete_chromium_build_task',
  },
  {
    type: 'RecreatePageSets',
    get_url: '/_/get_recreate_page_sets_tasks',
    delete_url: '/_/delete_recreate_page_sets_task',
  },
  {
    type: 'RecreateWebpageArchives',
    get_url: '/_/get_recreate_webpage_archives_tasks',
    delete_url: '/_/delete_recreate_webpage_archives_task',
  },
];
