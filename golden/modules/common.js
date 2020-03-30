/**
 * Takes a URL-encoded search query and returns that query with newlines between each of the
 * terms. This returned value should be easier for a human to understand.
 * @param queryStr {string} URL-encoded query.
 * @return {string} a human readable version of the input.
 */
export function humanReadableQuery(queryStr) {
  if (!queryStr) {
    return '';
  }
  return queryStr.split('&').map(decodeURIComponent).join('\n');
}

/**
 * Takes a string and trims it to be no longer than maxLength. If the string needs to be trimmed,
 * an ellipsis (...) will be added as a suffix, but the total string length (with ellipsis) will
 * stay under maxLength.
 * @param str {string}
 * @param maxLength {number}
 * @return {string}
 */
export function truncateWithEllipses(str, maxLength = 15) {
  if (maxLength < 3) {
    throw 'maxLength must be greater than the length of the ellipsis.';
  }
  if (str.length <= maxLength) {
    return str;
  }
  return `${str.substr(0, maxLength - 3)}...`;
}

let imagePrefix = '/img/images';
let diffPrefix = '/img/diffs';

/**
 * Changes the image endpoints to be based on /dist, which is where they are served when running
 * the demo pages locally. It should *not* be called by tests, as the effects will persist and make
 * for interdependent tests.
 */
export function setImageEndpointsForDemos() {
  imagePrefix = '/dist';
  diffPrefix = '/dist';
}

/**
 * Returns a link to the PNG image associated with the given digest.
 * @param digest {string}
 * @return {string}
 */
export function digestImagePath(digest) {
  if (!digest) {
    return '';
  }

  return `${imagePrefix}/${digest}.png`;
}

/**
 * Returns a link to the PNG image associated with the diff between the given digests.
 * @param d1 {string}
 * @param d2 {string}
 * @return {string}
 */
export function digestDiffImagePath(d1, d2) {
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
 * @param test {string}
 * @param digest {string}
 * @param issue {string} Optional, omit or use empty string for master branch.
 * @return {string}
 */
export function detailHref(test, digest, issue = '') {
  const u = `/detail?test=${test}&digest=${digest}`;
  if (issue) {
    return `${u}&issue=${issue}`;
  }
  return u;
}

/**
 * Returns a link to the diff page for a given pair of digests.
 * @param grouping {string}
 * @param left {string}
 * @param right {string}
 * @param issue {string} Optional, omit or use empty string for master branch.
 * @return {string}
 */
export function diffPageHref(grouping, left, right, issue = '') {
  if (!left || !right) {
    return '';
  }

  const u = `/diff?test=${grouping}&left=${left}&right=${right}`;
  if (issue) {
    return `${u}&issue=${issue}`;
  }
  return u;
}
