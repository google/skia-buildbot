import { Locator, Page } from 'playwright-core';

/**
 * Page object for /u (report-page-sk) covering requested vs. overlapping
 * anomaly tables, chart rendering, and anomaly tooltip inspection.
 */
export class ReportPage {
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
    await this.page.goto(`${this.baseUrl}/u${suffix}`, { waitUntil: 'networkidle' });
    await this.page.locator('report-page-sk').waitFor({ state: 'visible', timeout: 10000 });
    await this.waitForLoaded();
  }

  public async waitForLoaded(): Promise<void> {
    await this.page.waitForFunction(
      () => {
        const el = document.querySelector('report-page-sk') as any;
        return !!el && !el._currentlyLoading;
      },
      undefined,
      { timeout: 15000 }
    );
  }

  public get anomaliesTable(): Locator {
    return this.page.locator('report-page-sk anomalies-table-sk');
  }

  public get overlappingSeparator(): Locator {
    return this.anomaliesTable.locator('.separator-text');
  }

  public get commonCommitMessages(): Locator {
    return this.page.locator(
      'report-page-sk .common-commits #all-commits-scroll li #commit-message'
    );
  }

  public get graphs(): Locator {
    return this.page.locator('report-page-sk explore-simple-sk');
  }

  public get tooltip(): Locator {
    return this.page.locator('report-page-sk explore-simple-sk chart-tooltip-sk').first();
  }

  public async checkAnomaly(anomalyId: string): Promise<void> {
    const cb = this.anomaliesTable.locator(`input[type="checkbox"][id$="-${anomalyId}"]`);
    await cb.waitFor({ state: 'visible', timeout: 10000 });
    if (!(await cb.isChecked())) {
      await cb.click();
    }
  }

  public async openAnomalyTooltip(commitOffset?: number): Promise<void> {
    await this.graphs.first().waitFor({ state: 'visible', timeout: 10000 });
    await this.page.waitForFunction(
      () => {
        const explore = document.querySelector('report-page-sk explore-simple-sk') as any;
        const df = explore?.dfRepo?.value?.dataframe;
        return !!df && Object.keys(df.traceset || {}).length > 0 && (df.header || []).length > 0;
      },
      undefined,
      { timeout: 15000 }
    );
    await this.page.evaluate((targetOffset) => {
      const explore = document.querySelector('report-page-sk explore-simple-sk') as any;
      const repo = explore.dfRepo.value;
      const df = repo.dataframe;
      const traceKey = Object.keys(df.traceset)[0];
      const header = df.header as any[];
      let xIdx = header.findIndex((h) => h.offset === targetOffset);
      if (xIdx < 0) {
        for (let i = 0; i < header.length; i++) {
          if (repo.getAnomaly(traceKey, header[i].offset)) {
            xIdx = i;
            break;
          }
        }
      }
      if (xIdx < 0) {
        xIdx = Math.min(12, header.length - 1);
      }
      const commitPos = header[xIdx].offset;
      const yVal = df.traceset[traceKey][xIdx];
      explore.selectedAnomaly = repo.getAnomaly(traceKey, commitPos) || null;
      explore.enableTooltip(
        {
          x: xIdx - (explore.selectedRange?.begin || 0),
          y: yVal,
          xPos: 120,
          yPos: 120,
          name: traceKey,
        },
        [header[Math.max(0, xIdx - 1)], header[xIdx]],
        true
      );
    }, commitOffset);
    await this.tooltip.locator('.container').waitFor({ state: 'visible', timeout: 10000 });
  }

  /**
   * Returns whether the #bisect and #try-job action buttons are visible inside
   * the active chart-tooltip-sk.
   */
  public async getTooltipButtonVisibility(): Promise<{
    bisectVisible: boolean;
    tryJobVisible: boolean;
  }> {
    const bisectBtn = this.tooltip.locator('#bisect');
    const tryJobBtn = this.tooltip.locator('#try-job');
    return {
      bisectVisible: !(await bisectBtn.isHidden()),
      tryJobVisible: !(await tryJobBtn.isHidden()),
    };
  }
}
