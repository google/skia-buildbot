import { Locator, Page } from 'playwright-core';
import {
  clickPickerPlot,
  removePickerFieldValue,
  selectAllPickerField,
  selectPickerField,
  selectPickerFields,
} from './picker_helpers';

/**
 * Page object for /m (explore-multi-sk) covering test-picker-sk selection,
 * split-by-key, graph count, and shortcut persistence.
 */
export class MultigraphPage {
  private readonly pickerSelector = 'test-picker-sk';

  constructor(
    public readonly page: Page,
    public readonly baseUrl: string
  ) {}

  public async goto(queryAndParams: string = ''): Promise<void> {
    const suffix = queryAndParams
      ? queryAndParams.startsWith('?')
        ? queryAndParams
        : `?${queryAndParams}`
      : '';
    await this.page.goto(`${this.baseUrl}/m/${suffix}`, { waitUntil: 'networkidle' });
    await this.page.locator('explore-multi-sk').waitFor({ state: 'visible', timeout: 10000 });
  }

  public get graphs(): Locator {
    return this.page.locator('explore-multi-sk explore-simple-sk:not([style*="display: none"])');
  }

  public get charts(): Locator {
    return this.page.locator(
      'explore-multi-sk explore-simple-sk:not([style*="display: none"]) plot-google-chart-sk'
    );
  }

  public async waitForGraphCount(count: number): Promise<void> {
    await this.page.waitForFunction(
      (expected: number) => {
        const visibleGraphs = Array.from(
          document.querySelectorAll('explore-multi-sk explore-simple-sk')
        ).filter((el) => (el as HTMLElement).style.display !== 'none');
        return (
          visibleGraphs.length === expected &&
          visibleGraphs.every((el) => el.querySelector('plot-google-chart-sk') !== null)
        );
      },
      count,
      { timeout: 10000 }
    );
  }

  public async selectPickerField(label: string, values: string[]): Promise<void> {
    await selectPickerField(this.page, this.pickerSelector, label, values);
  }

  public async selectPickerFields(fields: Record<string, string[]>): Promise<void> {
    await selectPickerFields(this.page, this.pickerSelector, fields);
  }

  public async selectAll(label: string, checked: boolean = true): Promise<string[]> {
    return selectAllPickerField(this.page, this.pickerSelector, label, checked);
  }

  public async removePickerValue(label: string, valueToRemove: string): Promise<void> {
    await removePickerFieldValue(this.page, this.pickerSelector, label, valueToRemove);
  }

  public async setSplitBy(label: string, split: boolean): Promise<void> {
    const fieldLocator = this.page.locator(
      `${this.pickerSelector} picker-field-sk[label="${label}"]:not([disabled])`
    );
    await fieldLocator.waitFor({ state: 'visible', timeout: 10000 });
    const splitCheckbox = fieldLocator.locator(
      'checkbox-sk#split-by:not([disabled]):not([hidden])'
    );
    await splitCheckbox.waitFor({ state: 'visible', timeout: 10000 });
    await splitCheckbox.evaluate((splitBox: any, wantSplit: boolean) => {
      if (splitBox.disabled || splitBox.hidden) {
        throw new Error('checkbox-sk#split-by is disabled or hidden');
      }
      if (Boolean(splitBox.checked) !== wantSplit) {
        const input = splitBox.querySelector('input') as HTMLInputElement | null;
        if (!input || input.disabled) {
          throw new Error('checkbox-sk#split-by input is missing or disabled');
        }
        input.click();
      }
    }, split);
    await this.page.waitForLoadState('networkidle');
  }

  public async clickPlot(): Promise<void> {
    await clickPickerPlot(this.page, this.pickerSelector);
  }

  /**
   * Returns the trace keys rendered in the nth explore-simple-sk graph.
   */
  public async getGraphTraceKeys(graphIndex: number = 0): Promise<string[]> {
    return this.graphs.nth(graphIndex).evaluate((el: any) => {
      const ts = el.getTraceset?.() || el._dataframe?.traceset;
      if (ts && Object.keys(ts).length > 0) {
        return Object.keys(ts);
      }
      const chart = el.querySelector('plot-google-chart-sk') as any;
      if (chart && typeof chart.getAllTraces === 'function') {
        return chart.getAllTraces();
      }
      return [];
    });
  }

  /**
   * Triggers a summary selection range change on a visible split graph so
   * explore-multi-sk synchronizes sibling graphs and URL begin/end parameters.
   */
  public async selectRangeOnSplitGraph(
    visibleGraphIndex: number,
    startIdx: number,
    endIdx: number
  ): Promise<{
    beginOffset: number;
    endOffset: number;
    beginTimestamp: number;
    endTimestamp: number;
  }> {
    return this.graphs.nth(visibleGraphIndex).evaluate(
      (el: any, { start, end }) => {
        const header = el.getHeader?.() || el.dfRepo?.value?.dataframe?.header || [];
        const beginHeader = header[start];
        const endHeader = header[end];
        const beginOffset = Number(beginHeader?.offset ?? 0);
        const endOffset = Number(endHeader?.offset ?? 0);
        const beginTimestamp = Number(beginHeader?.timestamp ?? 0);
        const endTimestamp = Number(endHeader?.timestamp ?? 0);
        if (el.plotSummary?.value) {
          el.plotSummary.value.selectedValueRange = { begin: beginOffset, end: endOffset };
        }
        el.summarySelected({
          value: { begin: beginOffset, end: endOffset },
          domain: 'commit',
          start,
          end,
        });
        return { beginOffset, endOffset, beginTimestamp, endTimestamp };
      },
      { start: startIdx, end: endIdx }
    );
  }

  /**
   * Reads the current selected range state across all visible split graphs.
   */
  public async getVisibleSplitGraphRanges(): Promise<
    Array<{
      graphIndex: number;
      stateBegin: number;
      stateEnd: number;
      summaryBegin: number;
      summaryEnd: number;
    }>
  > {
    return this.page.evaluate(() => {
      const visibleGraphs = Array.from(
        document.querySelectorAll('explore-multi-sk explore-simple-sk')
      ).filter((el) => (el as HTMLElement).style.display !== 'none') as any[];
      return visibleGraphs.map((el) => {
        const summaryRange = el.plotSummary?.value?.selectedValueRange || el.selectedRange || {};
        return {
          graphIndex: Number(el.state?.graph_index ?? -1),
          stateBegin: Number(el.state?.begin ?? 0),
          stateEnd: Number(el.state?.end ?? 0),
          summaryBegin: Math.round(Number(summaryRange.begin ?? 0)),
          summaryEnd: Math.round(Number(summaryRange.end ?? 0)),
        };
      });
    });
  }

  /**
   * Activates the anomaly point tooltip (chart-tooltip-sk) on a visible split
   * graph at the given commit offset and returns the tooltip text.
   */
  public async openAnomalyTooltipOnGraph(
    visibleGraphIndex: number,
    commitOffset: number
  ): Promise<{
    traceName: string;
    anomalyId: string;
    anomalyType: string;
    commitPosition: number;
    tooltipText: string;
  }> {
    const graphLocator = this.graphs.nth(visibleGraphIndex);
    const info = await graphLocator.evaluate(async (el: any, targetOffset: number) => {
      const dfRepo = el.dfRepo?.value;
      const header = dfRepo?.dataframe?.header || [];
      const traceset = dfRepo?.dataframe?.traceset || {};
      const startIdx = Number(el.selectedRange?.begin || 0);
      let headerIdx = -1;
      for (let i = Math.max(0, startIdx); i < header.length; i++) {
        if (Number(header[i]?.offset) === targetOffset) {
          headerIdx = i;
          break;
        }
      }
      if (headerIdx < 0) {
        headerIdx = header.findIndex((h: any) => Number(h.offset) === targetOffset);
      }
      const pointX = headerIdx - startIdx;

      let matchedTrace = '';
      let anomalyObj: any = null;
      for (const traceKey of Object.keys(traceset)) {
        const a = dfRepo?.getAnomaly?.(traceKey, targetOffset);
        if (a) {
          matchedTrace = traceKey;
          anomalyObj = a;
          break;
        }
      }
      if (!matchedTrace) {
        matchedTrace = Object.keys(traceset)[0] || '';
      }
      const yVal = traceset[matchedTrace]?.[headerIdx] ?? 0;
      const commits = [header[Math.max(0, headerIdx - 1)] || null, header[headerIdx] || null];
      el.selectedAnomaly = anomalyObj;
      el.enableTooltip(
        { x: pointX, y: yVal, xPos: 120, yPos: 120, name: matchedTrace },
        commits,
        true
      );
      const tooltip = el.querySelector('chart-tooltip-sk') as any;
      if (tooltip?.updateComplete) {
        await tooltip.updateComplete;
      }
      const anomalyBadge = tooltip?.querySelector(
        '#anomaly-details .regression, #anomaly-details .improvement'
      ) as HTMLElement | null;
      return {
        traceName: matchedTrace,
        anomalyId: anomalyObj ? String(anomalyObj.id) : '',
        anomalyType: (anomalyBadge?.textContent || '').trim(),
        commitPosition: Number(tooltip?.commit_position ?? targetOffset),
        tooltipText: (tooltip?.innerText || tooltip?.textContent || '').trim(),
      };
    }, commitOffset);
    await graphLocator
      .locator('chart-tooltip-sk .container')
      .waitFor({ state: 'visible', timeout: 10000 });
    return info;
  }
}
