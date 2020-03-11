/**
 * Utility javascript functions used across the different CT FE pages.
 */
import { pad } from 'common-sk/modules/human';

/**
 * Converts the timestamp used in CTFE DB into a user friendly string.
 * */
export function getFormattedTimestamp(timestamp) {
  if (timestamp === 0) {
    return '<pending>';
  }
  return getTimestamp(timestamp).toLocaleString();
}

/**
 * Converts the timestamp used in CTFE DB into a Javascript timestamp.
 */
export function getTimestamp(timestamp) {
  if (timestamp === 0) {
    return timestamp;
  }
  const pattern = /(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/;
  return new Date(String(timestamp).replace(pattern, '$1-$2-$3T$4:$5:$6'));
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
