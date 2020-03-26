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
 * @param str {string} string to shorten
 * @param maxLength {number} integer of length
 * @return {string}
 */
export function shorten(str, maxLength = 15) {
  if (str.length <= maxLength) {
    return str;
  }
  return `${str.substr(0, maxLength - 3)}...`;
}
