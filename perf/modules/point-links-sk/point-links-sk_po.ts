import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { POLLING_CONSTANT } from '../common/puppeteer-test-util';

export class PointLinksSkPO extends PageObject {
  private async poll(
    checkFn: () => Promise<boolean>,
    message: string,
    timeout = POLLING_CONSTANT.TIMEOUT_MS,
    interval = POLLING_CONSTANT.INTERVAL_MS
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
}
