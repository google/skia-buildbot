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

  /**
   * Returns the current visible X-axis range of the chart.
   * This corresponds to the leftmost and rightmost values currently displayed.
   */
  async getXAxisRange(): Promise<{ min: number; max: number } | null> {
    const chartElement = this.bySelectorShadow('google-chart');
    return chartElement.applyFnToDOMNode((el: any) => {
      const wrapper = el.chartWrapper;
      if (!wrapper) return null;
      const chart = wrapper.getChart();
      if (!chart) return null;
      const layout = chart.getChartLayoutInterface();
      const chartArea = layout.getChartAreaBoundingBox();

      const min = layout.getHAxisValue(chartArea.left);
      const max = layout.getHAxisValue(chartArea.left + chartArea.width);

      const minValue = min instanceof Date ? min.getTime() / 1000 : min;
      const maxValue = max instanceof Date ? max.getTime() / 1000 : max;

      return { min: minValue, max: maxValue };
    });
  }
}
