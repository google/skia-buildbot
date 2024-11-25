/**
 * Utility javascript functions used across the different CT FE pages.
 */
import { pad } from '../../infra-sk/modules/human';
import { fromObject } from '../../infra-sk/modules/query';
import { jsonOrThrow } from '../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../../elements-sk/modules/errorMessage';
import { BenchmarksPlatformsResponse } from './json';

/**
 * Converts the timestamp used in CTFE DB into a user friendly string.
 */
export function getFormattedTimestamp(timestamp: number): string {
  if (!timestamp) {
    return '<pending>';
  }
  return getTimestamp(timestamp).toLocaleString();
}

/**
 * Converts the timestamp used in CTFE DB into a Javascript timestamp.
 */
export function getTimestamp(timestamp: number): Date {
  const date = new Date();
  if (!timestamp) {
    return date;
  }
  // Timestamp is of the form YYYYMMDDhhmmss.
  // Consume the pieces off the right to build the date.
  const consumeDigits = (n: number) => {
    const first_n_digits = timestamp % 10 ** n;
    timestamp = (timestamp - first_n_digits) / 10 ** n;
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
export function getCtDbTimestamp(d: Date): string {
  const timestamp =
    String(d.getUTCFullYear()) +
    pad(d.getUTCMonth() + 1, 2) +
    pad(d.getUTCDate(), 2) +
    pad(d.getUTCHours(), 2) +
    pad(d.getUTCMinutes(), 2) +
    pad(d.getUTCSeconds(), 2);
  return timestamp;
}

/**
 * Append gsPath with appropriate url to fetch from ct.skia.org.
 */
export function getGSLink(gsPath: string): string {
  return `https://ct.skia.org/results/cluster-telemetry/${gsPath}`;
}

/**
 * Returns true if gsPath is not set or if the patch's SHA1 digest in the specified
 * google storage path is for an empty string.
 */
export function isEmptyPatch(gsPath: string): boolean {
  // Compare against empty string and against the SHA1 digest of an empty string.
  return gsPath === '' || gsPath === 'patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch';
}

/**
 * Express numeric days in a readable format (e.g. 'Weekly'; 'Every 3 days')
 */
export function formatRepeatAfterDays(num: number): string {
  if (num === 0) {
    return 'N/A';
  }
  if (num === 1) {
    return 'Daily';
  }
  if (num === 7) {
    return 'Weekly';
  }
  return `Every ${num} days`;
}

/**
 * Fetches benchmarks with doc links, and platforms with descriptions.
 *
 * @param {func<Object>} func - Function called with fetched benchmarks and
 * platforms object.
 */
export function fetchBenchmarksAndPlatforms(
  func: (json: BenchmarksPlatformsResponse) => void
): void {
  fetch('/_/benchmarks_platforms/', {
    method: 'POST',
  })
    .then(jsonOrThrow)
    .then(func)
    .catch(errorMessage);
}

/**
 *
 * @param {Array<string>} descriptions - Array of CL descriptions, combined
 * into a description string if at least one is noneempty.
 *
 * @returns string - Combined description.
 */
export function combineClDescriptions(descriptions: string[]): string {
  const combinedDesc = descriptions
    .filter(Boolean)
    .reduce((str, desc) => (str += str === '' ? desc : ` and ${desc}`), '');
  return combinedDesc ? `Testing ${combinedDesc}` : '';
}

export function missingLiveSitesWithCustomWebpages(
  customWebpages: string,
  benchmarkArgs: string
): boolean {
  if (customWebpages && !benchmarkArgs.includes('--use-live-sites')) {
    errorMessage(
      'Please specify --use-live-sites in benchmark arguments ' + 'when using custom web pages.'
    );
    return true;
  }
  return false;
}

let activeTasks = 0;
/**
 * Asynchronously queries the logged in user's active task count.
 * This is best effort, so doesn't bother with returning a promise.
 *
 * @returns function() boolean : Whether or not the task count
 * previously fetched is more than 3.
 */
export function moreThanThreeActiveTasksChecker(): () => boolean {
  const queryParams = {
    size: 1,
    not_completed: true,
    filter_by_logged_in_user: true,
  };
  const queryStr = `?${fromObject(queryParams)}`;

  taskDescriptors.forEach((obj) => {
    fetch(obj.get_url + queryStr, {
      method: 'POST',
    })
      .then(jsonOrThrow)
      .then((json) => {
        activeTasks += json.pagination.total;
      })
      .catch(errorMessage);
  });
  return () => {
    if (activeTasks > 3) {
      errorMessage(
        `You have ${activeTasks} currently running tasks. Please wait` +
          ' for them to complete before scheduling more CT tasks.'
      );
      return true;
    }
    return false;
  };
}

/**
 * Used to describe the type and get/delete URLs of various CT tasks.
 */
export interface TaskDescriptor {
  type: string;
  get_url: string;
  delete_url: string;
}

/**
 * List of task types and the associated urls to fetch and delete them.
 */
export const taskDescriptors: Array<TaskDescriptor> = [
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
