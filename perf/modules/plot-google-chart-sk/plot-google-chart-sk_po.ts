import { PageObject } from '../../../infra-sk/modules/page_object/page_object';

export class PlotGoogleChartSkPO extends PageObject {
  async getGoogleChartObject(): Promise<any> {
    const chartElement = this.bySelectorShadow('google-chart');
    return chartElement.applyFnToDOMNode((el: any) => el.chart);
  }

  async isChartVisible(): Promise<boolean> {
    const chartElement = this.bySelectorShadow('google-chart');
    return !(await chartElement.hasAttribute('hidden'));
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
}
