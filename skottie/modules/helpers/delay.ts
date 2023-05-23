/**
 * delay returns a Promise that will be resolved
 * after the elapsed time (in ms)
 *
 * @param delayMs The time to wait before resolving the promise (in ms)
 */

export default (delayMs: number = 0) =>
  new Promise((resolve) => setTimeout(resolve, delayMs));
