import { Page } from 'puppeteer';
import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { poll } from '../common/puppeteer-test-util';

/**
 * Page Object for the PlotSummarySk component.
 */
export class PlotSummarySkPO extends PageObject {
  get leftLoadButton(): PageObjectElement {
    return this.bySelectorShadow('.load-btn:first-of-type');
  }

  get rightLoadButton(): PageObjectElement {
    return this.bySelectorShadow('.load-btn:last-of-type');
  }

  get googleChart(): PageObjectElement {
    return this.bySelectorShadow('google-chart');
  }

  private _boundingBox: any;

  get boundingBox(): any {
    return this._boundingBox;
  }

  set boundingBox(boundingBox: any) {
    this._boundingBox = boundingBox;
  }

  async clickLeftLoad(): Promise<void> {
    await this.leftLoadButton.click();
  }

  async clickRightLoad(): Promise<void> {
    await this.rightLoadButton.click();
  }

  async waitForPlotSummaryToLoad(): Promise<void> {
    await poll(async () => {
      return await this.element.applyFnToDOMNode((el) => {
        // 1. Check Data Loading phase
        if (el.hasAttribute('loading')) return false;

        const root = el.shadowRoot || el;
        const chart = root.querySelector('google-chart') as any; // Cast to access chartWrapper
        if (!chart) return false;

        // 2. Check Rendering phase (SVG existence)
        // Google Charts usually render SVGs inside their shadow root
        const chartRoot = chart.shadowRoot || chart;
        const svg = chartRoot.querySelector('svg');
        if (!svg) return false;

        // 3. Check API Readiness phase (Crucial for selectRange)
        const wrapper = chart.chartWrapper;
        const coreChart = wrapper?.getChart();

        // Verify we can actually get the layout interface
        return coreChart && typeof coreChart.getChartLayoutInterface === 'function';
      });
    }, 'Waiting for plot summary to load data, render SVG, and initialize Chart API');
  }

  /**
   * Selects a range based on relative ratios (0.0 to 1.0).
   * This guarantees the same proportional selection regardless of screen size.
   *
   * @param page The Puppeteer page.
   * @param startRatio Start position (e.g., 0.25 for 25% from left).
   * @param endRatio End position (e.g., 0.55 for 55% from left).
   */
  async selectRange(page: Page, startRatio: number, endRatio: number): Promise<void> {
    const rect = await this.element.applyFnToDOMNode((el) => {
      const r = el.getBoundingClientRect();
      return { x: r.left, y: r.top, width: r.width, height: r.height };
    });

    const centerY = rect.y + rect.height / 2;
    const startX = Math.round(rect.x + rect.width * startRatio);
    const endX = Math.round(rect.x + rect.width * endRatio);

    await page.mouse.move(startX, centerY);
    await page.mouse.down();
    await page.mouse.move(endX, centerY, { steps: 10 });
    await page.mouse.up();
  }
}
