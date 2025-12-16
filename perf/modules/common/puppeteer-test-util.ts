export const DEFAULT_VIEWPORT = { width: 600, height: 600 };
export const STANDARD_LAPTOP_VIEWPORT = { width: 1536, height: 960 };

/**
 * Common timeouts and intervals used in PointLinksSkPO.
 */
export const POLLING_CONSTANT = {
  TIMEOUT_MS: 30000,
  INTERVAL_MS: 100,
};

export const CLIPBOARD_READ_TIMEOUT_MS = 5000; // 5 second timeout

export async function poll(
  checkFn: () => Promise<boolean>,
  message: string,
  timeout = 5000,
  interval = 100
): Promise<void> {
  const startTime = Date.now();
  while (Date.now() - startTime < timeout) {
    if (await checkFn()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, interval));
  }
  throw new Error(`Timeout: ${message}`);
}
