// A module to start and monitor the progress of long running server tasks.

import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { progress } from '../json';

// eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
export const startRequest = (startingURL: string, body: any, period: number, spinner: SpinnerSk): Promise<progress.SerializedProgress> => new Promise<progress.SerializedProgress>((resolve, reject) => {
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
        if (json.status === 'Running') {
          window.setTimeout(() => {
            processFetch(fetch(json.url, {
              method: 'GET',
            }));
          }, period);
        } else {
          resolve(json);
        }
      }).catch((msg) => {
        spinner.active = false;
        reject(msg);
      });
  };

  // Make the initial request that starts the polling process.
  processFetch(fetch(startingURL, {
    method: 'POST',
    body: JSON.stringify(body),
    headers: {
      'Content-Type': 'application/json',
    },
  }));
});
