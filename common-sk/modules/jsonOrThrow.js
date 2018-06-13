/** @module common-sk/modules/jsonOrThrow */

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
 * @throws {Object} with status, body, and message elements. See the docs on
 *         a fetch Response for more detail on reading body (e.g. body.text()).
 */
export function jsonOrThrow(resp) {
  if (resp.ok) {
    return resp.json();
  }
  throw {
    message: `Bad network response: ${resp.statusText}`,
    body: resp.body,
    status: resp.status
  };
}
