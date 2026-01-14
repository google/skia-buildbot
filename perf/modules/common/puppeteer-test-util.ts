import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
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

export const DEFAULT_WAIT_FOR_FUNNCTION_MS = 3000;

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

export async function waitForDataLoad(
  chartElement: PageObjectElement,
  timeout = 10000
): Promise<void> {
  try {
    await chartElement.applyFnToDOMNode((el: any) => {
      el.dataLoading = true;
    });
  } catch (e) {
    console.warn(`waitForDataLoad timed out after ${timeout}ms,`, e);
  }
}

export async function waitForReady(po: PageObject): Promise<void> {
  await po.bySelectorShadow('google-chart');
  try {
    // Wait for the specific element that indicates the chart drawing is complete.
    // This selector depends on the internals of plot-google-chart-sk.
    await po.bySelector('svg > g');
  } catch (e) {
    throw new Error(e as string);
  }
}
