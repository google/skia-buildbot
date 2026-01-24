// A module to start and monitor the progress of long running server tasks.

import { progress } from '../json';

export interface RequestOptions {
  /** Triggered when the request process is initiated. */
  onStart?: () => void;

  /** Triggered each time a progress update is received from the server. */
  onProgressUpdate?: (data: progress.SerializedProgress) => void;

  /** Triggered only upon successful completion of the long-running process. */
  onSuccess?: (data: progress.SerializedProgress) => void;

  /** Triggered when the process ends, whether it succeeded or failed (like 'finally'). */
  onSettled?: () => void;

  /** Time between poll requests in milliseconds. Defaults to 200. */
  pollingIntervalMs?: number;
}

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
 * @param options - Optional configuration for the request lifecycle and polling.
 */
export const startRequest = (
  startingURL: string,
  // eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
  body: any,
  options: RequestOptions = {}
): Promise<progress.SerializedProgress> =>
  new Promise<progress.SerializedProgress>((resolve, reject) => {
    if (options.onStart) {
      options.onStart();
    }

    const pollingInterval = options.pollingIntervalMs || 200;

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
          if (options.onProgressUpdate) {
            options.onProgressUpdate(json);
          }
          if (json.status === 'Running') {
            window.setTimeout(() => {
              processFetch(
                fetch(json.url, {
                  method: 'GET',
                })
              );
            }, pollingInterval);
          } else {
            if (options.onSuccess) {
              options.onSuccess(json);
            }
            if (options.onSettled) {
              options.onSettled();
            }
            resolve(json);
          }
        })
        .catch((msg) => {
          if (options.onSettled) {
            options.onSettled();
          }
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
      })
    );
  });

/**
 * Utility function to convert Messages into an error string.
 *
 * If there is no 'Error' message and all the key/value pairs in 'messages' are
 * returned in a single string.
 */
export const messagesToErrorString = (messages: progress.Message[]): string => {
  if (!messages || messages.length === 0) {
    return '(no error message available)';
  }

  const errorMessages = messages.filter((msg) => msg?.key === 'Error');
  if (errorMessages.length === 1) {
    return errorMessages.map((msg) => `${msg?.key}: ${msg?.value}`).join('');
  }
  return messages.map((msg) => `${msg?.key}: ${msg?.value}`).join(' ');
};

/**
 * Converts a Message into a string, one line per key-value pair.
 */
export const messagesToPreString = (messages: progress.Message[]): string =>
  messages.map((msg) => `${msg.key}: ${msg.value}`).join('\n');

/** Utility function to extract on Message from an Array of Messages. */
export const messageByName = (
  messages: progress.Message[],
  key: string,
  fallback: string = ''
): string => {
  if (!messages || messages.length === 0) {
    return fallback;
  }

  const matching = messages.filter((msg) => msg?.key === key);
  if (matching.length === 1) {
    return matching[0].value;
  }
  return fallback;
};
