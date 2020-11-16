// A module to start and monitor the progress of long running server tasks.

import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { progress } from '../json';

export type callback = (arg: progress.SerializedProgress)=> void;

/**
 * startRequest returns a Promise that resolves then the long running server
 * side process has compeleted.
 *
 * The results of the long running process are provided in the
 * progress.SerializedProgress returned.
 *
 * @param startingURL - The URL to make the first request to.
 * @param body - The body to sent in a POST request to the first URL. Will be
 *        serialized to JSON before sending.
 * @param period - How often to check on the status of the long running proces.
 * @param spinner - The spinner-sk to start and stop.
 * @param cb - An optional callback that will be called every update period.
 */
export const startRequest = (
  startingURL: string,
  // eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
  body: any,
  period: number,
  spinner: SpinnerSk,
  cb: callback | null,
): Promise<progress.SerializedProgress> => new Promise<progress.SerializedProgress>((resolve, reject) => {
  spinner.active = true;

  // Regardless if this is the first fetch, or any of the subsequent polling
  // fetches, we do the same exact processing on the Promise, so consolidate all
  // the functionality into a single function.
  const processFetch = (fetchPromise: Promise<Response>) => {
    fetchPromise
      .then((resp: Response) => {
        if (!resp.ok) {
          reject(new Error(`Bad network response: ${resp.statusText}`));
        }
        return resp.json();
      })
      .then((json: progress.SerializedProgress) => {
        if (cb) {
          cb(json);
        }
        if (json.status === 'Running') {
          window.setTimeout(() => {
            processFetch(
              fetch(json.url, {
                method: 'GET',
              }),
            );
          }, period);
        } else {
          spinner.active = false;
          resolve(json);
        }
      })
      .catch((msg) => {
        spinner.active = false;
        reject(msg);
      });
  };

  // Make the initial request that starts the polling process.
  processFetch(
    fetch(startingURL, {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }),
  );
});
