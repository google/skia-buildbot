import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { PlotGoogleChartSkPO } from '../plot-google-chart-sk/plot-google-chart-sk_po';
import { PivotTableSkPO } from '../pivot-table-sk/pivot-table-sk_po';
import { errorMessage } from '../errorMessage';
import { ChartTooltipSkPO } from '../chart-tooltip-sk/chart-tooltip-sk_po';
import { GraphTitleSkPO } from '../graph-title-sk/graph-title-sk_po';
import { poll } from '../common/puppeteer-test-util';

export class ExploreSimpleSkPO extends PageObject {
  get pickerField(): PageObjectElement {
    return this.bySelector('picker-field-sk:nth-of-type(1)');
  }

  get queryCountSk(): PageObjectElement {
    return this.bySelector('query-count-sk');
  }

  get querySk(): QuerySkPO {
    return this.poBySelector('query-sk', QuerySkPO);
  }

  get graphTitleSk(): GraphTitleSkPO {
    return this.poBySelector('graph-title-sk', GraphTitleSkPO);
  }

  get plotGoogleChartSk(): PlotGoogleChartSkPO {
    return this.poBySelector('plot-google-chart-sk', PlotGoogleChartSkPO);
  }

  get pivotTableSk(): PivotTableSkPO {
    return this.poBySelector('pivot-table-sk', PivotTableSkPO);
  }

  get openQueryDialogButton(): PageObjectElement {
    return this.bySelector('#open_query_dialog');
  }

  get queryDialog(): PageObjectElement {
    return this.bySelector('#query-dialog');
  }

  get closeQueryDialogButton(): PageObjectElement {
    return this.bySelector('#close_query_dialog');
  }

  get googleChart(): PageObjectElement {
    return this.bySelector('plot-google-chart-sk');
  }

  get chartTooltip(): ChartTooltipSkPO {
    return this.poBySelector('chart-tooltip-sk', ChartTooltipSkPO);
  }

  get googleChartPO(): PlotGoogleChartSkPO {
    return this.poBySelector('plot-google-chart-sk', PlotGoogleChartSkPO);
  }

  get plotSummary(): PageObjectElement {
    return this.bySelector('plot-summary-sk');
  }

  get chartContainer(): PageObjectElement {
    return this.bySelector('#chart-container');
  }

  get xAxisSwitch(): PageObjectElement {
    return this.bySelector('#commit-switch');
  }

  get plotButton(): PageObjectElement {
    return this.bySelector('tabs-panel-sk button.action');
  }

  get spinner(): PageObjectElement {
    return this.bySelector('spinner-sk');
  }

  get showSettingDialigButton(): PageObjectElement {
    return this.bySelector('#showSettingsDialog');
  }

  get settingDialog(): PageObjectElement {
    return this.bySelector('#settings-dialog');
  }

  async getXAxisDomain(): Promise<string> {
    return await this.googleChart.applyFnToDOMNode((c: any) => c.domain);
  }

  get removeAllButton(): PageObjectElement {
    return this.bySelector('#removeAll');
  }

  async clickRemoveAllButton(): Promise<void> {
    await this.removeAllButton.click();
  }

  async clickPlotButton(): Promise<void> {
    await this.plotButton.click();
  }

  get xAxisSwitchSelector(): string {
    // Replace with the actual CSS selector for the switch element
    // within the explore-simple-sk component.
    return '#commit-switch';
  }

  get getXAxisSwitch(): PageObjectElement {
    return this.bySelector(this.xAxisSwitchSelector);
  }

  async clickXAxisSwitch(): Promise<void> {
    await this.clickShowSettingsDialog();
    try {
      await this.bySelector(this.xAxisSwitchSelector);
    } catch (e) {
      await errorMessage(e as Error);
    }

    const switchEl = await this.getXAxisSwitch;
    if (!switchEl) {
      throw new Error('X-Axis switch element not found after visibility wait.');
    }
    try {
      await switchEl.click();
    } catch (e) {
      await errorMessage(e as Error);
    }
  }

  async clickShowSettingsDialog(): Promise<void> {
    await this.showSettingDialigButton.click();
  }

  async getAnomalyMap(): Promise<any> {
    return await this.googleChart.applyFnToDOMNode((el: any) => el.anomalyMap);
  }

  async getTraceKeys(): Promise<string[]> {
    return await this.googleChart.applyFnToDOMNode((el: any) => el.getAllTraces());
  }

  /**
   * Returns the absolute screen coordinates (x, y) for a specific data point on a trace.
   * This is useful for simulating mouse hover events or other interactions at a precise location
   * on the chart.
   *
   * @param traceKey The unique identifier for the trace (e.g., ',arch=x86,config=8888,').
   * @param pointIndex The index of the data point within the trace (row index in the underlying
   * DataTable).
   * @returns A promise that resolves to an object containing the x and y coordinates.
   */
  async getTraceCoordinates(
    traceKey: string,
    pointIndex: number
  ): Promise<{ x: number; y: number }> {
    return await this.googleChart.applyFnToDOMNode(
      (el: any, args: any) => {
        const data = el.data;
        let foundCol = -1;
        // The first two columns are reserved for 'Commit Position' and 'Date',
        // so we start iterating from index 2 to find the trace data.
        for (let i = 2; i < data.getNumberOfColumns(); i++) {
          if (data.getColumnLabel(i) === args.traceKey) {
            foundCol = i;
            break;
          }
        }
        if (foundCol === -1) throw new Error(`Trace ${args.traceKey} not found`);

        const pos = el.getPositionByIndex({ tableRow: args.pointIndex, tableCol: foundCol });
        const rect = el.getBoundingClientRect();
        return { x: rect.left + pos.x, y: rect.top + pos.y };
      },
      { traceKey, pointIndex }
    );
  }

  /**
   * Verifies that anomalies are present in the chart.
   */
  async verifyAnomaliesPresent(): Promise<void> {
    const anomalyMap = await this.getAnomalyMap();
    if (!anomalyMap || Object.keys(anomalyMap).length === 0) {
      throw new Error('No anomalies found in the map');
    }
  }

  /**
   * Clicks on the first visible anomaly in the chart.
   * @param page The Puppeteer Page object to perform the click.
   */
  async clickFirstAnomaly(page: any): Promise<void> {
    const anomalyRect = await this.googleChart.applyFnToDOMNode((el) => {
      const anomalyIcon = el.shadowRoot!.querySelector('div.anomaly > .anomaly');
      if (!anomalyIcon) return null;
      const rect = anomalyIcon.getBoundingClientRect();
      return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    });

    if (!anomalyRect) {
      throw new Error('No anomaly icon found to click.');
    }

    await page.mouse.click(
      anomalyRect.x + anomalyRect.width / 2,
      anomalyRect.y + anomalyRect.height / 2
    );
  }

  /**
   * Waits for the anomaly tooltip to appear and show "Anomaly".
   */
  async waitForAnomalyTooltip(): Promise<void> {
    const containerPO = this.chartTooltip.container;
    await poll(async () => {
      if (await containerPO.isEmpty()) return false;
      return await containerPO.applyFnToDOMNode((el) => {
        if ((el as HTMLElement).style.display === 'none') return false;
        const h3 = el.querySelector('h3');
        return h3?.textContent?.includes('Anomaly') || false;
      });
    }, 'Tooltip did not show Anomaly or was not visible');
  }
}
