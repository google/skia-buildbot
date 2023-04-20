/** errorMessage dispatches an event with the error message in it.
 * <p>
 *   Use this function in conjunction with the
 *   {@linkcode module:elements-sk/error-toast-sk} element.
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
 * @param message The value of 'message' is expected to be a string,
 *   an Object with a field 'message', an Object with a field 'resp'
 *   that is an instance of window.Response, or an Object.
 * @param duration The number of milliseconds the message should be
 * displayed.
 *
 */
export async function errorMessage(
  message: string | { message: string } | { resp: Response } | object,
  duration: number = 10000
) {
  if ((message as { resp: Response }).resp instanceof window.Response) {
    message = await (message as { resp: Response }).resp.text();
  } else if (typeof message === 'object') {
    message =
      // for handling Errors {name:String, message:String}
      (message as { message: string }).message ||
      // for everything else
      JSON.stringify(message);
  }
  const detail: ErrorSkEventDetail = {
    message: message,
    duration: duration,
  };
  document.dispatchEvent(
    new CustomEvent<ErrorSkEventDetail>('error-sk', {
      detail: detail,
      bubbles: true,
    })
  );
}

/** Defines the structure of the "error-sk" custom event's detail field. */
export interface ErrorSkEventDetail {
  readonly message: string;
  readonly duration: number;
}
