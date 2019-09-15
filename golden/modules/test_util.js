/** @module golden/test_util
 * @description
 *
 * <p>
 *  A general set of useful functions for tests and demos,
 *  e.g. reducing boilerplate.
 * </p>
 */

import { UNMATCHED } from 'fetch-mock';

/** expectNoUnmatchedCalls assets that there were no
 *  unexpected (unmatched) calls to fetchMock.
 */
export function expectNoUnmatchedCalls(fetchMock) {
    let calls = fetchMock.calls(UNMATCHED, 'GET');
    expect(calls.length, 'no unmatched (unexpected) GETs').to.equal(0);
    if (calls.length) {
      console.warn('unmatched GETS', calls);
    }
    calls = fetchMock.calls(UNMATCHED, 'POST');
    expect(calls.length, 'no unmatched (unexpected) POSTs').to.equal(0);
    if (calls.length) {
      console.warn('unmatched POSTS', calls);
    }
}