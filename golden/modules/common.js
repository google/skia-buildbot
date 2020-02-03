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
