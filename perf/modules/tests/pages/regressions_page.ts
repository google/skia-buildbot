import { Locator, Page } from 'playwright-core';
import { TriageMenu } from './triage_menu';

/**
 * Page object for /a (regressions-page-sk) covering sheriff selection,
 * Show Triaged / Show Improvements toggles, row selection, Triage Selected,
 * and Graph Selected navigation.
 */
export class RegressionsPage {
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
    await this.page.goto(`${this.baseUrl}/a/${suffix}`, { waitUntil: 'networkidle' });
    await this.page.locator('regressions-page-sk').waitFor({ state: 'visible', timeout: 10000 });
  }

  public get sheriffSelect(): Locator {
    return this.page.locator('regressions-page-sk select[id^="filter-"]');
  }

  public get anomaliesTable(): Locator {
    return this.page.locator('regressions-page-sk anomalies-table-sk');
  }

  public get rows(): Locator {
    return this.anomaliesTable.locator('table.anomalies-table tbody tr[data-bugid]');
  }

  public async waitForIdle(): Promise<void> {
    await this.page.waitForFunction(
      () => {
        const el = document.querySelector('regressions-page-sk') as any;
        return !!el && !el.anomaliesLoadingSpinner && !el.showMoreLoadingSpinner;
      },
      undefined,
      { timeout: 10000 }
    );
  }

  public async selectSheriff(sheriffName: string): Promise<void> {
    await this.sheriffSelect.waitFor({ state: 'visible', timeout: 10000 });
    await this.sheriffSelect.locator(`option[value="${sheriffName}"]`).waitFor({
      state: 'attached',
      timeout: 10000,
    });
    await this.sheriffSelect.selectOption(sheriffName);
    await this.page.waitForLoadState('networkidle');
    await this.waitForIdle();
    const subTable = this.page.locator(
      'regressions-page-sk subscription-table-sk#subscription-table'
    );
    await subTable
      .locator('.subscription-details h2', { hasText: sheriffName })
      .waitFor({ state: 'visible', timeout: 10000 });
    await subTable.locator('#btn-toggle-alerts').waitFor({ state: 'visible', timeout: 10000 });
  }

  public async setShowTriaged(checked: boolean): Promise<void> {
    const btn = this.page.locator('regressions-page-sk #btnTriaged');
    await btn.waitFor({ state: 'visible', timeout: 10000 });
    const text = ((await btn.textContent()) || '').trim();
    const isCurrentlyShowing = text.includes('Hide Triaged');
    if (isCurrentlyShowing !== checked) {
      await btn.click();
      await this.page.waitForLoadState('networkidle');
      await this.waitForIdle();
    }
  }

  public async setShowImprovements(checked: boolean): Promise<void> {
    const btn = this.page.locator('regressions-page-sk #btnImprovements');
    await btn.waitFor({ state: 'visible', timeout: 10000 });
    const text = ((await btn.textContent()) || '').trim();
    const isCurrentlyShowing = text.includes('Hide Improvements');
    if (isCurrentlyShowing !== checked) {
      await btn.click();
      await this.page.waitForLoadState('networkidle');
      await this.waitForIdle();
    }
  }

  /**
   * Checks the checkbox for a specific anomaly ID in anomalies-table-sk.
   */
  public async checkAnomaly(anomalyId: string): Promise<void> {
    const cb = this.anomaliesTable.locator(`input[type="checkbox"][id$="-${anomalyId}"]`);
    await cb.waitFor({ state: 'visible', timeout: 10000 });
    if (!(await cb.isChecked())) {
      await cb.click();
    }
  }

  public async uncheckAnomaly(anomalyId: string): Promise<void> {
    const cb = this.anomaliesTable.locator(`input[type="checkbox"][id$="-${anomalyId}"]`);
    await cb.waitFor({ state: 'visible', timeout: 10000 });
    if (await cb.isChecked()) {
      await cb.click();
    }
  }

  public async clickTriageSelected(): Promise<void> {
    await this.anomaliesTable.locator('button[id^="triage-button-"]').click();
  }

  public async clickGraphSelected(): Promise<void> {
    await this.anomaliesTable.locator('button[id^="graph-button-"]').click();
  }

  public async clickGraphSelectedByGroup(): Promise<void> {
    await this.anomaliesTable.locator('button[id^="open-group-button-"]').click();
  }

  /**
   * Clicks the trendingicon-link button on a single-anomaly row.
   */
  public async clickSingleRowTrendingIcon(anomalyId: string): Promise<void> {
    const row = this.anomaliesTable
      .locator('table.anomalies-table tbody tr:not(.summary-row)')
      .filter({ has: this.page.locator(`input[type="checkbox"][id$="-${anomalyId}"]`) })
      .first();
    await row.locator('button.trendingicon-link').click();
  }

  /**
   * Clicks the trendingicon-link button on the first multi-anomaly summary row.
   */
  public async clickSummaryRowTrendingIcon(): Promise<void> {
    const summaryRow = this.anomaliesTable
      .locator('table.anomalies-table tbody tr.summary-row')
      .first();
    await summaryRow.waitFor({ state: 'visible', timeout: 10000 });
    await summaryRow.locator('button.trendingicon-link').click();
  }

  /**
   * Returns the trimmed text of the Bug ID column (.tooltip-cell) for a given anomaly row.
   */
  public async getBugColumnText(anomalyId: string): Promise<string> {
    const row = this.anomaliesTable
      .locator('table.anomalies-table tbody tr')
      .filter({ has: this.page.locator(`input[type="checkbox"][id$="-${anomalyId}"]`) })
      .first();
    const cell = row.locator('td.tooltip-cell');
    await cell.waitFor({ state: 'visible', timeout: 10000 });
    return ((await cell.textContent()) || '').replace(/\s+/g, ' ').trim();
  }

  public async installWindowOpenSpy(): Promise<void> {
    await this.page.evaluate(() => {
      (window as any).__openedWindows = [];
      window.open = (url?: string | URL) => {
        const fakeWin: any = {
          location: { href: url ? String(url) : '' },
          document: { write: () => {}, close: () => {} },
          close: () => {},
        };
        (window as any).__openedWindows.push(fakeWin);
        return fakeWin;
      };
    });
  }

  public async getOpenedUrls(): Promise<string[]> {
    return this.page.evaluate(() =>
      ((window as any).__openedWindows || []).map((w: any) => String(w.location.href))
    );
  }

  /**
   * Installs the window.open spy, executes `action`, waits for networkidle,
   * and returns the URLs passed to window.open.
   */
  public async captureOpenedUrls(action: () => Promise<void>): Promise<string[]> {
    await this.installWindowOpenSpy();
    await action();
    await this.page.waitForLoadState('networkidle');
    return this.getOpenedUrls();
  }

  /**
   * Checks `anomalyId`, opens Triage Selected, executes the given triage action,
   * reads the updated Bug ID column text (or the `anomaly-changed` event bug ID
   * when the component reports completion via event), unchecks the row, and
   * returns the text.
   */
  public async triageAnomaly(
    triageMenu: TriageMenu,
    anomalyId: string,
    action: 'ignore' | 'new-bug' | 'existing-bug',
    existingBugId?: number
  ): Promise<string> {
    await this.checkAnomaly(anomalyId);
    await this.clickTriageSelected();
    if (action === 'ignore') {
      await triageMenu.clickIgnore();
    } else if (action === 'new-bug') {
      await triageMenu.clickNewBug();
    } else {
      const associatedBugId = await triageMenu.associateExistingBug(existingBugId ?? 54321);
      await this.uncheckAnomaly(anomalyId);
      return associatedBugId;
    }
    const bugText = await this.getBugColumnText(anomalyId);
    await this.uncheckAnomaly(anomalyId);
    return bugText;
  }
}
