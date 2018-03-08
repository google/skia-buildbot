/** @module common/jsonOrThrow */

/** Helper function when making fetch() requests.
 *
 * Checks if the response is ok and converts it to JSON, otherwise it throws.
 *
 * @example
 *
 *    fetch('/_/list').then(jsonOrThrow).then(json => {
 *      // Do something with the parsed json here.
 *    }).catch(errorMessage);
 *
 * @returns {Promise}
 */
export function jsonOrThrow(resp) {
  if (resp.ok) {
    return resp.json();
  }
  throw 'Bad network response.';
}
