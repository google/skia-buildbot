// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/** @module common-sk/modules/jsonOrThrow */

// ** The type of Error thrown by jsonOrThrow. */
export class JsonOrThrowError extends Error {
  message: string;

  resp: Response;

  status: number;

  constructor(resp: Response) {
    super();
    this.message = `Bad network response: ${resp.statusText}`;
    this.resp = resp;
    this.status = resp.status;
  }
}

/** Helper function when making fetch() requests.
 *
 * Checks if the response is ok and converts it to JSON, otherwise it throws.
 *
 * @example
 *
 *    fetch('/_/list').then(jsonOrThrow).then((json) => {
 *      // Do something with the parsed json here.
 *    }).catch((r) => {
 *      if (r.status === 403) {
 *        // Handle HTTP response 403 - not authorized here.
 *      } else {
 *        console.err(r.message);
 *      }
 *    });
 *
 * @throws A JsonOrThrowErrr. See the
 *   [Response docs]{@link https://developer.mozilla.org/en-US/docs/Web/API/Response}
 *   for more detail on reading resp (e.g. resp.text()).
 */
export function jsonOrThrow(resp: Response) {
  if (resp.ok) {
    return resp.json();
  }
  throw new JsonOrThrowError(resp);
}
