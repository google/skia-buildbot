import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';
import { PlotGoogleChartSkPO } from '../plot-google-chart-sk/plot-google-chart-sk_po';
import { PivotTableSkPO } from '../pivot-table-sk/pivot-table-sk_po';
import { errorMessage } from '../errorMessage';
import { ChartTooltipSkPO } from '../chart-tooltip-sk/chart-tooltip-sk_po';
import { GraphTitleSkPO } from '../graph-title-sk/graph-title-sk_po';
import { poll } from '../common/puppeteer-test-util';
import { PlotSummarySkPO } from '../plot-summary-sk/plot-summary-sk_po';
import { QueryCountSkPO } from '../query-count-sk/query-count-sk_po';
import { ParamSetSkPO } from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';

export class ExploreSimpleSkPO extends PageObject {
  get pickerField(): PageObjectElement {
    return this.bySelector('picker-field-sk:nth-of-type(1)');
  }

  get queryCountSkPO(): QueryCountSkPO {
    return this.poBySelector('query-count-sk', QueryCountSkPO);
  }

  get queryCountSk(): PageObjectElement {
    return this.bySelector('query-count-sk');
  }

  get queryCountText(): PageObjectElement {
    return this.queryCountSkPO.bySelector('span');
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

  get plotSummary(): PlotSummarySkPO {
    return this.poBySelector('plot-summary-sk', PlotSummarySkPO);
  }

  get chartContainer(): PageObjectElement {
    return this.bySelector('#chart-container');
  }

  get xAxisSwitch(): PageObjectElement {
    return this.bySelector('#commit-switch');
  }

  get zoomDirectionSwitch(): PageObjectElement {
    return this.bySelector('#zoom-direction-switch');
  }

  get evenXAxisSpacingSwitch(): PageObjectElement {
    return this.bySelector('#even-x-axis-spacing-switch');
  }

  get summaryParamsetSkPO(): ParamSetSkPO {
    return this.poBySelector('#query-dialog paramset-sk#summary', ParamSetSkPO);
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

  get paramsTab(): PageObjectElement {
    return this.bySelector('tabs-sk#detailTab > button');
  }

  get paramsetSk(): PageObjectElement {
    return this.bySelector('paramset-sk#paramset');
  }

  get collapseButton(): PageObjectElement {
    return this.bySelector('#collapseButton');
  }

  async clickParam(key: string, value: string) {
    const paramElement = this.paramsetSk.bySelector(
      `div[data-key='${key}'][data-value='${value}']`
    );
    await paramElement.click();
  }

  async getXAxisDomain(): Promise<string> {
    return await this.googleChart.applyFnToDOMNode((c: any) => c.domain);
  }

  async getHorizontalZoom(): Promise<boolean> {
    return await this.zoomDirectionSwitch.applyFnToDOMNode((c: any) => c.selected);
  }

  async getEvenXAxisSpacing(): Promise<boolean> {
    return await this.googleChart.applyFnToDOMNode((c: any) => c.useDiscreteAxis);
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

  get zoomDirectionSwitchSelector(): string {
    return '#zoom-direction-switch';
  }

  get evenXAxisSpacingSwitchSelector(): string {
    return '#even-x-axis-spacing-switch';
  }

  get getXAxisSwitch(): PageObjectElement {
    return this.bySelector(this.xAxisSwitchSelector);
  }

  get getZoomDirectionSwitch(): PageObjectElement {
    return this.bySelector(this.zoomDirectionSwitchSelector);
  }

  get getEvenXAxisSpacingSwitch(): PageObjectElement {
    return this.bySelector(this.evenXAxisSpacingSwitchSelector);
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

  async clickZoomDirectionSwitch(): Promise<void> {
    await this.clickShowSettingsDialog();
    try {
      await this.bySelector(this.zoomDirectionSwitchSelector);
    } catch (e) {
      await errorMessage(e as Error);
    }

    const switchEl = await this.getZoomDirectionSwitch;
    if (!switchEl) {
      throw new Error('Zoom direction switch element not found after visibility wait.');
    }
    try {
      await switchEl.click();
    } catch (e) {
      await errorMessage(e as Error);
    }
  }

  async clickEvenXAxisSpacingSwitch(): Promise<void> {
    await this.clickShowSettingsDialog();
    try {
      await this.bySelector(this.evenXAxisSpacingSwitchSelector);
    } catch (e) {
      await errorMessage(e as Error);
    }

    const switchEl = await this.getEvenXAxisSpacingSwitch;
    if (!switchEl) {
      throw new Error('Even X-Axis spacing switch element not found after visibility wait.');
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
  ): Promise<{ x: number; y: number; width: number; height: number } | null> {
    return await this.googleChart.applyFnToDOMNode(
      (el) => {
        const anomalyIcon = el.shadowRoot!.querySelector('div.anomaly > .anomaly');
        if (!anomalyIcon) return null;
        const rect = anomalyIcon.getBoundingClientRect();
        return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
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
