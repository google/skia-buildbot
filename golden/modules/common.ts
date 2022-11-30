import { HintableObject } from 'common-sk/modules/hintable';
import { fromObject } from 'common-sk/modules/query';
import { Params } from './rpc_types';
import { SearchCriteria, SearchCriteriaToHintableObject } from './search-controls-sk/search-controls-sk';

/**
 * Takes a URL-encoded search query and returns that query with newlines between each of the
 * terms. This returned value should be easier for a human to understand.
 * @param queryStr URL-encoded query.
 * @return A human readable version of the input.
 */
export function humanReadableQuery(queryStr: string): string {
  if (!queryStr) {
    return '';
  }
  return queryStr.split('&').map(decodeURIComponent).join('\n');
}

const imagePrefix = '/img/images';
const diffPrefix = '/img/diffs';

/**
 * Returns a link to the PNG image associated with the given digest.
 * @param digest {string}
 * @return {string}
 */
export function digestImagePath(digest: string): string {
  if (!digest) {
    return '';
  }

  return `${imagePrefix}/${digest}.png`;
}

/** Returns a link to the PNG image associated with the diff between the given digests. */
export function digestDiffImagePath(d1: string, d2: string): string {
  if (!d1 || !d2) {
    return '';
  }
  // We have a canonical diff order where we sort the two digests alphabetically then join them
  // in order.
  const order = d1 < d2 ? `${d1}-${d2}` : `${d2}-${d1}`;
  return `${diffPrefix}/${order}.png`;
}

/**
 * Returns a link to the details page for a given test-digest pair.
 * @param grouping Grouping.
 * @param digest Digest.
 * @param clID CL ID. Optional, omit or use empty string for master branch.
 * @param crs Code review system. Optional, omit or use empty string for master branch.
 */
export function detailHref(grouping: Params, digest: string, clID = '', crs = ''): string {
  const u = `/detail?grouping=${encodeURIComponent(fromObject(grouping))}&digest=${digest}`;
  if (clID) {
    return `${u}&changelist_id=${clID}&crs=${crs}`;
  }
  return u;
}

/**
 * Returns a link to the diff page for a given pair of digests.
 * @param grouping Grouping.
 * @param left Left digest.
 * @param right Right digest.
 * @param clID CL ID. Optional, omit or use empty string for master branch.
 * @param crs Code review system. Optional, omit or use empty string for master branch.
 * @return {string}
 */
export function diffPageHref(
  grouping: Params,
  left: string,
  right: string,
  clID = '',
  crs = '',
): string {
  if (!left || !right) {
    return '';
  }

  const u = `/diff?grouping=${encodeURIComponent(fromObject(grouping))}`
    + `&left=${left}&right=${right}`;
  if (clID) {
    return `${u}&changelist_id=${clID}&crs=${crs}`;
  }
  return u;
}

/**
 * Returns a link to the cluster page for the given grouping and search criteria.
 *
 * Note that this function clears the search criteria's left-hand trace filter before constructing
 * the link.
 *
 * @param grouping Grouping.
 * @param searchCriteria Search criteria.
 * @param clID CL ID. Optional, omit or use empty string for master branch.
 * @param crs Code review system. Optional, omit or use empty string for master branch.
 */
export function clusterPageHref(
  grouping: Params,
  searchCriteria: Partial<SearchCriteria>,
  clID = '',
  crs = '',
): string {
  const searchCriteriaHintableObject = SearchCriteriaToHintableObject(searchCriteria);
  searchCriteriaHintableObject.left_filter = '';
  const u = `${'/cluster?'
    + `grouping=${encodeURIComponent(fromObject(grouping))}&`}${
    fromObject(searchCriteriaHintableObject as HintableObject)}`;
  if (clID) {
    return `${u}&changeListID=${clID}&crs=${crs}`;
  }
  return u;
}

/**
 * Helper to tell gold-scaffold-sk that a task has started and the spinner should be set to active.
 */
export function sendBeginTask(ele: Element) {
  ele.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
}

/**
 * Helper to tell gold-scaffold-sk that a task has finished and the spinner should maybe be stopped.
 */
export function sendEndTask(ele: Element) {
  ele.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
}

/** Detail of the fetch-error event. */
export interface FetchErrorEventDetail {
  error: any;
  loading: string;
}

/**
 * Helper to tell gold-scaffold-sk that a fetch failed. This will pop up on the toast-sk.
 * @param ele Element from which to dispatch the 'fetch-error' custom element.
 * @param e Error received from promise rejection.
 * @param what Description of what was being fetched.
 */
export function sendFetchError(ele: Element, e: any, what: string) {
  ele.dispatchEvent(new CustomEvent<FetchErrorEventDetail>('fetch-error', {
    detail: {
      error: e,
      loading: what,
    },
    bubbles: true,
  }));
}
