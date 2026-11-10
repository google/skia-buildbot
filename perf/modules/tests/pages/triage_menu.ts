import { Locator, Page } from 'playwright-core';

/**
 * Page object for triage-menu-sk, new-bug-dialog-sk, and existing-bug-dialog-sk.
 */
export class TriageMenu {
  constructor(
    public readonly page: Page,
    private readonly containerSelector: string = 'anomalies-table-sk'
  ) {}

  public get root(): Locator {
    return this.page.locator(`${this.containerSelector} triage-menu-sk`).first();
  }

  public async clickIgnore(): Promise<void> {
    const btn = this.root.locator('#ignore');
    await btn.waitFor({ state: 'visible', timeout: 10000 });
    await btn.click();
    await this.page.waitForLoadState('networkidle');
  }

  public async clickNewBug(): Promise<void> {
    const btn = this.root.locator('#new-bug');
    await btn.waitFor({ state: 'visible', timeout: 10000 });
    await btn.click();
    await this.page.waitForLoadState('networkidle');
  }

  public async associateExistingBug(bugId: string | number): Promise<string> {
    const btn = this.root.locator('#existing-bug:not([disabled])');
    await btn.waitFor({ state: 'visible', timeout: 10000 });
    await btn.click();

    const dialog = this.root.locator('existing-bug-dialog-sk #existing-bug-dialog');
    await dialog.waitFor({ state: 'visible', timeout: 10000 });
    const expectedBugId = Number(bugId);
    const eventPromise = this.root.evaluate(
      (el: any, wantBugId: number) =>
        new Promise<string>((resolve, reject) => {
          el.addEventListener(
            'anomaly-changed',
            (e: any) => {
              const updatedAnomalies: any[] = e.detail?.anomalies || [];
              if (
                updatedAnomalies.length === 0 ||
                !updatedAnomalies.every((a: any) => Number(a.bug_id) === wantBugId)
              ) {
                reject(
                  new Error(`existing-bug-dialog-sk did not update anomaly bug_id to ${wantBugId}`)
                );
                return;
              }
              resolve(String(e.detail?.bugId ?? updatedAnomalies[0].bug_id));
            },
            { once: true }
          );
        }),
      expectedBugId
    );
    const input = dialog.locator('#bug_id');
    await input.fill(String(bugId));
    await dialog.locator('#file-button:not([disabled])').click();
    const associatedBugId = await eventPromise;
    await this.page.waitForLoadState('networkidle');
    return associatedBugId;
  }
}
