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
 * @param maxLength {number} integer of length
 * @return {string}
 */
export function shorten(str, maxLength = 15) {
  if (str.length <= maxLength) {
    return str;
  }
  return `${str.substr(0, maxLength - 3)}...`;
}

let imagePrefix = '/img/images';
let diffPrefix = '/img/diffs';

export function setImageEndpointsForDemos() {
  imagePrefix = '/dist';
  diffPrefix = '/dist';
}

/**
 * Returns a link to the png image associated with the given digest.
 * @param digest {string}
 * @return {string}
 */
export function imgSrc(digest) {
  if (!digest) {
    return '';
  }

  return `${imagePrefix}/${digest}.png`;
}

/**
 * Returns a link to the png image associated with the diff between the given digests.
 * @param d1 {string}
 * @param d2 {string}
 * @return {string}
 */
export function diffImgSrc(d1, d2) {
  if (!d1 || !d2) {
    return '';
  }
  // We have a canonical diff order where we sort the two digests alphabetically then join them
  // in order.
  const order = d1 < d2 ? `${d1}-${d2}` : `${d2}-${d1}`;
  return `${diffPrefix}/${order}.png`;
}

export function detailHref(test, digest, issue) {
  const u = `/detail?test=${test}&digest=${digest}`;
  if (issue) {
    return `${u}&issue=${issue}`;
  }
  return u;
}

export function diffPageHref(grouping, left, right, issue) {
  if (!left || !right) {
    return '';
  }

  return `/diff${diffQuery(grouping, left, right, issue)}`;
}

function diffQuery(test, left, right, issue) {
  const u = `?test=${test}&left=${left}&right=${right}`;
  if (issue) {
    return `${u}&issue=${issue}`;
  }
  return u;
}
