import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { poll } from '../common/puppeteer-test-util';

export class PlotGoogleChartSkPO extends PageObject {
  async getGoogleChartObject(): Promise<any> {
    const chartElement = this.bySelectorShadow('google-chart');
    return chartElement.applyFnToDOMNode((el: any) => el.chart);
  }

  async isChartVisible(): Promise<boolean> {
    const chartElement = this.bySelectorShadow('google-chart');
    if (await chartElement.isEmpty()) {
      return false;
    }
    return !(await chartElement.hasAttribute('hidden'));
  }

  async waitForChartVisible(options: { timeout?: number } = {}): Promise<boolean> {
    const timeout = options.timeout ?? 10000;
    try {
      await poll(
        async () => {
          const chartElement = this.bySelectorShadow('google-chart');

          if (!chartElement) return false;
          // Adjust selector for what indicates a rendered chart:
          const hasPaths = !!chartElement?.bySelectorShadow('svg g > path');
          if (!hasPaths) return false;
          return true;
        },
        `Waiting for graph to load`,
        timeout
      );
    } catch (e) {
      throw new Error(
        `Chart did not become visible within ${timeout}ms.Error: ${e instanceof Error}`
      );
    }
    return true;
  }

  async getChartType(): Promise<string | null> {
    const chartElement = this.bySelectorShadow('google-chart');
    return chartElement.getAttribute('type');
  }

  async isResetButtonVisible(): Promise<boolean> {
    const resetButton = this.bySelectorShadow('#reset-view');
    return !(await resetButton.hasAttribute('hidden'));
  }

  async clickResetButton(): Promise<void> {
    const resetButton = this.bySelectorShadow('#reset-view #closeIcon');
    await resetButton.click();
  }

  async getChartData(): Promise<any> {
    const chartElement = await this.bySelectorShadow('google-chart');
    return chartElement.applyFnToDOMNode((el) => (el as any).data);
  }
}
