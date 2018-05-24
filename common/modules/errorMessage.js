/** @module common-sk/modules/errorMessage */

/** errorMessage dispatches an event with the error message in it.
 * <p>
 *   Use this function in conjunction with the {@linkcode module:common-sk/modules/error-toast-sk} element.
 * </p>
 *
 * @evt error-sk The event has a detail of the form:
 *
 * <pre>
 * {
 *   message: "some string",
 *   duration: (duration in milliseconds),
 * }
 * </pre>
 *
 * @param {(string|Object)} message The value of 'message' is expected to be a
 *   string, an Object with a field 'message', or an Object.
 * @param {number} duration The number of milliseconds the message should be displayed.
 *
 */
export function errorMessage(message, duration=10000) {
  if (typeof message === 'object') {
    message = message.message        || // for handling Errors {name:String, message:String}
              JSON.stringify(message);  // for everything else
  }
  var detail = {
    message: message,
    duration: duration,
  }
  document.dispatchEvent(new CustomEvent('error-sk', {detail: detail, bubbles: true}));
}
