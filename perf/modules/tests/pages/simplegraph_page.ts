import { Locator, Page } from 'playwright-core';
import {
  clickPickerPlot,
  selectAllPickerField,
  selectPickerField,
  selectPickerFields,
} from './picker_helpers';

/**
 * Page object for /e (explore-sk) supporting both the classic query-dialog UI
 * and the test-picker-sk UI.
 */
export class SimplegraphPage {
  private readonly pickerSelector = 'explore-sk test-picker-sk';

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
    await this.page.goto(`${this.baseUrl}/e/${suffix}`, { waitUntil: 'networkidle' });
    await this.page.locator('explore-sk').waitFor({ state: 'visible', timeout: 10000 });
  }

  public get chart(): Locator {
    return this.page.locator('explore-sk explore-simple-sk plot-google-chart-sk').first();
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

  public async clickPlot(): Promise<void> {
    await clickPickerPlot(this.page, this.pickerSelector);
  }

  /**
   * Waits for the chart to be visible and returns the plotted trace keys and commit count.
   */
  public async getTraceInfo(): Promise<{ keys: string[]; headerCount: number }> {
    await this.chart.waitFor({ state: 'visible', timeout: 10000 });
    return this.page.evaluate(() => {
      const exploreSimple = document.querySelector('explore-sk explore-simple-sk') as any;
      const ts = exploreSimple?.getTraceset?.() || exploreSimple?._dataframe?.traceset || {};
      const header = exploreSimple?.getHeader?.() || exploreSimple?._dataframe?.header || [];
      return {
        keys: Object.keys(ts),
        headerCount: header.length,
      };
    });
  }
}
