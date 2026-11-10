import { Locator, Page } from 'playwright-core';

/**
 * Page object for bisect-dialog-sk inside chart-tooltip-sk.
 */
export class BisectDialog {
  constructor(public readonly page: Page) {}

  public get root(): Locator {
    return this.page.locator('bisect-dialog-sk').first();
  }

  public get dialog(): Locator {
    return this.root.locator('#bisect-dialog');
  }

  public async openAndSubmit(params?: {
    testPath?: string;
    startCommit?: string;
    endCommit?: string;
    bugId?: string;
    story?: string;
    patch?: string;
    anomalyId?: string;
  }): Promise<void> {
    const bisectBtn = this.page.locator('chart-tooltip-sk #bisect:not([hidden])').first();
    await bisectBtn.waitFor({ state: 'visible', timeout: 10000 });

    await this.root.evaluate(
      (el: any, p: { anomalyId?: string }) => {
        if (!el.user) {
          el.user = 'test@google.com';
        }
        el.anomalyId = p.anomalyId || '456';
      },
      { anomalyId: params?.anomalyId }
    );

    await bisectBtn.click();
    await this.dialog.waitFor({ state: 'visible', timeout: 10000 });

    const testPath =
      params?.testPath ||
      'ChromiumPerf/mac-m3-pro-perf/jetstream3.crossbench/Total/JetStream2/Total';
    const bugId = params?.bugId || '12345';
    const startCommit = params?.startCommit || '67134';
    const endCommit = params?.endCommit || '67135';
    const story = params?.story || 'Total';
    const patch = params?.patch || '';

    await this.dialog.locator('input#testpath').fill(testPath);
    await this.dialog.locator('input#bug-id').fill(bugId);
    await this.dialog.locator('input#start-commit').fill(startCommit);
    await this.dialog.locator('input#end-commit').fill(endCommit);
    await this.dialog.locator('input#story').fill(story);
    const patchInput = this.dialog.locator('input#patch');
    await patchInput.waitFor({ state: 'visible', timeout: 10000 });
    await patchInput.fill(patch);

    await this.dialog.locator('#submit-button:not([disabled])').click();
    await this.page.waitForLoadState('networkidle');
  }
}
