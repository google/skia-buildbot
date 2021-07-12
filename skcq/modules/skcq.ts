// Functions used by more than one element.
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

/**
 * Does a POST to the specified URL with the specified content.
 *
 * @param url - The URL to make the POST call to.
 * @param detail - Will be converted to JSON and specified as body of
 *                 the POST call.
 * @param action - The response of the POST call will be converted
 *                 to JSON and will be passed to the action function.
 */
export function doImpl<S, T>(url: string, detail: S, action: (json: T)=> any): void {
  fetch(url, {
    body: JSON.stringify(detail),
    headers: {
      'content-type': 'application/json',
    },
    credentials: 'include',
    method: 'POST',
  }).then(jsonOrThrow).then((json) => {
    action(json);
  }).catch((msg) => {
    console.error(msg); // eslint-disable-line no-console
    msg.resp.then(errorMessage);
  });
}
